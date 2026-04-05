package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"plum/internal/db"
	"plum/internal/transcoder"
)

func writeFFprobeShim(t *testing.T, payload string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ffprobe")
	script := "#!/bin/sh\ncat <<'EOF'\n" + payload + "\nEOF\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write ffprobe shim: %v", err)
	}
	originalPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+originalPath); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
	})
}

func TestWarmEmbeddedSubtitleCachesReturnsAccepted(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"test@example.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"Anime",
		db.LibraryTypeAnime,
		"/anime",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	var episodeID int
	if err := dbConn.QueryRow(
		`INSERT INTO anime_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Test E1",
		"/anime/e1.mkv",
		0,
		db.MatchStatusLocal,
		1,
		1,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert episode: %v", err)
	}

	var globalID int
	if err := dbConn.QueryRow(
		`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`,
		db.LibraryTypeAnime,
		episodeID,
	).Scan(&globalID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}

	handler := &PlaybackHandler{DB: dbConn, Sessions: transcoder.NewPlaybackSessionManager(context.Background(), t.TempDir(), nil)}
	req := httptest.NewRequest(http.MethodPost, "/api/media/"+strconv.Itoa(globalID)+"/embedded-subtitles/warm-cache", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(globalID))
	req = req.WithContext(withUser(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), &db.User{ID: userID}))
	rec := httptest.NewRecorder()
	handler.WarmEmbeddedSubtitleCaches(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("warm-cache status = %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestCreateSessionReturnsNotFoundWhenMediaFileIsMissing(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"test@example.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"Anime",
		db.LibraryTypeAnime,
		"/anime",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	var episodeID int
	if err := dbConn.QueryRow(
		`INSERT INTO anime_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Dragon Ball - S01E11",
		"/anime/Dragonball/Season 01/Dragon Ball (1986) - S01E11 - The Penalty is Pinball [SDTV][AAC 2.0][x265].mkv",
		0,
		db.MatchStatusLocal,
		1,
		11,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert episode: %v", err)
	}

	var globalID int
	if err := dbConn.QueryRow(
		`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`,
		db.LibraryTypeAnime,
		episodeID,
	).Scan(&globalID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}

	handler := &PlaybackHandler{
		DB:       dbConn,
		Sessions: transcoder.NewPlaybackSessionManager(context.Background(), t.TempDir(), nil),
	}
	req := httptest.NewRequest(http.MethodPost, "/api/playback/sessions/"+strconv.Itoa(globalID), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(globalID))
	req = req.WithContext(withUser(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), &db.User{ID: userID}))
	rec := httptest.NewRecorder()

	handler.CreateSession(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "media file not found on disk") {
		t.Fatalf("body = %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Rescan the library") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestCreateSessionReturnsSidecarSubtitlesBeforeEnrichment(t *testing.T) {
	writeFFprobeShim(t, `{"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","bit_rate":"128000","duration":"120"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"index":1,"codec_type":"audio","codec_name":"aac","channels":2}]}`)

	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	root := t.TempDir()
	videoPath := filepath.Join(root, "Example Movie.mp4")
	subtitlePath := filepath.Join(root, "Example Movie.en.srt")
	if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write video: %v", err)
	}
	if err := os.WriteFile(subtitlePath, []byte("1\n00:00:00,000 --> 00:00:01,000\nHello\n"), 0o644); err != nil {
		t.Fatalf("write subtitle: %v", err)
	}

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"test@example.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"Movies",
		db.LibraryTypeMovie,
		root,
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var movieID int
	if err := dbConn.QueryRow(
		`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Example Movie",
		videoPath,
		120,
		db.MatchStatusLocal,
	).Scan(&movieID); err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	var globalID int
	if err := dbConn.QueryRow(
		`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`,
		db.LibraryTypeMovie,
		movieID,
	).Scan(&globalID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}

	handler := &PlaybackHandler{
		DB:       dbConn,
		Sessions: transcoder.NewPlaybackSessionManager(context.Background(), t.TempDir(), nil),
	}
	body := strings.NewReader(`{"clientCapabilities":{"supportsNativeHls":false,"supportsMseHls":false,"videoCodecs":["h264"],"audioCodecs":["aac"],"containers":["mp4"]}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/playback/sessions/"+strconv.Itoa(globalID), body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(globalID))
	req = req.WithContext(withUser(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), &db.User{ID: userID}))
	rec := httptest.NewRecorder()

	handler.CreateSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Delivery  string `json:"delivery"`
		Subtitles []struct {
			Language string `json:"language"`
		} `json:"subtitles"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Delivery != "direct" {
		t.Fatalf("delivery = %q", payload.Delivery)
	}
	if len(payload.Subtitles) != 1 || payload.Subtitles[0].Language != "en" {
		t.Fatalf("subtitles = %#v", payload.Subtitles)
	}
}

func TestRefreshPlaybackTracksReturnsOnDemandMetadata(t *testing.T) {
	writeFFprobeShim(t, `{"format":{"format_name":"matroska,webm","bit_rate":"256000","duration":"180"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"index":1,"codec_type":"audio","codec_name":"aac","channels":2},{"index":7,"codec_type":"subtitle","codec_name":"hdmv_pgs_subtitle","tags":{"language":"eng","title":"English PGS"}}]}`)

	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	root := t.TempDir()
	videoPath := filepath.Join(root, "Refresh Target.mkv")
	subtitlePath := filepath.Join(root, "Refresh Target.en.srt")
	if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write video: %v", err)
	}
	if err := os.WriteFile(subtitlePath, []byte("1\n00:00:00,000 --> 00:00:01,000\nHello\n"), 0o644); err != nil {
		t.Fatalf("write subtitle: %v", err)
	}

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"refresh@example.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"Shows",
		db.LibraryTypeTV,
		root,
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var episodeID int
	if err := dbConn.QueryRow(
		`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Refresh Target - S01E01",
		videoPath,
		180,
		db.MatchStatusLocal,
		1,
		1,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert episode: %v", err)
	}
	var globalID int
	if err := dbConn.QueryRow(
		`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`,
		db.LibraryTypeTV,
		episodeID,
	).Scan(&globalID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}

	handler := &PlaybackHandler{DB: dbConn}
	req := httptest.NewRequest(http.MethodPost, "/api/media/"+strconv.Itoa(globalID)+"/playback-tracks/refresh", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(globalID))
	req = req.WithContext(withUser(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), &db.User{ID: userID}))
	rec := httptest.NewRecorder()

	handler.RefreshPlaybackTracks(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Subtitles []struct {
			Language string `json:"language"`
		} `json:"subtitles"`
		EmbeddedSubtitles []struct {
			Codec     string `json:"codec"`
			Supported bool   `json:"supported"`
		} `json:"embeddedSubtitles"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Subtitles) != 1 || payload.Subtitles[0].Language != "en" {
		t.Fatalf("subtitles = %#v", payload.Subtitles)
	}
	if len(payload.EmbeddedSubtitles) != 1 {
		t.Fatalf("embedded subtitles = %#v", payload.EmbeddedSubtitles)
	}
	if payload.EmbeddedSubtitles[0].Codec != "hdmv_pgs_subtitle" || !payload.EmbeddedSubtitles[0].Supported {
		t.Fatalf("embedded subtitles = %#v", payload.EmbeddedSubtitles)
	}
}
