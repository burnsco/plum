package transcoder

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"plum/internal/db"
)

func TestRunRevisionFallsBackToSoftwareBeforeReady(t *testing.T) {
	root := t.TempDir()
	manager := NewPlaybackSessionManager(context.Background(), root, nil)
	session := &playbackSession{
		id: "session-fallback",
		media: db.MediaItem{
			ID:   42,
			Path: filepath.Join(root, "media.mkv"),
		},
		revisions: make(map[int]*playbackRevision),
	}

	settings := db.DefaultTranscodingSettings()
	settings.VAAPIEnabled = true
	settings.HardwareEncodingEnabled = true
	settings.AllowSoftwareFallback = true

	previousCommandContext := ffmpegCommandContext
	ffmpegCommandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		exitCode := "0"
		if hlsArgsUseHardware(args) {
			exitCode = "1"
		}
		return fakeHLSCommand(ctx, args, exitCode)
	}
	t.Cleanup(func() {
		ffmpegCommandContext = previousCommandContext
	})

	if _, err := manager.startRevision(session, settings, -1, playbackDecision{Delivery: "transcode"}, nil); err != nil {
		t.Fatalf("startRevision: %v", err)
	}

	revision := waitForRevisionStatus(t, session, 1, "ready")
	if revision.err != "" {
		t.Fatalf("expected empty revision error, got %q", revision.err)
	}

	session.mu.Lock()
	activeRevision := session.activeRevision
	session.mu.Unlock()
	if activeRevision != 1 {
		t.Fatalf("activeRevision = %d, want 1", activeRevision)
	}
}

func TestRevisionReadyRequiresBufferedSegments(t *testing.T) {
	dir := t.TempDir()
	if revisionReady(dir, 3600) {
		t.Fatal("expected false for missing playlist")
	}
	if err := os.WriteFile(filepath.Join(dir, "index.m3u8"), []byte("#EXTM3U\n"), 0o644); err != nil {
		t.Fatalf("write playlist: %v", err)
	}
	if revisionReady(dir, 3600) {
		t.Fatal("expected false with playlist but no segments")
	}
	if err := os.WriteFile(filepath.Join(dir, "segment_00000.ts"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write segment: %v", err)
	}
	if revisionReady(dir, 3600) {
		t.Fatal("expected false with only one segment for long content")
	}
	if err := os.WriteFile(filepath.Join(dir, "segment_00001.ts"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write segment: %v", err)
	}
	if !revisionReady(dir, 3600) {
		t.Fatal("expected true once two segments exist")
	}
}

func TestRevisionReadyShortMediaUsesFewerSegments(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.m3u8"), []byte("#EXTM3U\n"), 0o644); err != nil {
		t.Fatalf("write playlist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "segment_00000.ts"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write segment: %v", err)
	}
	// ~3s total → ceil(3/2)=2 segments required; one is not enough.
	if revisionReady(dir, 3) {
		t.Fatal("expected false with one segment when two are required")
	}
	if err := os.WriteFile(filepath.Join(dir, "segment_00001.ts"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write segment: %v", err)
	}
	if !revisionReady(dir, 3) {
		t.Fatal("expected true for short media once required segments exist")
	}
}

func TestCreateRespectsMaxSessionsPerUser(t *testing.T) {
	t.Setenv("PLUM_MAX_PLAYBACK_SESSIONS_PER_USER", "1")

	root := t.TempDir()
	mediaPath := filepath.Join(root, "media.mkv")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media file: %v", err)
	}

	previousProbeCommandContext := ffprobeCommandContext
	ffprobeCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return fakeFFProbeCommand(
			ctx,
			`{"format":{"format_name":"matroska","bit_rate":"0","duration":"120"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","bit_rate":"0"},{"index":1,"codec_type":"audio","codec_name":"aac","bit_rate":"0"}]}`,
		)
	}
	t.Cleanup(func() {
		ffprobeCommandContext = previousProbeCommandContext
	})

	previousCommandContext := ffmpegCommandContext
	ffmpegCommandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		return fakeHLSCommand(ctx, args, "0")
	}
	t.Cleanup(func() {
		ffmpegCommandContext = previousCommandContext
	})

	manager := NewPlaybackSessionManager(context.Background(), root, nil)
	// Empty video codec list forces transcode so a session is registered in the manager.
	caps := ClientPlaybackCapabilities{
		SupportsNativeHLS: true,
		SupportsMSEHLS:    true,
		VideoCodecs:       nil,
		AudioCodecs:       []string{"aac"},
		Containers:        []string{"mkv"},
	}
	first, err := manager.Create(
		context.Background(),
		db.MediaItem{ID: 501, Path: mediaPath},
		db.DefaultTranscodingSettings(),
		-1,
		77,
		caps,
		nil,
	)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	defer func() {
		if first.SessionID == "" {
			return
		}
		manager.mu.RLock()
		session := manager.sessions[first.SessionID]
		manager.mu.RUnlock()
		if session != nil {
			waitForRevisionStatus(t, session, 1, "ready")
		}
		manager.Close(first.SessionID)
	}()
	_, err = manager.Create(
		context.Background(),
		db.MediaItem{ID: 502, Path: mediaPath},
		db.DefaultTranscodingSettings(),
		-1,
		77,
		caps,
		nil,
	)
	if !errors.Is(err, ErrTooManyPlaybackSessions) {
		t.Fatalf("second Create: want ErrTooManyPlaybackSessions, got %v", err)
	}
}

func TestCreateIgnoresErroredSessionsForMaxSessions(t *testing.T) {
	t.Setenv("PLUM_MAX_PLAYBACK_SESSIONS_PER_USER", "1")

	root := t.TempDir()
	mediaPath := filepath.Join(root, "media.mkv")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media file: %v", err)
	}

	previousProbeCommandContext := ffprobeCommandContext
	ffprobeCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return fakeFFProbeCommand(
			ctx,
			`{"format":{"format_name":"matroska","bit_rate":"0","duration":"120"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","bit_rate":"0"},{"index":1,"codec_type":"audio","codec_name":"aac","bit_rate":"0"}]}`,
		)
	}
	t.Cleanup(func() {
		ffprobeCommandContext = previousProbeCommandContext
	})

	previousCommandContext := ffmpegCommandContext
	ffmpegCommandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		return fakeHLSCommand(ctx, args, "1")
	}
	t.Cleanup(func() {
		ffmpegCommandContext = previousCommandContext
	})

	manager := NewPlaybackSessionManager(context.Background(), root, nil)
	caps := ClientPlaybackCapabilities{
		SupportsNativeHLS: true,
		SupportsMSEHLS:    true,
		VideoCodecs:       nil,
		AudioCodecs:       []string{"aac"},
		Containers:        []string{"mkv"},
	}

	first, err := manager.Create(
		context.Background(),
		db.MediaItem{ID: 601, Path: mediaPath},
		db.DefaultTranscodingSettings(),
		-1,
		77,
		caps,
		nil,
	)
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	manager.mu.RLock()
	session := manager.sessions[first.SessionID]
	manager.mu.RUnlock()
	if session == nil {
		t.Fatalf("missing first session %q", first.SessionID)
	}

	waitForRevisionStatus(t, session, 1, "error")

	second, err := manager.Create(
		context.Background(),
		db.MediaItem{ID: 602, Path: mediaPath},
		db.DefaultTranscodingSettings(),
		-1,
		77,
		caps,
		nil,
	)
	if err != nil {
		t.Fatalf("second Create after errored session: %v", err)
	}
	if second.SessionID == "" {
		t.Fatal("expected second session id")
	}

	manager.Close(first.SessionID)
	manager.Close(second.SessionID)
}

func TestCreateReturnsDurationSecondsFromProbe(t *testing.T) {
	root := t.TempDir()
	mediaPath := filepath.Join(root, "media.mp4")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media file: %v", err)
	}

	previousProbeCommandContext := ffprobeCommandContext
	ffprobeCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return fakeFFProbeCommand(
			ctx,
			`{"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","bit_rate":"0","duration":"7200.9"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","bit_rate":"0"},{"index":1,"codec_type":"audio","codec_name":"aac","bit_rate":"0"}]}`,
		)
	}
	t.Cleanup(func() {
		ffprobeCommandContext = previousProbeCommandContext
	})

	manager := NewPlaybackSessionManager(context.Background(), root, nil)
	state, err := manager.Create(
		context.Background(),
		db.MediaItem{ID: 21, Path: mediaPath, Duration: 120},
		db.DefaultTranscodingSettings(),
		-1,
		99,
		ClientPlaybackCapabilities{
			Containers:  []string{"mp4"},
			VideoCodecs: []string{"h264"},
			AudioCodecs: []string{"aac"},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if state.Delivery != "direct" {
		t.Fatalf("delivery = %q, want direct", state.Delivery)
	}
	if state.DurationSeconds != 7200 {
		t.Fatalf("durationSeconds = %d, want 7200", state.DurationSeconds)
	}
}

func TestRunRevisionUpdatesDurationSecondsFromProbe(t *testing.T) {
	root := t.TempDir()
	mediaPath := filepath.Join(root, "media.mkv")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media file: %v", err)
	}

	previousProbeCommandContext := ffprobeCommandContext
	ffprobeCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return fakeFFProbeCommand(
			ctx,
			`{"format":{"format_name":"matroska","bit_rate":"0","duration":"3600.7"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","bit_rate":"0"},{"index":1,"codec_type":"audio","codec_name":"aac","bit_rate":"0"}]}`,
		)
	}
	t.Cleanup(func() {
		ffprobeCommandContext = previousProbeCommandContext
	})

	previousCommandContext := ffmpegCommandContext
	ffmpegCommandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		return fakeHLSCommand(ctx, args, "0")
	}
	t.Cleanup(func() {
		ffmpegCommandContext = previousCommandContext
	})

	manager := NewPlaybackSessionManager(context.Background(), root, nil)
	session := &playbackSession{
		id:              "session-duration",
		media:           db.MediaItem{ID: 44, Path: mediaPath},
		durationSeconds: 0,
		revisions:       make(map[int]*playbackRevision),
	}

	if _, err := manager.startRevision(session, db.DefaultTranscodingSettings(), -1, playbackDecision{Delivery: "transcode"}, nil); err != nil {
		t.Fatalf("startRevision: %v", err)
	}

	waitForRevisionStatus(t, session, 1, "ready")

	session.mu.Lock()
	durationSeconds := session.durationSeconds
	session.mu.Unlock()
	if durationSeconds != 3600 {
		t.Fatalf("durationSeconds = %d, want 3600", durationSeconds)
	}
}

func TestRunRevisionMarksErrorAfterAllPlansFail(t *testing.T) {
	root := t.TempDir()
	manager := NewPlaybackSessionManager(context.Background(), root, nil)
	session := &playbackSession{
		id: "session-error",
		media: db.MediaItem{
			ID:   7,
			Path: filepath.Join(root, "media.mkv"),
		},
		revisions: make(map[int]*playbackRevision),
	}

	settings := db.DefaultTranscodingSettings()
	settings.VAAPIEnabled = true
	settings.HardwareEncodingEnabled = true
	settings.AllowSoftwareFallback = true

	previousCommandContext := ffmpegCommandContext
	ffmpegCommandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		return fakeHLSCommand(ctx, args, "1")
	}
	t.Cleanup(func() {
		ffmpegCommandContext = previousCommandContext
	})

	if _, err := manager.startRevision(session, settings, -1, playbackDecision{Delivery: "transcode"}, nil); err != nil {
		t.Fatalf("startRevision: %v", err)
	}

	revision := waitForRevisionStatus(t, session, 1, "error")
	if revision.err == "" {
		t.Fatal("expected revision error to be populated")
	}

	session.mu.Lock()
	activeRevision := session.activeRevision
	session.mu.Unlock()
	if activeRevision != 0 {
		t.Fatalf("activeRevision = %d, want 0", activeRevision)
	}
}

func TestSeekStartsOffsetRevisionPreservingTracks(t *testing.T) {
	root := t.TempDir()
	mediaPath := filepath.Join(root, "media.mkv")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media file: %v", err)
	}

	previousProbeCommandContext := ffprobeCommandContext
	ffprobeCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return fakeFFProbeCommand(
			ctx,
			`{"format":{"format_name":"matroska","bit_rate":"0","duration":"3600"},"streams":[{"index":0,"codec_type":"video","codec_name":"hevc","pix_fmt":"yuv420p","height":1080,"bit_rate":"0"},{"index":2,"codec_type":"audio","codec_name":"aac","bit_rate":"0"},{"index":7,"codec_type":"subtitle","codec_name":"hdmv_pgs_subtitle"}]}`,
		)
	}
	t.Cleanup(func() {
		ffprobeCommandContext = previousProbeCommandContext
	})

	previousCommandContext := ffmpegCommandContext
	ffmpegCommandContext = func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		return fakeHLSCommand(ctx, args, "0")
	}
	t.Cleanup(func() {
		ffmpegCommandContext = previousCommandContext
	})

	burnStream := 7
	manager := NewPlaybackSessionManager(context.Background(), root, nil)
	session := &playbackSession{
		id:                         "session-seek",
		userID:                     42,
		media:                      db.MediaItem{ID: 55, Path: mediaPath, Duration: 3600},
		durationSeconds:            3600,
		audioIndex:                 2,
		revisions:                  make(map[int]*playbackRevision),
		burnEmbeddedSubtitleStream: &burnStream,
	}
	manager.sessions[session.id] = session

	state, err := manager.Seek(session.id, db.DefaultTranscodingSettings(), 120.25)
	if err != nil {
		t.Fatalf("Seek: %v", err)
	}
	if state.Status != "starting" || state.Revision != 1 || state.AudioIndex != 2 || state.StreamOffsetSeconds != 120.25 {
		t.Fatalf("unexpected seek state: %+v", state)
	}
	if state.BurnEmbeddedSubtitleStreamIndex == nil || *state.BurnEmbeddedSubtitleStreamIndex != burnStream {
		t.Fatalf("burn stream = %v, want %d", state.BurnEmbeddedSubtitleStreamIndex, burnStream)
	}

	session.mu.Lock()
	revision := session.revisions[1]
	sessionOffset := session.streamOffsetSeconds
	session.mu.Unlock()
	if revision == nil {
		t.Fatal("expected revision 1")
	}
	if revision.startOffsetSeconds != 120.25 || sessionOffset != 120.25 {
		t.Fatalf("offsets = revision %v session %v, want 120.25", revision.startOffsetSeconds, sessionOffset)
	}
	waitForRevisionStatus(t, session, 1, "ready")
}

func TestServeFileWaitsForDelayedSegment(t *testing.T) {
	root := t.TempDir()
	manager := NewPlaybackSessionManager(context.Background(), root, nil)
	revisionDir := filepath.Join(root, "session-serve", "revision_1")
	if err := os.MkdirAll(revisionDir, 0o755); err != nil {
		t.Fatalf("mkdir revision dir: %v", err)
	}

	manager.sessions["session-serve"] = &playbackSession{
		id: "session-serve",
		media: db.MediaItem{
			ID: 9,
		},
		revisions: map[int]*playbackRevision{
			1: {
				number: 1,
				dir:    revisionDir,
				status: "ready",
			},
		},
	}

	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(revisionDir, "segment_00001.ts"), []byte("segment"), 0o644)
	}()

	req := httptest.NewRequest(http.MethodGet, "/api/playback/sessions/session-serve/revisions/1/segment_00001.ts", nil)
	rec := httptest.NewRecorder()

	if err := manager.ServeFile(rec, req, "session-serve", 1, "segment_00001.ts"); err != nil {
		t.Fatalf("ServeFile: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "segment" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestServeFileUsesFrozenRevisionVirtualSubtitleTracks(t *testing.T) {
	root := t.TempDir()
	manager := NewPlaybackSessionManager(context.Background(), root, nil)
	revisionDir := filepath.Join(root, "session-frozen-virtual", "revision_1")
	if err := os.MkdirAll(revisionDir, 0o755); err != nil {
		t.Fatalf("mkdir revision dir: %v", err)
	}

	frozenMap, frozenMaster := freezeRevisionSubtitleTracks(db.MediaItem{
		ID: 11,
		Subtitles: []db.Subtitle{
			{ID: 101, Language: "en", Format: "srt"},
		},
	})

	session := &playbackSession{
		id: "session-frozen-virtual",
		media: db.MediaItem{
			ID: 11,
			Subtitles: []db.Subtitle{
				{ID: 101, Language: "en", Format: "srt"},
			},
		},
		durationSeconds: 120,
		revisions: map[int]*playbackRevision{
			1: {
				number:                       1,
				dir:                          revisionDir,
				status:                       "ready",
				subtitleTracksByPlaylistFile: frozenMap,
				subtitleTracksForMaster:      frozenMaster,
			},
		},
	}
	manager.sessions["session-frozen-virtual"] = session

	// Simulate metadata refresh mutating session-level subtitle list after revision creation.
	session.mu.Lock()
	session.media.Subtitles = []db.Subtitle{{ID: 202, Language: "fr", Format: "srt"}}
	session.mu.Unlock()

	playlistFile := hlsSubtitlePlaylistFileForLogicalID("ext:101")
	req := httptest.NewRequest(http.MethodGet, "/api/playback/sessions/session-frozen-virtual/revisions/1/"+playlistFile, nil)
	rec := httptest.NewRecorder()

	if err := manager.ServeFile(rec, req, "session-frozen-virtual", 1, playlistFile); err != nil {
		t.Fatalf("ServeFile: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/api/subtitles/101") {
		t.Fatalf("expected frozen subtitle path, got body %q", body)
	}
	if strings.Contains(body, "/api/subtitles/202") {
		t.Fatalf("unexpected mutated subtitle path in body %q", body)
	}
}

func TestServeFileUsesFrozenRevisionMasterSubtitleTracks(t *testing.T) {
	root := t.TempDir()
	manager := NewPlaybackSessionManager(context.Background(), root, nil)
	revisionDir := filepath.Join(root, "session-frozen-master", "revision_1")
	if err := os.MkdirAll(revisionDir, 0o755); err != nil {
		t.Fatalf("mkdir revision dir: %v", err)
	}
	playlist := "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=800000\nvariant_0/index.m3u8\n"
	if err := os.WriteFile(filepath.Join(revisionDir, "index.m3u8"), []byte(playlist), 0o644); err != nil {
		t.Fatalf("write playlist: %v", err)
	}

	frozenMap, frozenMaster := freezeRevisionSubtitleTracks(db.MediaItem{
		ID: 12,
		Subtitles: []db.Subtitle{
			{ID: 303, Language: "en", Format: "srt", Title: "English"},
		},
	})

	session := &playbackSession{
		id: "session-frozen-master",
		media: db.MediaItem{
			ID: 12,
			Subtitles: []db.Subtitle{
				{ID: 303, Language: "en", Format: "srt", Title: "English"},
			},
		},
		revisions: map[int]*playbackRevision{
			1: {
				number:                       1,
				dir:                          revisionDir,
				status:                       "ready",
				subtitleTracksByPlaylistFile: frozenMap,
				subtitleTracksForMaster:      frozenMaster,
			},
		},
	}
	manager.sessions["session-frozen-master"] = session

	// Mutate session-level media to a different subtitle after revision freeze.
	session.mu.Lock()
	session.media.Subtitles = []db.Subtitle{{ID: 404, Language: "fr", Format: "srt", Title: "French"}}
	session.mu.Unlock()

	req := httptest.NewRequest(http.MethodGet, "/api/playback/sessions/session-frozen-master/revisions/1/index.m3u8", nil)
	rec := httptest.NewRecorder()
	if err := manager.ServeFile(rec, req, "session-frozen-master", 1, "index.m3u8"); err != nil {
		t.Fatalf("ServeFile: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, hlsSubtitlePlaylistFileForLogicalID("ext:303")) {
		t.Fatalf("expected frozen subtitle rendition in master, got %q", body)
	}
	if strings.Contains(body, hlsSubtitlePlaylistFileForLogicalID("ext:404")) {
		t.Fatalf("unexpected mutated subtitle rendition in master %q", body)
	}
}

func TestParseHlsSegmentIndex(t *testing.T) {
	t.Parallel()
	n, ok := parseHlsSegmentIndex("segment_00022.ts")
	if !ok || n != 22 {
		t.Fatalf("got %d ok=%v", n, ok)
	}
	if _, ok := parseHlsSegmentIndex("other.ts"); ok {
		t.Fatal("unexpected ok")
	}
}

func TestTranscodeSegmentAppearDeadline_ScalesWithSegmentIndex(t *testing.T) {
	t.Parallel()
	low := transcodeSegmentAppearDeadline(0)
	high := transcodeSegmentAppearDeadline(80)
	if high <= low {
		t.Fatalf("expected deeper segments to wait longer: low=%v high=%v", low, high)
	}
	if low < time.Second || low > 30*time.Second {
		t.Fatalf("unexpected low deadline: %v", low)
	}
	if high > 8*time.Minute+time.Second {
		t.Fatalf("expected cap near 8m: %v", high)
	}
}

func TestMarkRevisionReadyDefersPreviousRevisionCancellation(t *testing.T) {
	manager := NewPlaybackSessionManager(context.Background(), t.TempDir(), nil)
	canceled := make(chan struct{}, 1)
	session := &playbackSession{
		id:              "session-ready",
		media:           db.MediaItem{ID: 11},
		activeRevision:  1,
		desiredRevision: 2,
		revisions: map[int]*playbackRevision{
			1: {
				number: 1,
				cancel: func() {
					canceled <- struct{}{}
				},
			},
			2: {
				number:     2,
				audioIndex: 2,
			},
		},
	}

	previousDelay := previousRevisionCancelDelay
	previousRevisionCancelDelay = 25 * time.Millisecond
	t.Cleanup(func() {
		previousRevisionCancelDelay = previousDelay
	})

	manager.markRevisionReady(session, session.revisions[2])

	select {
	case <-canceled:
		t.Fatal("previous revision canceled immediately")
	case <-time.After(10 * time.Millisecond):
	}

	select {
	case <-canceled:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("previous revision was not canceled after delay")
	}
}

func TestHandleDisconnectClosesSessionAfterGracePeriod(t *testing.T) {
	manager := NewPlaybackSessionManager(context.Background(), t.TempDir(), nil)
	session := &playbackSession{
		id:        "session-disconnect",
		userID:    7,
		media:     db.MediaItem{ID: 13},
		revisions: make(map[int]*playbackRevision),
	}

	manager.mu.Lock()
	manager.sessions[session.id] = session
	manager.mu.Unlock()

	previousGrace := playbackDisconnectGracePeriod
	playbackDisconnectGracePeriod = 25 * time.Millisecond
	t.Cleanup(func() {
		playbackDisconnectGracePeriod = previousGrace
	})

	if _, err := manager.Attach(session.id, session.userID, "client-a"); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	manager.HandleDisconnect(session.userID, "client-a")

	waitForSessionClosed(t, manager, session.id)
}

func TestAttachCancelsPendingDisconnectClose(t *testing.T) {
	manager := NewPlaybackSessionManager(context.Background(), t.TempDir(), nil)
	session := &playbackSession{
		id:        "session-reattach",
		userID:    8,
		media:     db.MediaItem{ID: 14},
		revisions: make(map[int]*playbackRevision),
	}

	manager.mu.Lock()
	manager.sessions[session.id] = session
	manager.mu.Unlock()

	previousGrace := playbackDisconnectGracePeriod
	playbackDisconnectGracePeriod = 80 * time.Millisecond
	t.Cleanup(func() {
		playbackDisconnectGracePeriod = previousGrace
	})

	if _, err := manager.Attach(session.id, session.userID, "client-a"); err != nil {
		t.Fatalf("Attach initial: %v", err)
	}

	manager.HandleDisconnect(session.userID, "client-a")

	time.Sleep(25 * time.Millisecond)

	if _, err := manager.Attach(session.id, session.userID, "client-b"); err != nil {
		t.Fatalf("Attach reconnect: %v", err)
	}

	time.Sleep(90 * time.Millisecond)

	manager.mu.RLock()
	remaining := manager.sessions[session.id]
	ownedBy := manager.clients["client-b"]
	manager.mu.RUnlock()

	if remaining == nil {
		t.Fatal("expected session to remain after reattach")
	}
	if ownedBy != session.id {
		t.Fatalf("client-b owner = %q, want %q", ownedBy, session.id)
	}
}

func TestAttachTransfersOwnershipFromPreviousClient(t *testing.T) {
	manager := NewPlaybackSessionManager(context.Background(), t.TempDir(), nil)
	session := &playbackSession{
		id:        "session-transfer",
		userID:    9,
		media:     db.MediaItem{ID: 15},
		revisions: make(map[int]*playbackRevision),
	}

	manager.mu.Lock()
	manager.sessions[session.id] = session
	manager.mu.Unlock()

	previousGrace := playbackDisconnectGracePeriod
	playbackDisconnectGracePeriod = 25 * time.Millisecond
	t.Cleanup(func() {
		playbackDisconnectGracePeriod = previousGrace
	})

	if _, err := manager.Attach(session.id, session.userID, "client-a"); err != nil {
		t.Fatalf("Attach initial: %v", err)
	}
	if _, err := manager.Attach(session.id, session.userID, "client-b"); err != nil {
		t.Fatalf("Attach transfer: %v", err)
	}

	manager.HandleDisconnect(session.userID, "client-a")
	time.Sleep(40 * time.Millisecond)

	manager.mu.RLock()
	remaining := manager.sessions[session.id]
	ownedBy := manager.clients["client-b"]
	manager.mu.RUnlock()

	if remaining == nil {
		t.Fatal("expected stale client disconnect not to close session")
	}
	if ownedBy != session.id {
		t.Fatalf("client-b owner = %q, want %q", ownedBy, session.id)
	}
}

func TestAttachReturnsReplayStateForReadyRevision(t *testing.T) {
	manager := NewPlaybackSessionManager(context.Background(), t.TempDir(), nil)
	session := &playbackSession{
		id:              "session-ready-replay",
		userID:          10,
		media:           db.MediaItem{ID: 16},
		durationSeconds: 1800,
		audioIndex:      2,
		activeRevision:  3,
		desiredRevision: 3,
		revisions: map[int]*playbackRevision{
			3: {
				number:     3,
				delivery:   "transcode",
				audioIndex: 2,
				status:     "ready",
				streamURL:  "/api/playback/sessions/session-ready-replay/revisions/3/index.m3u8",
			},
		},
	}

	manager.mu.Lock()
	manager.sessions[session.id] = session
	manager.mu.Unlock()

	state, err := manager.Attach(session.id, session.userID, "client-ready")
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if state == nil {
		t.Fatal("expected replay state")
	}
	if state.Status != "ready" || state.Revision != 3 || state.AudioIndex != 2 || state.DurationSeconds != 1800 {
		t.Fatalf("unexpected replay state: %+v", state)
	}
}

func TestAttachReturnsReplayStateForErroredRevision(t *testing.T) {
	manager := NewPlaybackSessionManager(context.Background(), t.TempDir(), nil)
	session := &playbackSession{
		id:              "session-error-replay",
		userID:          11,
		media:           db.MediaItem{ID: 17},
		durationSeconds: 2400,
		audioIndex:      -1,
		desiredRevision: 4,
		revisions: map[int]*playbackRevision{
			4: {
				number:     4,
				delivery:   "transcode",
				audioIndex: -1,
				status:     "error",
				streamURL:  "/api/playback/sessions/session-error-replay/revisions/4/index.m3u8",
				err:        "transcode failed",
			},
		},
	}

	manager.mu.Lock()
	manager.sessions[session.id] = session
	manager.mu.Unlock()

	state, err := manager.Attach(session.id, session.userID, "client-error")
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if state == nil {
		t.Fatal("expected replay state")
	}
	if state.Status != "error" || state.Error != "transcode failed" || state.Revision != 4 || state.DurationSeconds != 2400 {
		t.Fatalf("unexpected replay state: %+v", state)
	}
}

func TestUpdateAudioReturnsDurationSecondsFromProbe(t *testing.T) {
	root := t.TempDir()
	mediaPath := filepath.Join(root, "media.mp4")
	if err := os.WriteFile(mediaPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media file: %v", err)
	}

	previousProbeCommandContext := ffprobeCommandContext
	ffprobeCommandContext = func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		return fakeFFProbeCommand(
			ctx,
			`{"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","bit_rate":"0","duration":"5400.4"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","bit_rate":"0"},{"index":1,"codec_type":"audio","codec_name":"aac","bit_rate":"0"}]}`,
		)
	}
	t.Cleanup(func() {
		ffprobeCommandContext = previousProbeCommandContext
	})

	manager := NewPlaybackSessionManager(context.Background(), root, nil)
	session := &playbackSession{
		id:     "session-update-audio",
		userID: 42,
		media: db.MediaItem{
			ID:       22,
			Path:     mediaPath,
			Duration: 90,
		},
		capabilities: ClientPlaybackCapabilities{
			Containers:  []string{"mp4"},
			VideoCodecs: []string{"h264"},
			AudioCodecs: []string{"aac"},
		},
		revisions: map[int]*playbackRevision{
			1: {number: 1, delivery: "transcode"},
		},
	}
	manager.mu.Lock()
	manager.sessions[session.id] = session
	manager.mu.Unlock()

	state, err := manager.UpdateAudio(session.id, db.DefaultTranscodingSettings(), -1)
	if err != nil {
		t.Fatalf("UpdateAudio: %v", err)
	}

	if state.Delivery != "direct" {
		t.Fatalf("delivery = %q, want direct", state.Delivery)
	}
	if state.DurationSeconds != 5400 {
		t.Fatalf("durationSeconds = %d, want 5400", state.DurationSeconds)
	}
}

func waitForRevisionStatus(t *testing.T, session *playbackSession, revisionNumber int, status string) *playbackRevision {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		session.mu.Lock()
		revision := session.revisions[revisionNumber]
		if revision != nil && revision.status == status {
			session.mu.Unlock()
			return revision
		}
		session.mu.Unlock()
		time.Sleep(25 * time.Millisecond)
	}

	session.mu.Lock()
	revision := session.revisions[revisionNumber]
	session.mu.Unlock()
	if revision == nil {
		t.Fatalf("revision %d was never created", revisionNumber)
	}
	t.Fatalf("revision %d status = %q, want %q", revisionNumber, revision.status, status)
	return nil
}

func waitForSessionClosed(t *testing.T, manager *PlaybackSessionManager, sessionID string) {
	t.Helper()

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		manager.mu.RLock()
		session := manager.sessions[sessionID]
		manager.mu.RUnlock()
		if session == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	manager.mu.RLock()
	_, ok := manager.sessions[sessionID]
	manager.mu.RUnlock()
	if ok {
		t.Fatalf("session %q was not closed", sessionID)
	}
}

func hlsArgsUseHardware(args []string) bool {
	for _, arg := range args {
		if arg == "-vaapi_device" || strings.HasSuffix(arg, "_vaapi") {
			return true
		}
	}
	return false
}

func fakeHLSCommand(ctx context.Context, args []string, exitCode string) *exec.Cmd {
	playlistPath := args[len(args)-1]
	segmentTemplate := ""
	masterPlaylistName := ""
	for index := 0; index < len(args)-1; index += 1 {
		if args[index] == "-hls_segment_filename" && index+1 < len(args) {
			segmentTemplate = args[index+1]
		}
		if args[index] == "-master_pl_name" && index+1 < len(args) {
			masterPlaylistName = args[index+1]
		}
	}

	script := `
playlist_path="$1"
segment_template="$2"
master_playlist_name="$3"
exit_code="$4"
resolved_playlist_path="${playlist_path//%v/0}"
resolved_segment_template="${segment_template//%v/0}"
mkdir -p "$(dirname "$resolved_playlist_path")"
printf '#EXTM3U\n' > "$resolved_playlist_path"
if [ -n "$master_playlist_name" ]; then
  out_dir="$(dirname "$(dirname "$resolved_playlist_path")")"
  printf '#EXTM3U\n' > "$out_dir/$master_playlist_name"
fi
for i in 0 1 2 3; do
  num=$(printf '%05d' "$i")
  segment_path="${resolved_segment_template//%05d/$num}"
  mkdir -p "$(dirname "$segment_path")"
  printf 'segment' > "$segment_path"
done
exit "$exit_code"
`

	return exec.CommandContext(
		ctx,
		"bash",
		"-lc",
		script,
		"bash",
		playlistPath,
		segmentTemplate,
		masterPlaylistName,
		exitCode,
	)
}

func fakeFFProbeCommand(ctx context.Context, output string) *exec.Cmd {
	script := "printf '%s' '" + strings.ReplaceAll(output, "'", "'\\''") + "'"
	return exec.CommandContext(ctx, "bash", "-lc", script)
}
