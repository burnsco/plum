package transcoder

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"plum/internal/db"
	"plum/internal/ws"
)

var ffmpegCommandContext = exec.CommandContext
var previousRevisionCancelDelay = 20 * time.Second
var playbackDisconnectGracePeriod = 10 * time.Second

type PlaybackSessionState struct {
	SessionID           string                  `json:"sessionId,omitempty"`
	Delivery            string                  `json:"delivery"`
	MediaID             int                     `json:"mediaId"`
	Revision            int                     `json:"revision,omitempty"`
	AudioIndex          int                     `json:"audioIndex,omitempty"`
	Status              string                  `json:"status"`
	StreamURL           string                  `json:"streamUrl"`
	DurationSeconds     int                     `json:"durationSeconds"`
	Subtitles           []db.Subtitle           `json:"subtitles,omitempty"`
	EmbeddedSubtitles   []db.EmbeddedSubtitle   `json:"embeddedSubtitles,omitempty"`
	EmbeddedAudioTracks []db.EmbeddedAudioTrack `json:"embeddedAudioTracks,omitempty"`
	Error               string                  `json:"error,omitempty"`
	IntroSkipMode       string                  `json:"intro_skip_mode,omitempty"`
	IntroStartSeconds   *float64                `json:"intro_start_seconds,omitempty"`
	IntroEndSeconds     *float64                `json:"intro_end_seconds,omitempty"`
}

type playbackRevision struct {
	number     int
	delivery   string
	audioIndex int
	dir        string
	streamURL  string
	status     string
	err        string
	cancel     context.CancelFunc
	readySent  bool
}

type playbackSession struct {
	mu              sync.Mutex
	id              string
	userID          int
	media           db.MediaItem
	introSkipMode   string
	durationSeconds int
	capabilities    ClientPlaybackCapabilities
	audioIndex      int
	activeRevision  int
	desiredRevision int
	revisions       map[int]*playbackRevision
	ownerClientID   string
	disconnectTimer *time.Timer
}

func attachIntroFields(state *PlaybackSessionState, media db.MediaItem, introSkipMode string) {
	state.IntroSkipMode = db.NormalizeIntroSkipMode(introSkipMode)
	if media.IntroStartSeconds != nil {
		v := *media.IntroStartSeconds
		state.IntroStartSeconds = &v
	}
	if media.IntroEndSeconds != nil {
		v := *media.IntroEndSeconds
		state.IntroEndSeconds = &v
	}
}

type PlaybackSessionManager struct {
	root string
	hub  *ws.Hub

	mu       sync.RWMutex
	sessions map[string]*playbackSession
	clients  map[string]string
}

func NewPlaybackSessionManager(root string, hub *ws.Hub) *PlaybackSessionManager {
	return &PlaybackSessionManager{
		root:     root,
		hub:      hub,
		sessions: make(map[string]*playbackSession),
		clients:  make(map[string]string),
	}
}

func (m *PlaybackSessionManager) Create(
	media db.MediaItem,
	introSkipMode string,
	settings db.TranscodingSettings,
	audioIndex int,
	userID int,
	capabilities ClientPlaybackCapabilities,
) (PlaybackSessionState, error) {
	probe, err := probePlaybackSource(context.Background(), media.Path)
	if err != nil {
		log.Printf("playback probe failed media=%d path=%s error=%v", media.ID, media.Path, err)
	}
	durationSeconds := resolvePlaybackDurationSeconds(media.Duration, probe.DurationSeconds)
	decision := decidePlayback(media.ID, probe, capabilities, audioIndex)
	if decision.Delivery == "direct" {
		state := PlaybackSessionState{
			Delivery:            "direct",
			MediaID:             media.ID,
			AudioIndex:          audioIndex,
			Status:              "ready",
			StreamURL:           decision.StreamURL,
			DurationSeconds:     durationSeconds,
			Subtitles:           media.Subtitles,
			EmbeddedSubtitles:   media.EmbeddedSubtitles,
			EmbeddedAudioTracks: media.EmbeddedAudioTracks,
		}
		attachIntroFields(&state, media, introSkipMode)
		return state, nil
	}

	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return PlaybackSessionState{}, err
	}

	sessionID, err := newPlaybackSessionID()
	if err != nil {
		return PlaybackSessionState{}, err
	}

	session := &playbackSession{
		id:              sessionID,
		userID:          userID,
		media:           media,
		introSkipMode:   db.NormalizeIntroSkipMode(introSkipMode),
		durationSeconds: durationSeconds,
		capabilities:    capabilities,
		audioIndex:      audioIndex,
		activeRevision:  0,
		desiredRevision: 0,
		revisions:       make(map[int]*playbackRevision),
	}

	m.mu.Lock()
	m.sessions[sessionID] = session
	m.mu.Unlock()

	log.Printf(
		"playback session create session=%s media=%d audio_index=%d delivery=%s",
		sessionID,
		media.ID,
		audioIndex,
		decision.Delivery,
	)

	return m.startRevision(session, settings, audioIndex, decision)
}

func (m *PlaybackSessionManager) UpdateAudio(sessionID string, settings db.TranscodingSettings, audioIndex int) (PlaybackSessionState, error) {
	m.mu.RLock()
	session := m.sessions[sessionID]
	m.mu.RUnlock()
	if session == nil {
		return PlaybackSessionState{}, db.ErrNotFound
	}
	probe, err := probePlaybackSource(context.Background(), session.media.Path)
	if err != nil {
		log.Printf("playback probe failed media=%d path=%s error=%v", session.media.ID, session.media.Path, err)
	}
	durationSeconds := resolvePlaybackDurationSeconds(session.media.Duration, probe.DurationSeconds)
	session.mu.Lock()
	if durationSeconds > 0 {
		session.durationSeconds = durationSeconds
	} else {
		durationSeconds = session.durationSeconds
	}
	session.mu.Unlock()
	decision := decidePlayback(session.media.ID, probe, session.capabilities, audioIndex)
	if decision.Delivery == "direct" {
		m.Close(sessionID)
		state := PlaybackSessionState{
			Delivery:            "direct",
			MediaID:             session.media.ID,
			AudioIndex:          audioIndex,
			Status:              "ready",
			StreamURL:           decision.StreamURL,
			DurationSeconds:     durationSeconds,
			Subtitles:           session.media.Subtitles,
			EmbeddedSubtitles:   session.media.EmbeddedSubtitles,
			EmbeddedAudioTracks: session.media.EmbeddedAudioTracks,
		}
		attachIntroFields(&state, session.media, session.introSkipMode)
		return state, nil
	}
	return m.startRevision(session, settings, audioIndex, decision)
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

	log.Printf("playback session attach session=%s user=%d client=%s", sessionID, userID, clientID)
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
	ownerClientID := session.ownerClientID
	delivery := "transcode"
	if active := session.revisions[activeRevision]; active != nil && active.delivery != "" {
		delivery = active.delivery
	}
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
	_ = os.RemoveAll(filepath.Join(m.root, sessionID))
	closed := PlaybackSessionState{
		SessionID:           sessionID,
		Delivery:            delivery,
		MediaID:             mediaID,
		Revision:            activeRevision,
		AudioIndex:          audioIndex,
		Status:              "closed",
		DurationSeconds:     durationSeconds,
		Subtitles:           session.media.Subtitles,
		EmbeddedSubtitles:   session.media.EmbeddedSubtitles,
		EmbeddedAudioTracks: session.media.EmbeddedAudioTracks,
	}
	attachIntroFields(&closed, session.media, session.introSkipMode)
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
			SessionID:           s.id,
			Delivery:            revision.delivery,
			MediaID:             s.media.ID,
			Revision:            revision.number,
			AudioIndex:          revision.audioIndex,
			Status:              revision.status,
			StreamURL:           revision.streamURL,
			DurationSeconds:     s.durationSeconds,
			Subtitles:           s.media.Subtitles,
			EmbeddedSubtitles:   s.media.EmbeddedSubtitles,
			EmbeddedAudioTracks: s.media.EmbeddedAudioTracks,
			Error:               revision.err,
		}
		attachIntroFields(&replay, s.media, s.introSkipMode)
		return &replay
	}
	return nil
}

func (m *PlaybackSessionManager) ServeFile(w http.ResponseWriter, r *http.Request, sessionID string, revisionNumber int, name string) error {
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
	if cleanName == "/" || strings.Contains(cleanName, "..") {
		return db.ErrNotFound
	}
	target := filepath.Join(revision.dir, cleanName)
	if !strings.HasPrefix(target, revision.dir+string(filepath.Separator)) {
		return db.ErrNotFound
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
) (PlaybackSessionState, error) {
	session.mu.Lock()
	session.desiredRevision += 1
	revisionNumber := session.desiredRevision
	session.audioIndex = audioIndex
	durationSeconds := session.durationSeconds

	for _, revision := range session.revisions {
		if revision.number > session.activeRevision && revision.number != revisionNumber && revision.cancel != nil {
			revision.cancel()
		}
	}

	dir := filepath.Join(m.root, session.id, fmt.Sprintf("revision_%d", revisionNumber))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return PlaybackSessionState{}, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	revision := &playbackRevision{
		number:     revisionNumber,
		delivery:   decision.Delivery,
		audioIndex: audioIndex,
		dir:        dir,
		streamURL:  fmt.Sprintf("/api/playback/sessions/%s/revisions/%d/index.m3u8", session.id, revisionNumber),
		status:     "starting",
		cancel:     cancel,
	}
	session.revisions[revisionNumber] = revision
	session.mu.Unlock()

	log.Printf(
		"playback revision start session=%s media=%d revision=%d audio_index=%d delivery=%s",
		session.id,
		session.media.ID,
		revision.number,
		audioIndex,
		decision.Delivery,
	)

	go m.runRevision(ctx, session, revision, settings, decision)

	starting := PlaybackSessionState{
		SessionID:           session.id,
		Delivery:            revision.delivery,
		MediaID:             session.media.ID,
		Revision:            revision.number,
		AudioIndex:          audioIndex,
		Status:              revision.status,
		StreamURL:           revision.streamURL,
		DurationSeconds:     durationSeconds,
		Subtitles:           session.media.Subtitles,
		EmbeddedSubtitles:   session.media.EmbeddedSubtitles,
		EmbeddedAudioTracks: session.media.EmbeddedAudioTracks,
	}
	attachIntroFields(&starting, session.media, session.introSkipMode)
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
		m.Close(sessionID)
	})
	log.Printf(
		"playback session disconnect pending session=%s user=%d client=%s grace=%s",
		session.id,
		userID,
		clientID,
		playbackDisconnectGracePeriod,
	)
}

func (m *PlaybackSessionManager) runRevision(
	ctx context.Context,
	session *playbackSession,
	revision *playbackRevision,
	settings db.TranscodingSettings,
	decision playbackDecision,
) {
	probe, err := probePlaybackSource(ctx, session.media.Path)
	if err != nil {
		log.Printf("playback probe failed media=%d path=%s error=%v", session.media.ID, session.media.Path, err)
	}
	durationSeconds := resolvePlaybackDurationSeconds(session.media.Duration, probe.DurationSeconds)
	session.mu.Lock()
	if durationSeconds > 0 {
		session.durationSeconds = durationSeconds
	} else {
		durationSeconds = session.durationSeconds
	}
	session.mu.Unlock()
	plans := buildPlaybackHLSPlans(session.media.Path, revision.dir, settings, probe, decision)
	finalState := PlaybackSessionState{
		SessionID:           session.id,
		Delivery:            revision.delivery,
		MediaID:             session.media.ID,
		Revision:            revision.number,
		AudioIndex:          revision.audioIndex,
		Status:              "error",
		StreamURL:           revision.streamURL,
		DurationSeconds:     durationSeconds,
		Subtitles:           session.media.Subtitles,
		EmbeddedSubtitles:   session.media.EmbeddedSubtitles,
		EmbeddedAudioTracks: session.media.EmbeddedAudioTracks,
	}
	attachIntroFields(&finalState, session.media, session.introSkipMode)

	for index, plan := range plans {
		if ctx.Err() != nil {
			return
		}

		log.Printf(
			"playback revision ffmpeg start session=%s media=%d revision=%d mode=%s attempt=%d/%d",
			session.id,
			session.media.ID,
			revision.number,
			plan.Mode,
			index+1,
			len(plans),
		)

		if err := os.RemoveAll(revision.dir); err == nil {
			_ = os.MkdirAll(revision.dir, 0o755)
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
			log.Printf(
				"playback revision ffmpeg start failed session=%s media=%d revision=%d mode=%s error=%q",
				session.id,
				session.media.ID,
				revision.number,
				plan.Mode,
				finalState.Error,
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
				return
			case err := <-waitCh:
				ticker.Stop()
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					finalState.Error = compactFFmpegError(stderrBuf.String(), err)
					log.Printf(
						"playback revision ffmpeg failed session=%s media=%d revision=%d mode=%s ready=%t error=%q",
						session.id,
						session.media.ID,
						revision.number,
						plan.Mode,
						ready,
						finalState.Error,
					)
					break loop
				}
				if !ready && revisionReady(revision.dir) {
					ready = true
					m.markRevisionReady(session, revision)
				}
				log.Printf(
					"playback revision ffmpeg exited session=%s media=%d revision=%d mode=%s ready=%t",
					session.id,
					session.media.ID,
					revision.number,
					plan.Mode,
					ready,
				)
				return
			case <-ticker.C:
				if !ready && revisionReady(revision.dir) {
					ready = true
					m.markRevisionReady(session, revision)
				}
			}
		}

		if plan.Mode == "hardware" && settings.AllowSoftwareFallback && index+1 < len(plans) && !ready {
			log.Printf(
				"playback revision fallback session=%s media=%d revision=%d from=%s to=%s",
				session.id,
				session.media.ID,
				revision.number,
				plan.Mode,
				plans[index+1].Mode,
			)
			continue
		}
		break
	}

	if finalState.Error == "" {
		finalState.Error = "playback stream failed"
	}
	revision.status = "error"
	revision.err = finalState.Error
	log.Printf(
		"playback revision error session=%s media=%d revision=%d error=%q",
		session.id,
		session.media.ID,
		revision.number,
		finalState.Error,
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
	}
	activeRevision := session.activeRevision
	audioIndex := session.audioIndex
	mediaID := session.media.ID
	sessionID := session.id
	durationSeconds := session.durationSeconds
	session.mu.Unlock()

	ready := PlaybackSessionState{
		SessionID:           sessionID,
		Delivery:            revision.delivery,
		MediaID:             mediaID,
		Revision:            revision.number,
		AudioIndex:          audioIndex,
		Status:              "ready",
		StreamURL:           revision.streamURL,
		DurationSeconds:     durationSeconds,
		Subtitles:           session.media.Subtitles,
		EmbeddedSubtitles:   session.media.EmbeddedSubtitles,
		EmbeddedAudioTracks: session.media.EmbeddedAudioTracks,
	}
	attachIntroFields(&ready, session.media, session.introSkipMode)
	m.broadcast(ready)

	if previousActive > 0 && previousActive != activeRevision {
		session.mu.Lock()
		previous := session.revisions[previousActive]
		session.mu.Unlock()
		if previous != nil && previous.cancel != nil {
			delay := previousRevisionCancelDelay
			log.Printf(
				"playback revision ready session=%s media=%d revision=%d previous_revision=%d cancel_delay=%s",
				sessionID,
				mediaID,
				revision.number,
				previousActive,
				delay,
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
	msg := map[string]any{
		"type":            "playback_session_update",
		"sessionId":       state.SessionID,
		"delivery":        state.Delivery,
		"mediaId":         state.MediaID,
		"revision":        state.Revision,
		"audioIndex":      state.AudioIndex,
		"status":          state.Status,
		"streamUrl":       state.StreamURL,
		"durationSeconds": state.DurationSeconds,
		"error":           state.Error,
	}
	if state.IntroSkipMode != "" {
		msg["intro_skip_mode"] = state.IntroSkipMode
	}
	if state.IntroStartSeconds != nil {
		msg["intro_start_seconds"] = *state.IntroStartSeconds
	}
	if state.IntroEndSeconds != nil {
		msg["intro_end_seconds"] = *state.IntroEndSeconds
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		log.Printf("marshal playback session update: %v", err)
		return
	}
	m.hub.Broadcast(payload)
}

func revisionReady(dir string) bool {
	playlistPath := filepath.Join(dir, "index.m3u8")
	playlistInfo, err := os.Stat(playlistPath)
	if err != nil || playlistInfo.Size() == 0 {
		return false
	}

	ready := false
	walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info == nil || info.IsDir() {
			return nil
		}
		if strings.HasPrefix(filepath.Base(path), "segment_") && filepath.Ext(path) == ".ts" && info.Size() > 0 {
			ready = true
			return io.EOF
		}
		return nil
	})
	return ready || walkErr == io.EOF
}

func waitForPlaybackFile(ctx context.Context, target string) error {
	_, err := os.Stat(target)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) || !isPlaybackArtifact(target) {
		return err
	}

	deadline := time.NewTimer(1500 * time.Millisecond)
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
