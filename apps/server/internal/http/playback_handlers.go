package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"plum/internal/db"
	"plum/internal/transcoder"
)

type PlaybackHandler struct {
	DB       *sql.DB
	Sessions *transcoder.PlaybackSessionManager
	ThumbDir string
	ArtDir   string
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
	media, err := db.GetMediaByID(h.DB, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if media == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
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
	user := UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload struct {
		AudioIndex         int                                   `json:"audioIndex"`
		ClientCapabilities transcoder.ClientPlaybackCapabilities `json:"clientCapabilities"`
	}
	payload.AudioIndex = -1
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
	}

	state, err := h.Sessions.Create(*media, settings, payload.AudioIndex, user.ID, payload.ClientCapabilities)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(state)
}

func (h *PlaybackHandler) RefreshPlaybackTracks(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	media, err := db.GetMediaByID(h.DB, id)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if media == nil {
		http.Error(w, "not found", http.StatusNotFound)
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

func (h *PlaybackHandler) UpdateSessionAudio(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	var payload struct {
		AudioIndex int `json:"audioIndex"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
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
	h.Sessions.Close(chi.URLParam(r, "sessionId"))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "closed"})
}

func (h *PlaybackHandler) ServeSessionRevision(w http.ResponseWriter, r *http.Request) {
	revision, ok := parsePathInt(w, chi.URLParam(r, "revision"), "invalid revision")
	if !ok {
		return
	}
	if err := h.Sessions.ServeFile(w, r, chi.URLParam(r, "sessionId"), revision, chi.URLParam(r, "*")); err != nil {
		writePlaybackError(w, err)
	}
}

func (h *PlaybackHandler) StreamMedia(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	if err := db.HandleStreamMedia(w, r, h.DB, id); err != nil {
		writePlaybackError(w, err)
	}
}

func (h *PlaybackHandler) StreamEmbeddedSubtitle(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	streamIndex, ok := parsePathInt(w, chi.URLParam(r, "index"), "invalid index")
	if !ok {
		return
	}
	if err := db.HandleStreamEmbeddedSubtitle(w, r, h.DB, id, streamIndex); err != nil {
		writePlaybackError(w, err)
	}
}

func (h *PlaybackHandler) StreamSubtitle(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	if err := db.HandleStreamSubtitle(w, r, h.DB, id); err != nil {
		writePlaybackError(w, err)
	}
}

func (h *PlaybackHandler) ServeThumbnail(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
	if err := db.HandleServeThumbnail(w, r, h.DB, id, h.ThumbDir); err != nil {
		writePlaybackError(w, err)
	}
}

func (h *PlaybackHandler) ServeArtwork(w http.ResponseWriter, r *http.Request) {
	id, ok := parsePathInt(w, chi.URLParam(r, "id"), "invalid id")
	if !ok {
		return
	}
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
	if err := db.HandleServeShowArtwork(w, r, h.DB, target.ID, h.ArtDir, "poster", target.PosterPath); err != nil {
		writePlaybackError(w, err)
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
	status := http.StatusInternalServerError
	if errors.Is(err, db.ErrNotFound) {
		status = http.StatusNotFound
	}
	var statusErr *db.StatusError
	if errors.As(err, &statusErr) {
		status = statusErr.Status
	}
	http.Error(w, err.Error(), status)
}
