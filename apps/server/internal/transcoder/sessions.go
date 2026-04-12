package transcoder

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"plum/internal/db"
	"plum/internal/httputil"
	"plum/internal/ws"
)

// ErrTooManyPlaybackSessions is returned when a user already holds the configured maximum
// number of concurrent playback sessions (see PLUM_MAX_PLAYBACK_SESSIONS_PER_USER).
var ErrTooManyPlaybackSessions = errors.New("too many concurrent playback sessions")

var ffmpegCommandContext = exec.CommandContext
var previousRevisionCancelDelay = 20 * time.Second
var playbackDisconnectGracePeriod = 10 * time.Second
var playbackCloseWait = 2 * time.Second

func maxPlaybackSessionsPerUser() int {
	raw := strings.TrimSpace(os.Getenv("PLUM_MAX_PLAYBACK_SESSIONS_PER_USER"))
	if raw == "" {
		return 3
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 3
	}
	return n
}

type PlaybackSessionState struct {
	SessionID                       string                         `json:"sessionId,omitempty"`
	Delivery                        string                         `json:"delivery"`
	MediaID                         int                            `json:"mediaId"`
	Revision                        int                            `json:"revision,omitempty"`
	AudioIndex                      int                            `json:"audioIndex,omitempty"`
	Status                          string                         `json:"status"`
	StreamURL                       string                         `json:"streamUrl"`
	DurationSeconds                 int                            `json:"durationSeconds"`
	StreamOffsetSeconds             float64                        `json:"streamOffsetSeconds"`
	Subtitles                       []db.Subtitle                  `json:"subtitles,omitempty"`
	EmbeddedSubtitles               []PlaybackEmbeddedSubtitleJSON `json:"embeddedSubtitles,omitempty"`
	EmbeddedAudioTracks             []db.EmbeddedAudioTrack        `json:"embeddedAudioTracks,omitempty"`
	MediaAttachments                []db.MediaAttachment           `json:"mediaAttachments,omitempty"`
	BurnEmbeddedSubtitleStreamIndex *int                           `json:"burnEmbeddedSubtitleStreamIndex,omitempty"`
	Error                           string                         `json:"error,omitempty"`
	IntroStartSeconds               *float64                       `json:"intro_start_seconds,omitempty"`
	IntroEndSeconds                 *float64                       `json:"intro_end_seconds,omitempty"`
	CreditsStartSeconds             *float64                       `json:"credits_start_seconds,omitempty"`
	CreditsEndSeconds               *float64                       `json:"credits_end_seconds,omitempty"`
}

// MarshalWSPayload serialises the state fields that belong in a
// "playback_session_update" WebSocket frame. Using this single method
// for both the broadcast path and the attach-replay path guarantees the
// two frames are always identical.
//
// The JSON shape is the contract of record alongside PlaybackSessionUpdateEventSchema in
// @plum/contracts (packages/contracts) and Android PlaybackSessionUpdateEventJson.
func (s PlaybackSessionState) MarshalWSPayload() ([]byte, error) {
	type wsPayload struct {
		Type                            string   `json:"type"`
		SessionID                       string   `json:"sessionId"`
		Delivery                        string   `json:"delivery"`
		MediaID                         int      `json:"mediaId"`
		Revision                        int      `json:"revision"`
		AudioIndex                      int      `json:"audioIndex"`
		Status                          string   `json:"status"`
		StreamURL                       string   `json:"streamUrl"`
		DurationSeconds                 int      `json:"durationSeconds"`
		StreamOffsetSeconds             float64  `json:"streamOffsetSeconds"`
		Error                           string   `json:"error,omitempty"`
		BurnEmbeddedSubtitleStreamIndex *int     `json:"burnEmbeddedSubtitleStreamIndex,omitempty"`
		IntroStartSeconds               *float64 `json:"intro_start_seconds,omitempty"`
		IntroEndSeconds                 *float64 `json:"intro_end_seconds,omitempty"`
		CreditsStartSeconds             *float64 `json:"credits_start_seconds,omitempty"`
		CreditsEndSeconds               *float64 `json:"credits_end_seconds,omitempty"`
	}
	return json.Marshal(wsPayload{
		Type:                            "playback_session_update",
		SessionID:                       s.SessionID,
		Delivery:                        s.Delivery,
		MediaID:                         s.MediaID,
		Revision:                        s.Revision,
		AudioIndex:                      s.AudioIndex,
		Status:                          s.Status,
		StreamURL:                       s.StreamURL,
		DurationSeconds:                 s.DurationSeconds,
		StreamOffsetSeconds:             s.StreamOffsetSeconds,
		Error:                           s.Error,
		BurnEmbeddedSubtitleStreamIndex: s.BurnEmbeddedSubtitleStreamIndex,
		IntroStartSeconds:               s.IntroStartSeconds,
		IntroEndSeconds:                 s.IntroEndSeconds,
		CreditsStartSeconds:             s.CreditsStartSeconds,
		CreditsEndSeconds:               s.CreditsEndSeconds,
	})
}

type playbackRevision struct {
	number             int
	delivery           string
	audioIndex         int
	dir                string
	streamURL          string
	startOffsetSeconds float64
	status             string
	err                string
	cancel             context.CancelFunc
	done               chan struct{}
	readySent          bool
	// Frozen subtitle rendition set for this revision, captured at revision creation.
	subtitleTracksByPlaylistFile map[string]HlsWebSubtitle
	subtitleTracksForMaster      []HlsWebSubtitle
	// HLS event playlist path relative to revision.dir (e.g. "main.m3u8" when index.m3u8 is a bootstrap multivariant master).
	segmentMediaPlaylistName string

	masterMu          sync.Mutex
	cachedMaster      string
	cachedMasterMTime time.Time
	cachedMasterBurn  bool
}

type playbackSession struct {
	mu                         sync.Mutex
	id                         string
	userID                     int
	media                      db.MediaItem
	durationSeconds            int
	streamOffsetSeconds        float64
	capabilities               ClientPlaybackCapabilities
	audioIndex                 int
	activeRevision             int
	desiredRevision            int
	revisions                  map[int]*playbackRevision
	ownerClientID              string
	disconnectTimer            *time.Timer
	burnEmbeddedSubtitleStream *int // non-nil when subtitles are burned into the transcoded video
}

func attachIntroFields(state *PlaybackSessionState, media db.MediaItem) {
	if media.IntroStartSeconds != nil {
		v := *media.IntroStartSeconds
		state.IntroStartSeconds = &v
	}
	if media.IntroEndSeconds != nil {
		v := *media.IntroEndSeconds
		state.IntroEndSeconds = &v
	}
	if media.CreditsStartSeconds != nil {
		v := *media.CreditsStartSeconds
		state.CreditsStartSeconds = &v
	}
	if media.CreditsEndSeconds != nil {
		v := *media.CreditsEndSeconds
		state.CreditsEndSeconds = &v
	}
}

// PlaybackSessionManager coordinates transcode sessions and WebSocket updates.
//
// Locking (static review): m.mu protects the sessions and clients maps; each session.mu
// protects that session's fields and its revisions map. ServeFile holds m.mu as RLock
// briefly, then session.mu for revision lookup. runRevision avoids holding session.mu
// across ffmpeg I/O; markRevisionReady updates state under session.mu then releases
// before json.Marshal and hub fan-out. Before widening locks or sharding, confirm with
// mutex or block pprof under realistic concurrent playback.
type PlaybackSessionManager struct {
	shutdownCtx context.Context
	root        string
	hub         *ws.Hub

	mu       sync.RWMutex
	sessions map[string]*playbackSession
	clients  map[string]string
}

func NewPlaybackSessionManager(shutdownCtx context.Context, root string, hub *ws.Hub) *PlaybackSessionManager {
	if shutdownCtx == nil {
		shutdownCtx = context.Background()
	}
	return &PlaybackSessionManager{
		shutdownCtx: shutdownCtx,
		root:        root,
		hub:         hub,
		sessions:    make(map[string]*playbackSession),
		clients:     make(map[string]string),
	}
}

// ShutdownContext is cancelled when the server begins graceful shutdown. Pass it to
// background work so ffmpeg and DB calls stop promptly instead of using context.Background().
func (m *PlaybackSessionManager) ShutdownContext() context.Context {
	if m == nil {
		return context.Background()
	}
	if m.shutdownCtx == nil {
		return context.Background()
	}
	return m.shutdownCtx
}

// EnsureSessionOwner returns ErrNotFound if the session does not exist or belongs to another user.
func (m *PlaybackSessionManager) EnsureSessionOwner(sessionID string, userID int) error {
	m.mu.RLock()
	session := m.sessions[sessionID]
	m.mu.RUnlock()
	if session == nil {
		return db.ErrNotFound
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.userID != userID {
		return db.ErrNotFound
	}
	return nil
}

func (m *PlaybackSessionManager) countSessionsForUser(userID int) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	for _, s := range m.sessions {
		if s.userID != userID {
			continue
		}
		s.mu.Lock()
		active := s.countsTowardPlaybackLimitLocked()
		s.mu.Unlock()
		if active {
			n++
		}
	}
	return n
}

// countsTowardPlaybackLimitLocked reports whether a session should be included in the
// per-user concurrency limit and active-session listings.
//
// Terminal-error sessions stay in memory long enough for clients to observe the failure,
// but they should not block a fresh retry or clutter the active-session view.
func (s *playbackSession) countsTowardPlaybackLimitLocked() bool {
	for _, revision := range s.revisions {
		if revision == nil {
			continue
		}
		switch revision.status {
		case "starting", "ready":
			return true
		}
	}
	return false
}

// ActiveSessionIDSet returns the set of in-memory playback session IDs (transcode workdirs use these names).
func (m *PlaybackSessionManager) ActiveSessionIDSet() map[string]struct{} {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]struct{}, len(m.sessions))
	for id, session := range m.sessions {
		session.mu.Lock()
		active := session.countsTowardPlaybackLimitLocked()
		session.mu.Unlock()
		if active {
			out[id] = struct{}{}
		}
	}
	return out
}

// ActivePlaybackSessionAdmin describes an active session for admin dashboards.
type ActivePlaybackSessionAdmin struct {
	SessionID       string `json:"sessionId"`
	UserID          int    `json:"userId"`
	MediaID         int    `json:"mediaId"`
	Title           string `json:"title"`
	LibraryID       int    `json:"libraryId"`
	Kind            string `json:"kind"` // mirrors media type (movie, tv_episode, etc.)
	Delivery        string `json:"delivery"`
	Status          string `json:"status"`
	DurationSeconds int    `json:"durationSeconds"`
}

// ListActiveSessionsForAdmin returns a snapshot of all in-memory playback sessions.
func (m *PlaybackSessionManager) ListActiveSessionsForAdmin() []ActivePlaybackSessionAdmin {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	type pair struct {
		id string
		s  *playbackSession
	}
	pairs := make([]pair, 0, len(m.sessions))
	for id, s := range m.sessions {
		pairs = append(pairs, pair{id: id, s: s})
	}
	m.mu.RUnlock()

	out := make([]ActivePlaybackSessionAdmin, 0, len(pairs))
	for _, item := range pairs {
		session := item.s
		session.mu.Lock()
		if !session.countsTowardPlaybackLimitLocked() {
			session.mu.Unlock()
			continue
		}
		media := session.media
		userID := session.userID
		duration := session.durationSeconds
		delivery := ""
		status := ""
		if rev := session.revisions[session.activeRevision]; rev != nil {
			delivery = rev.delivery
			status = rev.status
		}
		session.mu.Unlock()
		out = append(out, ActivePlaybackSessionAdmin{
			SessionID:       item.id,
			UserID:          userID,
			MediaID:         media.ID,
			Title:           media.Title,
			LibraryID:       media.LibraryID,
			Kind:            media.Type,
			Delivery:        delivery,
			Status:          status,
			DurationSeconds: duration,
		})
	}
	return out
}

// Shutdown cancels all in-flight transcodes. It does not remove session records or temp dirs;
// use Close(sessionID) per session for that. Safe to call more than once.
func (m *PlaybackSessionManager) Shutdown() {
	m.mu.RLock()
	sessions := make([]*playbackSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.RUnlock()

	for _, session := range sessions {
		session.mu.Lock()
		for _, rev := range session.revisions {
			if rev != nil && rev.cancel != nil {
				rev.cancel()
			}
		}
		session.mu.Unlock()
	}
}

func (m *PlaybackSessionManager) Create(
	ctx context.Context,
	media db.MediaItem,
	settings db.TranscodingSettings,
	audioIndex int,
	userID int,
	capabilities ClientPlaybackCapabilities,
	burnEmbeddedSubtitleStreamIndex *int,
) (PlaybackSessionState, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	probe, err := probePlaybackSource(ctx, media.Path)
	if err != nil {
		slog.Warn("playback probe failed", "media_id", media.ID, "path", media.Path, "error", err)
	}
	durationSeconds := resolvePlaybackDurationSeconds(media.Duration, probe.DurationSeconds)

	if burnEmbeddedSubtitleStreamIndex != nil && *burnEmbeddedSubtitleStreamIndex >= 0 {
		if valErr := ValidateBurnEmbeddedSubtitle(probe, media, *burnEmbeddedSubtitleStreamIndex); valErr != nil {
			return PlaybackSessionState{}, valErr
		}
	}

	decision := decidePlayback(media.ID, probe, capabilities, audioIndex, burnEmbeddedSubtitleStreamIndex)

	var burnStored *int
	if burnEmbeddedSubtitleStreamIndex != nil && *burnEmbeddedSubtitleStreamIndex >= 0 {
		v := *burnEmbeddedSubtitleStreamIndex
		burnStored = &v
	}

	if decision.Delivery == "direct" {
		state := PlaybackSessionState{
			Delivery:                        "direct",
			MediaID:                         media.ID,
			AudioIndex:                      audioIndex,
			Status:                          "ready",
			StreamURL:                       decision.StreamURL,
			DurationSeconds:                 durationSeconds,
			Subtitles:                       media.Subtitles,
			EmbeddedSubtitles:               embeddedSubtitlesForPlaybackJSON(media, decision.Delivery),
			EmbeddedAudioTracks:             media.EmbeddedAudioTracks,
			MediaAttachments:                media.MediaAttachments,
			BurnEmbeddedSubtitleStreamIndex: burnStreamJSON(burnStored),
		}
		attachIntroFields(&state, media)
		return state, nil
	}

	m.mu.Lock()
	if lim := maxPlaybackSessionsPerUser(); lim > 0 {
		n := 0
		for _, s := range m.sessions {
			if s.userID != userID {
				continue
			}
			s.mu.Lock()
			active := s.countsTowardPlaybackLimitLocked()
			s.mu.Unlock()
			if active {
				n++
			}
		}
		if n >= lim {
			m.mu.Unlock()
			return PlaybackSessionState{}, ErrTooManyPlaybackSessions
		}
	}
	sessionID, err := newPlaybackSessionID()
	if err != nil {
		m.mu.Unlock()
		return PlaybackSessionState{}, err
	}

	session := &playbackSession{
		id:                         sessionID,
		userID:                     userID,
		media:                      media,
		durationSeconds:            durationSeconds,
		capabilities:               capabilities,
		audioIndex:                 audioIndex,
		activeRevision:             0,
		desiredRevision:            0,
		revisions:                  make(map[int]*playbackRevision),
		burnEmbeddedSubtitleStream: burnStored,
	}

	m.sessions[sessionID] = session
	m.mu.Unlock()

	burnLog := -1
	if burnStored != nil {
		burnLog = *burnStored
	}
	slog.Info("playback session create",
		"session_id", sessionID,
		"media_id", media.ID,
		"audio_index", audioIndex,
		"delivery", decision.Delivery,
		"burn_sub", burnLog,
	)

	return m.startRevision(session, settings, audioIndex, decision, &probe)
}

func (m *PlaybackSessionManager) UpdateAudio(sessionID string, settings db.TranscodingSettings, audioIndex int) (PlaybackSessionState, error) {
	m.mu.RLock()
	session := m.sessions[sessionID]
	m.mu.RUnlock()
	if session == nil {
		return PlaybackSessionState{}, db.ErrNotFound
	}
	probe, err := probePlaybackSource(m.shutdownCtx, session.media.Path)
	if err != nil {
		slog.Warn("playback probe failed", "media_id", session.media.ID, "path", session.media.Path, "error", err)
	}
	durationSeconds := resolvePlaybackDurationSeconds(session.media.Duration, probe.DurationSeconds)
	session.mu.Lock()
	if durationSeconds > 0 {
		session.durationSeconds = durationSeconds
	} else {
		durationSeconds = session.durationSeconds
	}
	burnPtr := session.burnEmbeddedSubtitleStream
	session.mu.Unlock()
	decision := decidePlayback(session.media.ID, probe, session.capabilities, audioIndex, burnPtr)
	if decision.Delivery == "direct" {
		m.Close(sessionID)
		state := PlaybackSessionState{
			Delivery:                        "direct",
			MediaID:                         session.media.ID,
			AudioIndex:                      audioIndex,
			Status:                          "ready",
			StreamURL:                       decision.StreamURL,
			DurationSeconds:                 durationSeconds,
			Subtitles:                       session.media.Subtitles,
			EmbeddedSubtitles:               embeddedSubtitlesForPlaybackJSON(session.media, decision.Delivery),
			EmbeddedAudioTracks:             session.media.EmbeddedAudioTracks,
			MediaAttachments:                session.media.MediaAttachments,
			BurnEmbeddedSubtitleStreamIndex: burnStreamJSON(burnPtr),
		}
		attachIntroFields(&state, session.media)
		return state, nil
	}
	return m.startRevision(session, settings, audioIndex, decision, &probe)
}

func (m *PlaybackSessionManager) Seek(sessionID string, settings db.TranscodingSettings, positionSeconds float64) (PlaybackSessionState, error) {
	m.mu.RLock()
	session := m.sessions[sessionID]
	m.mu.RUnlock()
	if session == nil {
		return PlaybackSessionState{}, db.ErrNotFound
	}
	if math.IsNaN(positionSeconds) || math.IsInf(positionSeconds, 0) {
		positionSeconds = 0
	}
	probe, err := probePlaybackSource(m.shutdownCtx, session.media.Path)
	if err != nil {
		slog.Warn("playback probe failed", "media_id", session.media.ID, "path", session.media.Path, "error", err)
	}
	durationSeconds := resolvePlaybackDurationSeconds(session.media.Duration, probe.DurationSeconds)
	session.mu.Lock()
	if durationSeconds > 0 {
		session.durationSeconds = durationSeconds
	} else {
		durationSeconds = session.durationSeconds
	}
	audioIndex := session.audioIndex
	burnPtr := session.burnEmbeddedSubtitleStream
	session.mu.Unlock()

	maxPosition := float64(durationSeconds)
	if maxPosition > 0 {
		positionSeconds = math.Min(positionSeconds, math.Max(0, maxPosition-0.05))
	}
	positionSeconds = math.Max(0, positionSeconds)
	decision := decidePlayback(session.media.ID, probe, session.capabilities, audioIndex, burnPtr)
	if decision.Delivery == "direct" {
		state := PlaybackSessionState{
			Delivery:                        "direct",
			MediaID:                         session.media.ID,
			AudioIndex:                      audioIndex,
			Status:                          "ready",
			StreamURL:                       decision.StreamURL,
			DurationSeconds:                 durationSeconds,
			Subtitles:                       session.media.Subtitles,
			EmbeddedSubtitles:               embeddedSubtitlesForPlaybackJSON(session.media, decision.Delivery),
			EmbeddedAudioTracks:             session.media.EmbeddedAudioTracks,
			MediaAttachments:                session.media.MediaAttachments,
			BurnEmbeddedSubtitleStreamIndex: burnStreamJSON(burnPtr),
		}
		attachIntroFields(&state, session.media)
		return state, nil
	}
	return m.startRevisionAt(session, settings, audioIndex, decision, &probe, positionSeconds)
}

func (m *PlaybackSessionManager) Attach(sessionID string, userID int, clientID string) (*PlaybackSessionState, error) {
	m.mu.Lock()
	session := m.sessions[sessionID]
	if session == nil {
		m.mu.Unlock()
		return nil, db.ErrNotFound
	}

	session.mu.Lock()
	if session.userID != userID {
		session.mu.Unlock()
		m.mu.Unlock()
		return nil, db.ErrNotFound
	}
	previousOwner := session.ownerClientID
	if session.disconnectTimer != nil {
		session.disconnectTimer.Stop()
		session.disconnectTimer = nil
	}
	session.ownerClientID = clientID
	replayState := session.stateForReplayLocked()
	session.mu.Unlock()

	if previousSessionID, ok := m.clients[clientID]; ok && previousSessionID != sessionID {
		if previous := m.sessions[previousSessionID]; previous != nil {
			m.scheduleDisconnectLocked(previous, userID, clientID)
		}
	}
	m.clients[clientID] = sessionID
	if previousOwner != "" && previousOwner != clientID {
		if ownedSessionID, ok := m.clients[previousOwner]; ok && ownedSessionID == sessionID {
			delete(m.clients, previousOwner)
		}
	}
	m.mu.Unlock()

	slog.Debug("playback session attach", "session_id", sessionID, "user_id", userID, "client_id", clientID)
	return replayState, nil
}

func (m *PlaybackSessionManager) Detach(sessionID string, userID int, clientID string) {
	m.mu.Lock()
	session := m.sessions[sessionID]
	if session == nil {
		m.mu.Unlock()
		return
	}
	m.scheduleDisconnectLocked(session, userID, clientID)
	m.mu.Unlock()
}

func (m *PlaybackSessionManager) HandleDisconnect(userID int, clientID string) {
	m.mu.Lock()
	sessionID := m.clients[clientID]
	if sessionID == "" {
		m.mu.Unlock()
		return
	}
	session := m.sessions[sessionID]
	if session == nil {
		delete(m.clients, clientID)
		m.mu.Unlock()
		return
	}
	m.scheduleDisconnectLocked(session, userID, clientID)
	m.mu.Unlock()
}

func (m *PlaybackSessionManager) Close(sessionID string) {
	m.mu.Lock()
	session := m.sessions[sessionID]
	delete(m.sessions, sessionID)
	m.mu.Unlock()
	if session == nil {
		return
	}

	session.mu.Lock()
	revisions := make([]*playbackRevision, 0, len(session.revisions))
	for _, revision := range session.revisions {
		revisions = append(revisions, revision)
	}
	if session.disconnectTimer != nil {
		session.disconnectTimer.Stop()
		session.disconnectTimer = nil
	}
	activeRevision := session.activeRevision
	audioIndex := session.audioIndex
	mediaID := session.media.ID
	durationSeconds := session.durationSeconds
	streamOffsetSeconds := session.streamOffsetSeconds
	ownerClientID := session.ownerClientID
	delivery := "transcode"
	if active := session.revisions[activeRevision]; active != nil && active.delivery != "" {
		delivery = active.delivery
	}
	burnClosed := session.burnEmbeddedSubtitleStream
	session.ownerClientID = ""
	session.mu.Unlock()

	if ownerClientID != "" {
		m.mu.Lock()
		if ownedSessionID, ok := m.clients[ownerClientID]; ok && ownedSessionID == sessionID {
			delete(m.clients, ownerClientID)
		}
		m.mu.Unlock()
	}

	for _, revision := range revisions {
		if revision.cancel != nil {
			revision.cancel()
		}
	}
	var wg sync.WaitGroup
	deadline := time.Now().Add(playbackCloseWait)
	for _, revision := range revisions {
		wg.Add(1)
		go func(done <-chan struct{}) {
			defer wg.Done()
			select {
			case <-done:
			case <-time.After(time.Until(deadline)):
			}
		}(revision.done)
	}
	wg.Wait()
	_ = os.RemoveAll(filepath.Join(m.root, sessionID))
	closed := PlaybackSessionState{
		SessionID:                       sessionID,
		Delivery:                        delivery,
		MediaID:                         mediaID,
		Revision:                        activeRevision,
		AudioIndex:                      audioIndex,
		Status:                          "closed",
		DurationSeconds:                 durationSeconds,
		StreamOffsetSeconds:             streamOffsetSeconds,
		Subtitles:                       session.media.Subtitles,
		EmbeddedSubtitles:               embeddedSubtitlesForPlaybackJSON(session.media, delivery),
		EmbeddedAudioTracks:             session.media.EmbeddedAudioTracks,
		MediaAttachments:                session.media.MediaAttachments,
		BurnEmbeddedSubtitleStreamIndex: burnStreamJSON(burnClosed),
	}
	attachIntroFields(&closed, session.media)
	m.broadcast(closed)
}

func (s *playbackSession) stateForReplayLocked() *PlaybackSessionState {
	candidates := []int{s.desiredRevision, s.activeRevision}
	seen := make(map[int]struct{}, len(candidates))
	for _, revisionNumber := range candidates {
		if revisionNumber <= 0 {
			continue
		}
		if _, ok := seen[revisionNumber]; ok {
			continue
		}
		seen[revisionNumber] = struct{}{}
		revision := s.revisions[revisionNumber]
		if revision == nil {
			continue
		}
		if revision.status != "ready" && revision.status != "error" {
			continue
		}
		replay := PlaybackSessionState{
			SessionID:                       s.id,
			Delivery:                        revision.delivery,
			MediaID:                         s.media.ID,
			Revision:                        revision.number,
			AudioIndex:                      revision.audioIndex,
			Status:                          revision.status,
			StreamURL:                       revision.streamURL,
			DurationSeconds:                 s.durationSeconds,
			StreamOffsetSeconds:             revision.startOffsetSeconds,
			Subtitles:                       s.media.Subtitles,
			EmbeddedSubtitles:               embeddedSubtitlesForPlaybackJSON(s.media, revision.delivery),
			EmbeddedAudioTracks:             s.media.EmbeddedAudioTracks,
			MediaAttachments:                s.media.MediaAttachments,
			BurnEmbeddedSubtitleStreamIndex: burnStreamJSON(s.burnEmbeddedSubtitleStream),
			Error:                           revision.err,
		}
		attachIntroFields(&replay, s.media)
		return &replay
	}
	return nil
}

func freezeRevisionSubtitleTracks(media db.MediaItem) (map[string]HlsWebSubtitle, []HlsWebSubtitle) {
	collected := CollectHlsWebSubtitles(media)
	if len(collected) == 0 {
		return nil, nil
	}
	byPlaylist := make(map[string]HlsWebSubtitle, len(collected))
	for _, track := range collected {
		byPlaylist[filepath.Base(track.PlaylistFile)] = track
	}
	forMaster := make([]HlsWebSubtitle, len(collected))
	copy(forMaster, collected)
	return byPlaylist, forMaster
}

func frozenRevisionSubtitleTracksForMaster(revision *playbackRevision, burning bool) []HlsWebSubtitle {
	if revision == nil || burning || len(revision.subtitleTracksForMaster) == 0 {
		return nil
	}
	out := make([]HlsWebSubtitle, len(revision.subtitleTracksForMaster))
	copy(out, revision.subtitleTracksForMaster)
	return out
}

func serveVirtualHlsSubtitlePlaylist(w http.ResponseWriter, revision *playbackRevision, durationSeconds int, baseName string) error {
	if revision == nil || len(revision.subtitleTracksByPlaylistFile) == 0 {
		return db.ErrNotFound
	}
	picked, ok := revision.subtitleTracksByPlaylistFile[baseName]
	if !ok {
		return db.ErrNotFound
	}
	body := BuildWebVttSubtitleMediaPlaylist(picked.VTTPath, durationSeconds)
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-store")
	_, err := w.Write([]byte(body))
	return err
}

func (m *PlaybackSessionManager) ServeFile(w http.ResponseWriter, r *http.Request, sessionID string, revisionNumber int, name string) error {
	httputil.ClearStreamWriteDeadline(w)

	m.mu.RLock()
	session := m.sessions[sessionID]
	m.mu.RUnlock()
	if session == nil {
		return db.ErrNotFound
	}

	session.mu.Lock()
	revision := session.revisions[revisionNumber]
	session.mu.Unlock()
	if revision == nil {
		return db.ErrNotFound
	}

	cleanName := filepath.Clean("/" + name)
	if cleanName == "/" {
		return db.ErrNotFound
	}
	relFromRoot := strings.TrimPrefix(cleanName, "/")
	target := filepath.Join(revision.dir, relFromRoot)
	if !strings.HasPrefix(target, revision.dir+string(filepath.Separator)) {
		return db.ErrNotFound
	}

	baseName := filepath.Base(relFromRoot)
	if _, ok := ParseVirtualSubtitlePlaylistName(baseName); ok {
		session.mu.Lock()
		dur := session.durationSeconds
		session.mu.Unlock()
		return serveVirtualHlsSubtitlePlaylist(w, revision, dur, baseName)
	}

	if err := waitForPlaybackFile(r.Context(), target); err != nil {
		if os.IsNotExist(err) {
			return db.ErrNotFound
		}
		return err
	}

	switch filepath.Ext(target) {
	case ".m3u8":
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	case ".ts":
		w.Header().Set("Content-Type", "video/mp2t")
	}
	w.Header().Set("Cache-Control", "no-store")

	if filepath.Ext(target) == ".m3u8" && baseName == "index.m3u8" && relFromRoot == "index.m3u8" {
		info, err := os.Stat(target)
		if err != nil {
			return err
		}
		session.mu.Lock()
		burning := session.burnEmbeddedSubtitleStream != nil
		session.mu.Unlock()

		revision.masterMu.Lock()
		hit := revision.cachedMaster != "" &&
			revision.cachedMasterMTime.Equal(info.ModTime()) &&
			revision.cachedMasterBurn == burning
		if hit {
			body := revision.cachedMaster
			revision.masterMu.Unlock()
			http.ServeContent(w, r, baseName, info.ModTime(), strings.NewReader(body))
			return nil
		}
		revision.masterMu.Unlock()

		raw, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		info, err = os.Stat(target)
		if err != nil {
			return err
		}
		tracks := frozenRevisionSubtitleTracksForMaster(revision, burning)
		out := InjectHlsSubtitleRenditions(string(raw), tracks)

		revision.masterMu.Lock()
		revision.cachedMaster = out
		revision.cachedMasterMTime = info.ModTime()
		revision.cachedMasterBurn = burning
		revision.masterMu.Unlock()

		http.ServeContent(w, r, baseName, info.ModTime(), strings.NewReader(out))
		return nil
	}

	file, err := os.Open(target)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	http.ServeContent(w, r, filepath.Base(target), info.ModTime(), file)
	return nil
}

func (m *PlaybackSessionManager) startRevision(
	session *playbackSession,
	settings db.TranscodingSettings,
	audioIndex int,
	decision playbackDecision,
	cachedProbe *playbackSourceProbe,
) (PlaybackSessionState, error) {
	return m.startRevisionAt(session, settings, audioIndex, decision, cachedProbe, 0)
}

func (m *PlaybackSessionManager) startRevisionAt(
	session *playbackSession,
	settings db.TranscodingSettings,
	audioIndex int,
	decision playbackDecision,
	cachedProbe *playbackSourceProbe,
	startOffsetSeconds float64,
) (PlaybackSessionState, error) {
	if math.IsNaN(startOffsetSeconds) || math.IsInf(startOffsetSeconds, 0) || startOffsetSeconds < 0 {
		startOffsetSeconds = 0
	}
	session.mu.Lock()
	session.desiredRevision += 1
	revisionNumber := session.desiredRevision
	session.audioIndex = audioIndex
	session.streamOffsetSeconds = startOffsetSeconds
	durationSeconds := session.durationSeconds

	for _, revision := range session.revisions {
		if revision.number > session.activeRevision && revision.number != revisionNumber && revision.cancel != nil {
			revision.cancel()
		}
	}

	dir := filepath.Join(m.root, session.id, fmt.Sprintf("revision_%d", revisionNumber))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		session.mu.Unlock()
		return PlaybackSessionState{}, err
	}

	ctx, cancel := context.WithCancel(m.shutdownCtx)
	revision := &playbackRevision{
		number:             revisionNumber,
		delivery:           decision.Delivery,
		audioIndex:         audioIndex,
		dir:                dir,
		streamURL:          fmt.Sprintf("/api/playback/sessions/%s/revisions/%d/index.m3u8", session.id, revisionNumber),
		startOffsetSeconds: startOffsetSeconds,
		status:             "starting",
		cancel:             cancel,
		done:               make(chan struct{}),
	}
	revision.subtitleTracksByPlaylistFile, revision.subtitleTracksForMaster = freezeRevisionSubtitleTracks(session.media)
	session.revisions[revisionNumber] = revision
	session.mu.Unlock()

	slog.Info("playback revision start",
		"session_id", session.id,
		"media_id", session.media.ID,
		"revision", revision.number,
		"audio_index", audioIndex,
		"delivery", decision.Delivery,
	)

	go m.runRevision(ctx, session, revision, settings, decision, cachedProbe)

	starting := PlaybackSessionState{
		SessionID:                       session.id,
		Delivery:                        revision.delivery,
		MediaID:                         session.media.ID,
		Revision:                        revision.number,
		AudioIndex:                      audioIndex,
		Status:                          revision.status,
		StreamURL:                       revision.streamURL,
		DurationSeconds:                 durationSeconds,
		StreamOffsetSeconds:             startOffsetSeconds,
		Subtitles:                       session.media.Subtitles,
		EmbeddedSubtitles:               embeddedSubtitlesForPlaybackJSON(session.media, revision.delivery),
		EmbeddedAudioTracks:             session.media.EmbeddedAudioTracks,
		MediaAttachments:                session.media.MediaAttachments,
		BurnEmbeddedSubtitleStreamIndex: burnStreamJSON(session.burnEmbeddedSubtitleStream),
	}
	attachIntroFields(&starting, session.media)
	return starting, nil
}

func (m *PlaybackSessionManager) scheduleDisconnectLocked(session *playbackSession, userID int, clientID string) {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.userID != userID || session.ownerClientID != clientID {
		return
	}
	if session.disconnectTimer != nil {
		session.disconnectTimer.Stop()
	}
	session.ownerClientID = ""
	delete(m.clients, clientID)
	sessionID := session.id
	session.disconnectTimer = time.AfterFunc(playbackDisconnectGracePeriod, func() {
		// Guard against race: if a new client attached while this goroutine was
		// blocked on m.mu, the session now has an active owner and must not be closed.
		m.mu.RLock()
		s := m.sessions[sessionID]
		m.mu.RUnlock()
		if s != nil {
			s.mu.Lock()
			hasOwner := s.ownerClientID != ""
			s.mu.Unlock()
			if hasOwner {
				return
			}
		}
		m.Close(sessionID)
	})
	slog.Info("playback session disconnect pending",
		"session_id", session.id,
		"user_id", userID,
		"client_id", clientID,
		"grace", playbackDisconnectGracePeriod,
	)
}

func (m *PlaybackSessionManager) runRevision(
	ctx context.Context,
	session *playbackSession,
	revision *playbackRevision,
	settings db.TranscodingSettings,
	decision playbackDecision,
	cachedProbe *playbackSourceProbe,
) {
	defer close(revision.done)
	var probe playbackSourceProbe
	var err error
	if cachedProbe != nil {
		probe = *cachedProbe
	} else {
		probe, err = probePlaybackSource(ctx, session.media.Path)
		if err != nil {
			slog.Warn("playback probe failed", "media_id", session.media.ID, "path", session.media.Path, "error", err)
		}
	}
	durationSeconds := resolvePlaybackDurationSeconds(session.media.Duration, probe.DurationSeconds)
	session.mu.Lock()
	if durationSeconds > 0 {
		session.durationSeconds = durationSeconds
	} else {
		durationSeconds = session.durationSeconds
	}
	session.mu.Unlock()
	readyDurationSeconds := math.Max(0, float64(durationSeconds)-revision.startOffsetSeconds)
	plans := buildPlaybackHLSPlans(session.media.Path, revision.dir, settings, probe, decision, revision.startOffsetSeconds, session.media)
	finalState := PlaybackSessionState{
		SessionID:                       session.id,
		Delivery:                        revision.delivery,
		MediaID:                         session.media.ID,
		Revision:                        revision.number,
		AudioIndex:                      revision.audioIndex,
		Status:                          "error",
		StreamURL:                       revision.streamURL,
		DurationSeconds:                 durationSeconds,
		StreamOffsetSeconds:             revision.startOffsetSeconds,
		Subtitles:                       session.media.Subtitles,
		EmbeddedSubtitles:               embeddedSubtitlesForPlaybackJSON(session.media, revision.delivery),
		EmbeddedAudioTracks:             session.media.EmbeddedAudioTracks,
		MediaAttachments:                session.media.MediaAttachments,
		BurnEmbeddedSubtitleStreamIndex: burnStreamJSON(session.burnEmbeddedSubtitleStream),
	}
	attachIntroFields(&finalState, session.media)

	for index, plan := range plans {
		if ctx.Err() != nil {
			return
		}

		slog.Info("playback revision ffmpeg start",
			"session_id", session.id,
			"media_id", session.media.ID,
			"revision", revision.number,
			"mode", plan.Mode,
			"attempt", index+1,
			"attempts", len(plans),
		)

		if err := os.RemoveAll(revision.dir); err == nil {
			_ = os.MkdirAll(revision.dir, 0o755)
		}

		revision.segmentMediaPlaylistName = ""
		if base := strings.TrimSpace(plan.RemuxHlsMediaPlaylistBase); base != "" {
			mediaPlaylist := base + ".m3u8"
			revision.segmentMediaPlaylistName = mediaPlaylist
			bootstrap := fmt.Sprintf("#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=%d\n%s\n", plan.RemuxHlsVariantStreamBandwidth, filepath.Base(mediaPlaylist))
			if err := os.WriteFile(filepath.Join(revision.dir, "index.m3u8"), []byte(bootstrap), 0o644); err != nil {
				finalState.Error = err.Error()
				slog.Error("playback revision bootstrap master write failed",
					"session_id", session.id,
					"media_id", session.media.ID,
					"revision", revision.number,
					"error", err,
				)
				continue
			}
		}

		// Quiet ffmpeg: default stderr is endless HLS progress (frame/size/time lines). Keep stderr
		// in-memory only and log it on failure via compactFFmpegError; avoid teeing to os.Stderr.
		playbackFFmpegArgs := append([]string{
			"-hide_banner",
			"-nostats",
			"-loglevel", "error",
		}, plan.Args...)
		cmd := ffmpegCommandContext(ctx, "ffmpeg", playbackFFmpegArgs...)
		var stderrBuf bytes.Buffer
		cmd.Stderr = &stderrBuf
		if err := cmd.Start(); err != nil {
			finalState.Error = err.Error()
			slog.Error("playback revision ffmpeg start failed",
				"session_id", session.id,
				"media_id", session.media.ID,
				"revision", revision.number,
				"mode", plan.Mode,
				"error", finalState.Error,
			)
			continue
		}

		waitCh := make(chan error, 1)
		go func() {
			waitCh <- cmd.Wait()
		}()

		ticker := time.NewTicker(250 * time.Millisecond)
		ready := false
	loop:
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				if cmd.Process != nil {
					_ = cmd.Process.Kill()
				}
				<-waitCh
				return
			case err := <-waitCh:
				ticker.Stop()
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					finalState.Error = compactFFmpegError(stderrBuf.String(), err)
					slog.Error("playback revision ffmpeg failed",
						"session_id", session.id,
						"media_id", session.media.ID,
						"revision", revision.number,
						"mode", plan.Mode,
						"ready", ready,
						"error", finalState.Error,
					)
					break loop
				}
				if !ready && revisionReady(revision.dir, readyDurationSeconds, revision.segmentMediaPlaylistName) {
					ready = true
					m.markRevisionReady(session, revision)
				}
				slog.Info("playback revision ffmpeg exited",
					"session_id", session.id,
					"media_id", session.media.ID,
					"revision", revision.number,
					"mode", plan.Mode,
					"ready", ready,
				)
				return
			case <-ticker.C:
				if !ready && revisionReady(revision.dir, readyDurationSeconds, revision.segmentMediaPlaylistName) {
					ready = true
					m.markRevisionReady(session, revision)
				}
			}
		}

		if plan.Mode == "hardware" && settings.AllowSoftwareFallback && index+1 < len(plans) && !ready {
			slog.Info("playback revision fallback",
				"session_id", session.id,
				"media_id", session.media.ID,
				"revision", revision.number,
				"from_mode", plan.Mode,
				"to_mode", plans[index+1].Mode,
			)
			continue
		}
		break
	}

	if finalState.Error == "" {
		finalState.Error = "playback stream failed"
	}
	session.mu.Lock()
	revision.status = "error"
	revision.err = finalState.Error
	session.mu.Unlock()
	slog.Error("playback revision error",
		"session_id", session.id,
		"media_id", session.media.ID,
		"revision", revision.number,
		"error", finalState.Error,
	)
	m.broadcast(finalState)
}

func (m *PlaybackSessionManager) markRevisionReady(session *playbackSession, revision *playbackRevision) {
	session.mu.Lock()
	if revision.readySent {
		session.mu.Unlock()
		return
	}
	revision.readySent = true
	revision.status = "ready"

	previousActive := session.activeRevision
	if revision.number == session.desiredRevision {
		session.activeRevision = revision.number
		session.audioIndex = revision.audioIndex
		session.streamOffsetSeconds = revision.startOffsetSeconds
	}
	activeRevision := session.activeRevision
	audioIndex := session.audioIndex
	mediaID := session.media.ID
	sessionID := session.id
	durationSeconds := session.durationSeconds
	burnReady := session.burnEmbeddedSubtitleStream
	session.mu.Unlock()

	ready := PlaybackSessionState{
		SessionID:                       sessionID,
		Delivery:                        revision.delivery,
		MediaID:                         mediaID,
		Revision:                        revision.number,
		AudioIndex:                      audioIndex,
		Status:                          "ready",
		StreamURL:                       revision.streamURL,
		DurationSeconds:                 durationSeconds,
		StreamOffsetSeconds:             revision.startOffsetSeconds,
		Subtitles:                       session.media.Subtitles,
		EmbeddedSubtitles:               embeddedSubtitlesForPlaybackJSON(session.media, revision.delivery),
		EmbeddedAudioTracks:             session.media.EmbeddedAudioTracks,
		MediaAttachments:                session.media.MediaAttachments,
		BurnEmbeddedSubtitleStreamIndex: burnStreamJSON(burnReady),
	}
	attachIntroFields(&ready, session.media)
	m.broadcast(ready)

	if previousActive > 0 && previousActive != activeRevision {
		session.mu.Lock()
		previous := session.revisions[previousActive]
		session.mu.Unlock()
		if previous != nil && previous.cancel != nil {
			delay := previousRevisionCancelDelay
			slog.Info("playback revision ready",
				"session_id", sessionID,
				"media_id", mediaID,
				"revision", revision.number,
				"previous_revision", previousActive,
				"cancel_delay", delay,
			)
			time.AfterFunc(delay, func() {
				previous.cancel()
			})
		}
	}
}

func (m *PlaybackSessionManager) broadcast(state PlaybackSessionState) {
	if m.hub == nil {
		return
	}
	payload, err := state.MarshalWSPayload()
	if err != nil {
		slog.Error("marshal playback session update", "error", err)
		return
	}
	m.hub.Broadcast(payload)
}

// revisionReady reports whether enough HLS media is on disk for the client to start without
// immediately stalling at the transcode live edge (previously we required only one segment).
func segmentHlsMediaPlaylistPath(dir string, segmentMediaPlaylistName string) string {
	name := strings.TrimSpace(segmentMediaPlaylistName)
	if name != "" {
		return filepath.Join(dir, filepath.Base(name))
	}
	if st, err := os.Stat(filepath.Join(dir, "variant_0")); err == nil && st.IsDir() {
		return filepath.Join(dir, "variant_0", "index.m3u8")
	}
	return filepath.Join(dir, "index.m3u8")
}

func revisionReady(dir string, durationSeconds float64, segmentMediaPlaylistName string) bool {
	playlistPath := segmentHlsMediaPlaylistPath(dir, segmentMediaPlaylistName)
	segCount := countHlsSegmentEntriesFromPlaylist(playlistPath)
	if segCount < 1 {
		segCount = countNonEmptyHlsSegments(filepath.Dir(playlistPath))
	}
	if segCount < 1 {
		return false
	}

	// Two segments (≈4s at 2s/seg) is enough for HLS event playlists — ExoPlayer/hls.js
	// will keep polling for more while the transcode continues. This gets playback started
	// much faster for large remux files (e.g. 80GB+ UHD remuxes).
	minSegments := 2
	if durationSeconds > 0 {
		needed := int(math.Ceil(durationSeconds / float64(hlsSegmentDurationSeconds)))
		if needed < 1 {
			needed = 1
		}
		if needed < minSegments {
			minSegments = needed
		}
	}

	return segCount >= minSegments
}

// countHlsSegmentEntriesFromPlaylist counts #EXTINF entries in the active media playlist (one read),
// which tracks what ffmpeg has committed to the playlist. Falls back to countNonEmptyHlsSegments when empty.
func countHlsSegmentEntriesFromPlaylist(playlist string) int {
	raw, err := os.ReadFile(playlist)
	if err != nil || len(raw) == 0 {
		return 0
	}
	return strings.Count(string(raw), "#EXTINF:")
}

// parseHlsSegmentIndex parses segment indices from ffmpeg HLS output names like "segment_00022.ts".
func parseHlsSegmentIndex(fileBase string) (index int, ok bool) {
	if !strings.HasPrefix(fileBase, "segment_") || !strings.HasSuffix(fileBase, ".ts") {
		return 0, false
	}
	num := strings.TrimSuffix(strings.TrimPrefix(fileBase, "segment_"), ".ts")
	n, err := strconv.Atoi(num)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// transcodeSegmentAppearDeadline is how long to wait for segment_<index>.ts to exist while ffmpeg
// is still catching up from t=0 (e.g. subtitle burn-in starts a fresh transcode but the web client
// resumes at the previous wall-clock position and requests a deep segment immediately).
func transcodeSegmentAppearDeadline(segmentIndex int) time.Duration {
	const maxWait = 8 * time.Minute
	const baseline = 15 * time.Second
	if segmentIndex < 0 {
		segmentIndex = 0
	}
	mediaSecondsAhead := float64(segmentIndex+1) * float64(hlsSegmentDurationSeconds)
	const minRealtimeSpeed = 0.25 // pessimistic vs realtime; avoids 404 while transcode ramps
	pessimisticSeconds := mediaSecondsAhead / minRealtimeSpeed
	pessimisticWall := time.Duration(math.Ceil(pessimisticSeconds)) * time.Second
	out := baseline + pessimisticWall
	if out > maxWait {
		return maxWait
	}
	return out
}

func countNonEmptyHlsSegments(root string) int {
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0
	}
	n := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "segment_") || !strings.HasSuffix(name, ".ts") {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.Size() == 0 {
			continue
		}
		n++
	}
	return n
}

func waitForPlaybackFile(ctx context.Context, target string) error {
	_, err := os.Stat(target)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) || !isPlaybackArtifact(target) {
		return err
	}

	waitCap := 1500 * time.Millisecond
	ext := filepath.Ext(target)
	switch ext {
	case ".ts":
		if idx, ok := parseHlsSegmentIndex(filepath.Base(target)); ok {
			waitCap = transcodeSegmentAppearDeadline(idx)
		}
	case ".m3u8":
		// Master / variant playlists can lag segment creation while ffmpeg initializes (especially
		// burn-in / hardware paths). A short wait returns 404 and the client loops on retry.
		const hlsPlaylistAppearWait = 2 * time.Minute
		waitCap = hlsPlaylistAppearWait
	}

	deadline := time.NewTimer(waitCap)
	defer deadline.Stop()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if _, statErr := os.Stat(target); statErr == nil {
				return nil
			}
			return ctx.Err()
		case <-deadline.C:
			_, statErr := os.Stat(target)
			return statErr
		case <-ticker.C:
			if _, statErr := os.Stat(target); statErr == nil {
				return nil
			} else if !os.IsNotExist(statErr) {
				return statErr
			}
		}
	}
}

func isPlaybackArtifact(target string) bool {
	switch filepath.Ext(target) {
	case ".m3u8", ".ts":
		return true
	default:
		return false
	}
}

func compactFFmpegError(stderr string, err error) string {
	stderr = strings.TrimSpace(stderr)
	if len(stderr) > 512 {
		stderr = stderr[len(stderr)-512:]
	}
	if stderr == "" {
		return err.Error()
	}
	return stderr
}

func resolvePlaybackDurationSeconds(mediaDuration int, probedDuration int) int {
	if probedDuration > 0 {
		return probedDuration
	}
	if mediaDuration > 0 {
		return mediaDuration
	}
	return 0
}

func newPlaybackSessionID() (string, error) {
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
