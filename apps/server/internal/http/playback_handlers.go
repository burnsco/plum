package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"plum/internal/db"
	"plum/internal/httputil"
	"plum/internal/transcoder"
)

// Limits concurrent embedded-subtitle cache warm jobs (each may spawn ffmpeg). Per-media coalescing
// in db.WarmEmbeddedSubtitleCachesForMedia avoids duplicate work for one title; this bounds total
// parallelism across many sessions (e.g. scan-then-play bursts).
var embeddedSubtitleWarmSem = make(chan struct{}, 6)

const embeddedSubtitleWarmTimeout = 5 * time.Minute

// acquireEmbeddedSubtitleWarmSem waits for a warm slot or returns false if ctx is cancelled first,
// so goroutines do not block indefinitely when shutdown or timeout fires before the semaphore frees.
func acquireEmbeddedSubtitleWarmSem(ctx context.Context) bool {
	select {
	case embeddedSubtitleWarmSem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

type PlaybackHandler struct {
	DB       *sql.DB
	Sessions *transcoder.PlaybackSessionManager
	ThumbDir string
	ArtDir   string
}

// mediaItemForUser returns media the authenticated user may access (library owner). Uses 404 when
// denied so existence is not leaked to other accounts.
func (h *PlaybackHandler) mediaItemForUser(w http.ResponseWriter, user *db.User, mediaID int) (*db.MediaItem, bool) {
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	media, err := db.GetMediaByID(h.DB, mediaID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, false
	}
	if media == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return nil, false
	}
	ok, err := db.UserHasLibraryAccess(h.DB, user.ID, media.LibraryID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, false
	}
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return nil, false
	}
	return media, true
}

// subtitleForUser resolves a sidecar subtitle by id and ensures the user may access its media (404 when denied).
func (h *PlaybackHandler) subtitleForUser(w http.ResponseWriter, user *db.User, subtitleID int) (*db.Subtitle, bool) {
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil, false
	}
	s, err := db.GetSubtitleByID(h.DB, subtitleID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, false
	}
	if s == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return nil, false
	}
	if _, ok := h.mediaItemForUser(w, user, s.MediaID); !ok {
		return nil, false
	}
	return s, true
}

func (h *PlaybackHandler) ListMedia(w http.ResponseWriter, r *http.Request) {
	u := UserFromContext(r.Context())
	if u == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	db.HandleListMediaForUser(w, r, h.DB, u.ID)
}

func (h *PlaybackHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	media, ok := h.mediaItemForUser(w, user, id)
	if !ok {
		return
	}
	if err := db.SetPlaybackProgressContinueWatchingHidden(h.DB, user.ID, media.ID, false); err != nil {
		slog.Warn(
			"playback: failed to mark continue-watching visible on session start",
			"user_id", user.ID,
			"media_id", media.ID,
			"error", err,
		)
	}
	if _, err := db.RefreshPlaybackTrackMetadata(r.Context(), h.DB, media); err != nil {
		writePlaybackError(w, err)
		return
	}
	settings, err := db.GetTranscodingSettings(h.DB)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var payload struct {
		AudioIndex                      int                                   `json:"audioIndex"`
		ClientCapabilities              transcoder.ClientPlaybackCapabilities `json:"clientCapabilities"`
		BurnEmbeddedSubtitleStreamIndex *int                                  `json:"burnEmbeddedSubtitleStreamIndex"`
	}
	payload.AudioIndex = -1
	if r.ContentLength != 0 {
		if !decodeRequestJSON(w, r, &payload) {
			return
		}
	}

	introMode := db.GetLibraryIntroSkipMode(h.DB, media.LibraryID)
	state, err := h.Sessions.Create(
		r.Context(),
		*media,
		introMode,
		settings,
		payload.AudioIndex,
		user.ID,
		payload.ClientCapabilities,
		payload.BurnEmbeddedSubtitleStreamIndex,
	)
	if err != nil {
		if errors.Is(err, transcoder.ErrTooManyPlaybackSessions) {
			http.Error(w, err.Error(), http.StatusTooManyRequests)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	mid := media.ID
	go func() {
		ctx, cancel := context.WithTimeout(h.Sessions.ShutdownContext(), embeddedSubtitleWarmTimeout)
		defer cancel()
		if !acquireEmbeddedSubtitleWarmSem(ctx) {
			return
		}
		defer func() { <-embeddedSubtitleWarmSem }()
		db.WarmEmbeddedSubtitleCachesForMedia(ctx, h.DB, mid)
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

// WarmEmbeddedSubtitleCaches starts background materialization of on-disk WebVTT caches for all
// embedded subtitle tracks. The client should call this as soon as playback is requested (e.g. when
// subtitles are enabled by default) so work begins before the first GET …/subtitles/embedded/… .
func (h *PlaybackHandler) WarmEmbeddedSubtitleCaches(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	media, ok := h.mediaItemForUser(w, user, id)
	if !ok {
		return
	}
	mid := media.ID
	go func() {
		ctx, cancel := context.WithTimeout(h.Sessions.ShutdownContext(), embeddedSubtitleWarmTimeout)
		defer cancel()
		if !acquireEmbeddedSubtitleWarmSem(ctx) {
			return
		}
		defer func() { <-embeddedSubtitleWarmSem }()
		db.WarmEmbeddedSubtitleCachesForMedia(ctx, h.DB, mid)
	}()
	w.WriteHeader(http.StatusAccepted)
}

func (h *PlaybackHandler) RefreshPlaybackTracks(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	media, ok := h.mediaItemForUser(w, user, id)
	if !ok {
		return
	}
	metadata, err := db.RefreshPlaybackTrackMetadata(r.Context(), h.DB, media)
	if err != nil {
		writePlaybackError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(metadata)
}

// PatchMediaIntro updates manual intro/credits bounds on the primary media file row.
func (h *PlaybackHandler) PatchMediaIntro(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	if _, ok := h.mediaItemForUser(w, user, id); !ok {
		return
	}
	var payload struct {
		IntroStartSeconds   *float64 `json:"intro_start_seconds"`
		IntroEndSeconds     *float64 `json:"intro_end_seconds"`
		IntroLocked         *bool    `json:"intro_locked"`
		ClearIntro          bool     `json:"clear_intro"`
		CreditsStartSeconds *float64 `json:"credits_start_seconds"`
		CreditsEndSeconds   *float64 `json:"credits_end_seconds"`
		ClearCredits        bool     `json:"clear_credits"`
	}
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	if !payload.ClearIntro && !payload.ClearCredits && payload.IntroStartSeconds == nil &&
		payload.IntroEndSeconds == nil && payload.IntroLocked == nil &&
		payload.CreditsStartSeconds == nil && payload.CreditsEndSeconds == nil {
		http.Error(w, "no fields to update", http.StatusBadRequest)
		return
	}
	err := db.PatchMediaPlaybackSegments(
		r.Context(), h.DB, id,
		payload.IntroStartSeconds, payload.IntroEndSeconds, payload.IntroLocked, payload.ClearIntro,
		payload.CreditsStartSeconds, payload.CreditsEndSeconds, payload.ClearCredits,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PlaybackHandler) UpdateSessionAudio(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := h.Sessions.EnsureSessionOwner(sessionID, user.ID); err != nil {
		writePlaybackError(w, err)
		return
	}
	var payload struct {
		AudioIndex int `json:"audioIndex"`
	}
	if !decodeRequestJSON(w, r, &payload) {
		return
	}
	settings, err := db.GetTranscodingSettings(h.DB)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	state, err := h.Sessions.UpdateAudio(sessionID, settings, payload.AudioIndex)
	if err != nil {
		writePlaybackError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func (h *PlaybackHandler) CloseSession(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	sessionID := chi.URLParam(r, "sessionId")
	if err := h.Sessions.EnsureSessionOwner(sessionID, user.ID); err != nil {
		writePlaybackError(w, err)
		return
	}
	h.Sessions.Close(sessionID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "closed"})
}

func (h *PlaybackHandler) ServeSessionRevision(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	sessionID := chi.URLParam(r, "sessionId")
	if err := h.Sessions.EnsureSessionOwner(sessionID, user.ID); err != nil {
		writePlaybackError(w, err)
		return
	}
	revision, ok := parsePathInt(w, chi.URLParam(r, "revision"), "invalid revision")
	if !ok {
		return
	}
	httputil.ClearStreamWriteDeadline(w)
	if err := h.Sessions.ServeFile(w, r, sessionID, revision, chi.URLParam(r, "*")); err != nil {
		writePlaybackError(w, err)
	}
}

func (h *PlaybackHandler) StreamMedia(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	if _, ok := h.mediaItemForUser(w, user, id); !ok {
		return
	}
	httputil.ClearStreamWriteDeadline(w)
	if err := db.HandleStreamMedia(w, r, h.DB, id); err != nil {
		writePlaybackError(w, err)
	}
}

func (h *PlaybackHandler) StreamEmbeddedSubtitle(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	if _, ok := h.mediaItemForUser(w, user, id); !ok {
		return
	}
	streamIndex, ok := parsePathInt(w, chi.URLParam(r, "index"), "invalid index")
	if !ok {
		return
	}
	httputil.ClearStreamWriteDeadline(w)
	var bodyStarted bool
	tw := &trackStreamBody{ResponseWriter: w, started: &bodyStarted}
	if err := db.HandleStreamEmbeddedSubtitle(tw, r, h.DB, id, streamIndex); err != nil {
		if !bodyStarted {
			writePlaybackError(w, err)
		} else {
			slog.Error("embedded subtitle stream ended after response started", "media_id", id, "stream_index", streamIndex, "error", err)
		}
	}
}

func (h *PlaybackHandler) StreamEmbeddedSubtitleSup(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	if _, ok := h.mediaItemForUser(w, user, id); !ok {
		return
	}
	streamIndex, ok := parsePathInt(w, chi.URLParam(r, "index"), "invalid index")
	if !ok {
		return
	}
	httputil.ClearStreamWriteDeadline(w)
	var bodyStarted bool
	tw := &trackStreamBody{ResponseWriter: w, started: &bodyStarted}
	if err := db.HandleStreamEmbeddedSubtitleSup(tw, r, h.DB, id, streamIndex); err != nil {
		if !bodyStarted {
			writePlaybackError(w, err)
		} else {
			slog.Error("embedded pgs stream ended after response started", "media_id", id, "stream_index", streamIndex, "error", err)
		}
	}
}

func (h *PlaybackHandler) StreamEmbeddedSubtitleAss(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	if _, ok := h.mediaItemForUser(w, user, id); !ok {
		return
	}
	streamIndex, ok := parsePathInt(w, chi.URLParam(r, "index"), "invalid index")
	if !ok {
		return
	}
	httputil.ClearStreamWriteDeadline(w)
	var bodyStarted bool
	tw := &trackStreamBody{ResponseWriter: w, started: &bodyStarted}
	if err := db.HandleStreamEmbeddedSubtitleAss(tw, r, h.DB, id, streamIndex); err != nil {
		if !bodyStarted {
			writePlaybackError(w, err)
		} else {
			slog.Error("embedded ass stream ended after response started", "media_id", id, "stream_index", streamIndex, "error", err)
		}
	}
}

func (h *PlaybackHandler) StreamSubtitle(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	if _, ok := h.subtitleForUser(w, user, id); !ok {
		return
	}
	httputil.ClearStreamWriteDeadline(w)
	var bodyStarted bool
	tw := &trackStreamBody{ResponseWriter: w, started: &bodyStarted}
	if err := db.HandleStreamSubtitle(tw, r, h.DB, id); err != nil {
		if !bodyStarted {
			writePlaybackError(w, err)
		} else {
			slog.Error("subtitle stream ended after response started", "subtitle_id", id, "error", err)
		}
	}
}

func (h *PlaybackHandler) StreamSubtitleAss(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	if _, ok := h.subtitleForUser(w, user, id); !ok {
		return
	}
	httputil.ClearStreamWriteDeadline(w)
	var bodyStarted bool
	tw := &trackStreamBody{ResponseWriter: w, started: &bodyStarted}
	if err := db.HandleStreamSubtitleAss(tw, r, h.DB, id); err != nil {
		if !bodyStarted {
			writePlaybackError(w, err)
		} else {
			slog.Error("subtitle ass stream ended after response started", "subtitle_id", id, "error", err)
		}
	}
}

func (h *PlaybackHandler) ServeThumbnail(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	if _, ok := h.mediaItemForUser(w, user, id); !ok {
		return
	}
	httputil.ClearStreamWriteDeadline(w)
	if err := db.HandleServeThumbnail(w, r, h.DB, id, h.ThumbDir); err != nil {
		writePlaybackError(w, err)
	}
}

func (h *PlaybackHandler) ServeArtwork(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	user := UserFromContext(r.Context())
	if _, ok := h.mediaItemForUser(w, user, id); !ok {
		return
	}
	httputil.ClearStreamWriteDeadline(w)
	kind := chi.URLParam(r, "kind")
	if err := db.HandleServeArtwork(w, r, h.DB, id, h.ArtDir, kind); err != nil {
		writePlaybackError(w, err)
	}
}

func (h *PlaybackHandler) ServeShowArtwork(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	libraryID, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	var ownerID int
	if err := h.DB.QueryRow(`SELECT user_id FROM libraries WHERE id = ?`, libraryID).Scan(&ownerID); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if ownerID != user.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	showKey := chi.URLParam(r, "showKey")
	target, err := db.GetShowArtworkTarget(h.DB, libraryID, showKey)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if target == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	httputil.ClearStreamWriteDeadline(w)
	if err := db.HandleServeShowArtwork(w, r, h.DB, target.ID, h.ArtDir, "poster", target.PosterPath); err != nil {
		writePlaybackError(w, err)
	}
}

// trackStreamBody sets *started when non-empty body bytes are written so handlers can avoid
// http.Error after a chunked response has begun (would corrupt output and trigger superfluous WriteHeader).
type trackStreamBody struct {
	http.ResponseWriter
	started *bool
}

func (t *trackStreamBody) Write(p []byte) (int, error) {
	n, err := t.ResponseWriter.Write(p)
	if n > 0 && t.started != nil {
		*t.started = true
	}
	return n, err
}

func (t *trackStreamBody) Flush() {
	if fl, ok := t.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

func parsePathInt(w http.ResponseWriter, raw string, message string) (int, bool) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		http.Error(w, message, http.StatusBadRequest)
		return 0, false
	}
	return value, true
}

func writePlaybackError(w http.ResponseWriter, err error) {
	var burnErr transcoder.BurnSubtitleError
	if errors.As(err, &burnErr) {
		httputil.WritePlumJSONError(w, http.StatusBadRequest, "invalid_burn_subtitle", burnErr.Error())
		return
	}
	if errors.Is(err, db.ErrNotFound) {
		httputil.WritePlumJSONError(w, http.StatusNotFound, "not_found", err.Error())
		return
	}
	var statusErr *db.StatusError
	if errors.As(err, &statusErr) {
		code := httputil.PlumErrorCodeFromHTTPStatus(statusErr.Status)
		httputil.WritePlumJSONError(w, statusErr.Status, code, statusErr.Message)
		return
	}
	httputil.WritePlumJSONError(w, http.StatusInternalServerError, "internal_error", err.Error())
}
