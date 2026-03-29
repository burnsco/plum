package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"plum/internal/db"
	"plum/internal/transcoder"
)

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
		Sessions: transcoder.NewPlaybackSessionManager(t.TempDir(), nil),
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

func TestServeShowArtwork_ReturnsInternalErrorWhenAccessLookupFails(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, role, is_admin, created_at) VALUES (?, ?, ?, 1, ?) RETURNING id`,
		"user@example.com",
		"hash",
		db.UserRoleAdmin,
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
		"/shows",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	if _, err := dbConn.Exec(`DROP TABLE user_library_access`); err != nil {
		t.Fatalf("drop user_library_access: %v", err)
	}

	handler := &PlaybackHandler{DB: dbConn}
	req := httptest.NewRequest(http.MethodGet, "/api/libraries/"+strconv.Itoa(libraryID)+"/shows/test/artwork", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	rctx.URLParams.Add("showKey", "test")
	req = req.WithContext(withUser(context.WithValue(req.Context(), chi.RouteCtxKey, rctx), &db.User{ID: userID, Role: db.UserRoleAdmin, IsAdmin: true}))
	rec := httptest.NewRecorder()

	handler.ServeShowArtwork(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}
