package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"plum/internal/metadata"

	_ "modernc.org/sqlite"
)

// newTestDB connects to SQLite (PLUM_TEST_DATABASE_URL or :memory:) and creates schema.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn := os.Getenv("PLUM_TEST_DATABASE_URL")
	if conn == "" {
		conn = ":memory:"
	}
	db, err := sql.Open("sqlite", conn)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping sqlite: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		t.Fatalf("pragma foreign_keys: %v", err)
	}
	if err := createSchema(db); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	// Insert a test user and two libraries (tv and movie) for scans
	_, _ = db.Exec(`DELETE FROM libraries`)
	_, _ = db.Exec(`DELETE FROM users`)
	var userID int
	err = db.QueryRow(`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`, "test@test.com", "hash", time.Now().UTC()).Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	now := time.Now().UTC()
	_, err = db.Exec(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?), (?, 'Movies', 'movie', ?, ?)`,
		userID, "TV", "tv", "/tv", now, userID, "/movies", now)
	if err != nil {
		t.Fatalf("insert libraries: %v", err)
	}
	return db
}

func getLibraryID(t *testing.T, db *sql.DB, typ string) int {
	t.Helper()
	var id int
	err := db.QueryRow(`SELECT id FROM libraries WHERE type = ? LIMIT 1`, typ).Scan(&id)
	if err != nil {
		t.Fatalf("get library id for %s: %v", typ, err)
	}
	return id
}

func createLibraryForTest(t *testing.T, db *sql.DB, typ, path string) int {
	t.Helper()
	var userID int
	if err := db.QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("get user id: %v", err)
	}
	var id int
	if err := db.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID, fmt.Sprintf("%s library", typ), typ, path, time.Now().UTC()).Scan(&id); err != nil {
		t.Fatalf("create library %s: %v", typ, err)
	}
	return id
}

func columnExistsForTest(t *testing.T, db *sql.DB, table, column string) bool {
	t.Helper()
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		t.Fatalf("pragma table_info(%s): %v", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			t.Fatalf("scan pragma table_info(%s): %v", table, err)
		}
		if name == column {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pragma table_info(%s): %v", table, err)
	}
	return false
}

func indexExistsForTest(t *testing.T, db *sql.DB, table, index string) bool {
	t.Helper()
	rows, err := db.Query(fmt.Sprintf("PRAGMA index_list(%s)", table))
	if err != nil {
		t.Fatalf("pragma index_list(%s): %v", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			seq     int
			name    string
			unique  int
			origin  string
			partial int
		)
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan pragma index_list(%s): %v", table, err)
		}
		if name == index {
			return true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pragma index_list(%s): %v", table, err)
	}
	return false
}

func TestCreateSchema_MigratesLegacyTables(t *testing.T) {
	dbConn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer dbConn.Close()

	legacySchema := `
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  is_admin INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL
);
CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at DATETIME NOT NULL,
  expires_at DATETIME NOT NULL
);
CREATE TABLE libraries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  type TEXT NOT NULL CHECK (type IN ('tv','movie','music','anime')),
  path TEXT NOT NULL,
  created_at DATETIME NOT NULL
);
CREATE TABLE media_global (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  kind TEXT NOT NULL CHECK (kind IN ('movie','tv','anime','music')),
  ref_id INTEGER NOT NULL
);
CREATE TABLE movies (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  path TEXT NOT NULL,
  duration INTEGER NOT NULL DEFAULT 0,
  tmdb_id INTEGER,
  overview TEXT,
  poster_path TEXT,
  backdrop_path TEXT,
  release_date TEXT,
  vote_average REAL DEFAULT 0
);
CREATE TABLE tv_episodes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  path TEXT NOT NULL,
  duration INTEGER NOT NULL DEFAULT 0,
  tmdb_id INTEGER,
  overview TEXT,
  poster_path TEXT,
  backdrop_path TEXT,
  release_date TEXT,
  vote_average REAL DEFAULT 0
);
CREATE TABLE anime_episodes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  path TEXT NOT NULL,
  duration INTEGER NOT NULL DEFAULT 0,
  tmdb_id INTEGER,
  overview TEXT,
  poster_path TEXT,
  backdrop_path TEXT,
  release_date TEXT,
  vote_average REAL DEFAULT 0
);
CREATE TABLE music_tracks (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  library_id INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
  title TEXT NOT NULL,
  path TEXT NOT NULL,
  duration INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE library_job_status (
  library_id INTEGER PRIMARY KEY REFERENCES libraries(id) ON DELETE CASCADE,
  phase TEXT NOT NULL
);`
	if _, err := dbConn.Exec(legacySchema); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	if err := createSchema(dbConn); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	for _, tc := range []struct {
		table  string
		column string
	}{
		{table: "libraries", column: "preferred_audio_language"},
		{table: "libraries", column: "preferred_subtitle_language"},
		{table: "libraries", column: "subtitles_enabled_by_default"},
		{table: "movies", column: "match_status"},
		{table: "movies", column: "tvdb_id"},
		{table: "movies", column: "imdb_id"},
		{table: "movies", column: "imdb_rating"},
		{table: "tv_episodes", column: "season"},
		{table: "tv_episodes", column: "episode"},
		{table: "tv_episodes", column: "metadata_review_needed"},
		{table: "tv_episodes", column: "metadata_confirmed"},
		{table: "tv_episodes", column: "thumbnail_path"},
		{table: "anime_episodes", column: "metadata_review_needed"},
		{table: "anime_episodes", column: "metadata_confirmed"},
		{table: "music_tracks", column: "artist"},
		{table: "music_tracks", column: "album"},
		{table: "music_tracks", column: "poster_path"},
		{table: "music_tracks", column: "musicbrainz_artist_id"},
		{table: "music_tracks", column: "musicbrainz_release_group_id"},
		{table: "music_tracks", column: "musicbrainz_release_id"},
		{table: "music_tracks", column: "musicbrainz_recording_id"},
		{table: "library_job_status", column: "identify_phase"},
		{table: "library_job_status", column: "queued_at"},
		{table: "library_job_status", column: "estimated_items"},
		{table: "library_job_status", column: "updated_at"},
		{table: "media_attachments", column: "stream_index"},
		{table: "media_attachments", column: "file_name"},
		{table: "media_attachments", column: "mime_type"},
	} {
		if !columnExistsForTest(t, dbConn, tc.table, tc.column) {
			t.Fatalf("expected %s.%s to exist after migration", tc.table, tc.column)
		}
	}

	for _, tc := range []struct {
		table string
		index string
	}{
		{table: "subtitles", index: "idx_subtitles_path"},
		{table: "library_job_status", index: "idx_library_job_status_phase_updated_at"},
		{table: "movies", index: "idx_movies_library_match_status"},
		{table: "tv_episodes", index: "idx_tv_episodes_library_match_status"},
		{table: "anime_episodes", index: "idx_anime_episodes_library_match_status"},
		{table: "media_attachments", index: "idx_media_attachments_media_stream"},
	} {
		if !indexExistsForTest(t, dbConn, tc.table, tc.index) {
			t.Fatalf("expected index %s on %s", tc.index, tc.table)
		}
	}
}

func TestInsertScannedItem_RollsBackOnMediaGlobalFailure(t *testing.T) {
	dbConn := newTestDB(t)
	libraryID := getLibraryID(t, dbConn, "movie")

	_, _, err := insertScannedItem(context.Background(), dbConn, "movies", "invalid", libraryID, MediaItem{
		Title:       "Broken Insert",
		Path:        "/movies/broken.mp4",
		Duration:    60,
		MatchStatus: MatchStatusLocal,
	}, time.Now().UTC().Format(time.RFC3339))
	if err == nil {
		t.Fatal("expected insertScannedItem to fail")
	}

	var count int
	if err := dbConn.QueryRow(`SELECT COUNT(*) FROM movies WHERE path = ?`, "/movies/broken.mp4").Scan(&count); err != nil {
		t.Fatalf("count movies: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected rollback to remove inserted movie row, found %d", count)
	}
}

func TestHandleStreamSubtitle_DoesNotWritePartialResponseOnConversionError(t *testing.T) {
	dbConn := newTestDB(t)
	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("get user id: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID, "Movies 2", "movie", "/movies2", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var refID int
	if err := dbConn.QueryRow(`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		libraryID, "Missing", "/movies/missing.mp4", 100, MatchStatusLocal).Scan(&refID); err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	var mediaID int
	if err := dbConn.QueryRow(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`, LibraryTypeMovie, refID).Scan(&mediaID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}
	var subtitleID int
	if err := dbConn.QueryRow(`INSERT INTO subtitles (media_id, title, language, format, path) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		mediaID, "Broken", "en", "srt", "/does/not/exist.srt").Scan(&subtitleID); err != nil {
		t.Fatalf("insert subtitle: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/subtitles/%d", subtitleID), nil)
	err := HandleStreamSubtitle(rec, req, dbConn, subtitleID)
	if err == nil {
		t.Fatal("expected subtitle conversion error")
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty response body on subtitle error, got %q", rec.Body.String())
	}
}

func TestHandleStreamEmbeddedSubtitle_ReturnsNotFoundForMissingStream(t *testing.T) {
	dbConn := newTestDB(t)
	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("get user id: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID, "TV 2", "tv", "/tv2", now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var refID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID, "Missing Stream - S01E01", "/tv2/Missing Stream/Season 1/Episode 1.mkv", 100, MatchStatusLocal, 1, 1).Scan(&refID); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}
	var mediaID int
	if err := dbConn.QueryRow(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`, LibraryTypeTV, refID).Scan(&mediaID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO embedded_subtitles (media_id, stream_index, language, title) VALUES (?, ?, ?, ?)`,
		mediaID, 1, "en", "English"); err != nil {
		t.Fatalf("insert embedded subtitle: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/media/%d/subtitles/embedded/%d", mediaID, 99), nil)
	err := HandleStreamEmbeddedSubtitle(rec, req, dbConn, mediaID, 99)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty response body on embedded subtitle error, got %q", rec.Body.String())
	}
}

func TestHandleStreamEmbeddedSubtitle_ReturnsClientErrorForUnsupportedCodec(t *testing.T) {
	dbConn := newTestDB(t)
	root := t.TempDir()
	sourcePath := filepath.Join(root, "Episode 1.mkv")
	if err := os.WriteFile(sourcePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("get user id: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID, "TV 4", "tv", root, now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var refID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID, "Unsupported Stream - S01E01", sourcePath, 100, MatchStatusLocal, 1, 1).Scan(&refID); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}
	var mediaID int
	if err := dbConn.QueryRow(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`, LibraryTypeTV, refID).Scan(&mediaID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO embedded_subtitles (media_id, stream_index, language, title, codec, supported) VALUES (?, ?, ?, ?, ?, ?)`,
		mediaID, 7, "en", "English PGS", "hdmv_pgs_subtitle", 0); err != nil {
		t.Fatalf("insert embedded subtitle: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/media/%d/subtitles/embedded/%d", mediaID, 7), nil)
	err := HandleStreamEmbeddedSubtitle(rec, req, dbConn, mediaID, 7)
	if err == nil {
		t.Fatal("expected unsupported subtitle error")
	}
	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected StatusError, got %T %v", err, err)
	}
	if statusErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d", statusErr.Status)
	}
	if !strings.Contains(statusErr.Error(), "hdmv_pgs_subtitle") {
		t.Fatalf("error = %q", statusErr.Error())
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty response body on embedded subtitle error, got %q", rec.Body.String())
	}
}

func TestHandleStreamEmbeddedSubtitleSup_RejectsNonPgsCodec(t *testing.T) {
	dbConn := newTestDB(t)
	root := t.TempDir()
	sourcePath := filepath.Join(root, "Episode sup.mkv")
	if err := os.WriteFile(sourcePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("get user id: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID, "TV sup1", "tv", root, now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var refID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID, "Sup Reject - S01E01", sourcePath, 100, MatchStatusLocal, 1, 1).Scan(&refID); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}
	var mediaID int
	if err := dbConn.QueryRow(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`, LibraryTypeTV, refID).Scan(&mediaID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO embedded_subtitles (media_id, stream_index, language, title, codec, supported) VALUES (?, ?, ?, ?, ?, ?)`,
		mediaID, 3, "en", "English", "subrip", 1); err != nil {
		t.Fatalf("insert embedded subtitle: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/media/%d/subtitles/embedded/%d/sup", mediaID, 3), nil)
	err := HandleStreamEmbeddedSubtitleSup(rec, req, dbConn, mediaID, 3)
	if err == nil {
		t.Fatal("expected error for non-PGS codec")
	}
	var statusErr *StatusError
	if !errors.As(err, &statusErr) || statusErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("expected StatusError 422, got %v", err)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestHandleStreamEmbeddedSubtitleSup_ServesWithShim(t *testing.T) {
	dbConn := newTestDB(t)
	root := t.TempDir()
	sourcePath := filepath.Join(root, "Episode sup2.mkv")
	if err := os.WriteFile(sourcePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	ffmpegDir := t.TempDir()
	ffmpegPath := filepath.Join(ffmpegDir, "ffmpeg")
	ffmpegScript := "#!/bin/sh\n" +
		"case \" $* \" in\n" +
		"  *\" -f sup \"*) last=\"\"; for a in \"$@\"; do last=\"$a\"; done; case \"$last\" in -) printf 'PGSOUT' ;; esac ;;\n" +
		"  *) last=\"\"; for a in \"$@\"; do last=\"$a\"; done; case \"$last\" in -) printf 'WEBVTT' ;; *) printf 'x' >\"$last\" ;; esac ;;\n" +
		"esac\n"
	if err := os.WriteFile(ffmpegPath, []byte(ffmpegScript), 0o755); err != nil {
		t.Fatalf("write ffmpeg shim: %v", err)
	}

	originalPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", ffmpegDir+string(os.PathListSeparator)+originalPath); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
	})

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("get user id: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID, "TV sup2", "tv", root, now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var refID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID, "Sup OK - S01E01", sourcePath, 100, MatchStatusLocal, 1, 1).Scan(&refID); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}
	var mediaID int
	if err := dbConn.QueryRow(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`, LibraryTypeTV, refID).Scan(&mediaID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO embedded_subtitles (media_id, stream_index, language, title, codec, supported) VALUES (?, ?, ?, ?, ?, ?)`,
		mediaID, 5, "en", "English PGS", "hdmv_pgs_subtitle", 1); err != nil {
		t.Fatalf("insert embedded subtitle: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/media/%d/subtitles/embedded/%d/sup", mediaID, 5), nil)
	if err := HandleStreamEmbeddedSubtitleSup(rec, req, dbConn, mediaID, 5); err != nil {
		t.Fatalf("HandleStreamEmbeddedSubtitleSup: %v", err)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/pgs" {
		t.Fatalf("content-type = %q", ct)
	}
	if rec.Body.String() != "PGSOUT" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestHandleStreamEmbeddedSubtitle_ServesConvertedVTT(t *testing.T) {
	dbConn := newTestDB(t)
	root := t.TempDir()
	sourcePath := filepath.Join(root, "Episode 1.mkv")
	if err := os.WriteFile(sourcePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	ffmpegDir := t.TempDir()
	ffmpegPath := filepath.Join(ffmpegDir, "ffmpeg")
	ffmpegScript := "#!/bin/sh\n" +
		"last=\"\"\n" +
		"for a in \"$@\"; do last=\"$a\"; done\n" +
		"case \"$last\" in\n" +
		"  -) printf 'WEBVTT\\n\\n00:00:00.000 --> 00:00:02.000\\nHello world\\n' ;;\n" +
		"  *) printf '1\\n00:00:00,000 --> 00:00:02,000\\nHello\\n' >\"$last\" ;;\n" +
		"esac\n"
	if err := os.WriteFile(ffmpegPath, []byte(ffmpegScript), 0o755); err != nil {
		t.Fatalf("write ffmpeg shim: %v", err)
	}

	originalPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", ffmpegDir+string(os.PathListSeparator)+originalPath); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
	})

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("get user id: %v", err)
	}
	var libraryID int
	if err := dbConn.QueryRow(`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID, "TV 3", "tv", root, now).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}
	var refID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID, "Embedded Stream - S01E01", sourcePath, 100, MatchStatusLocal, 1, 1).Scan(&refID); err != nil {
		t.Fatalf("insert tv episode: %v", err)
	}
	var mediaID int
	if err := dbConn.QueryRow(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`, LibraryTypeTV, refID).Scan(&mediaID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO embedded_subtitles (media_id, stream_index, language, title) VALUES (?, ?, ?, ?)`,
		mediaID, 7, "en", "English"); err != nil {
		t.Fatalf("insert embedded subtitle: %v", err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/media/%d/subtitles/embedded/%d", mediaID, 7), nil)
	if err := HandleStreamEmbeddedSubtitle(rec, req, dbConn, mediaID, 7); err != nil {
		t.Fatalf("HandleStreamEmbeddedSubtitle: %v", err)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/vtt") {
		t.Fatalf("expected text/vtt content type, got %q", contentType)
	}
	if body := rec.Body.String(); body != "WEBVTT\n\n00:00:00.000 --> 00:00:02.000\nHello world\n" {
		t.Fatalf("unexpected embedded subtitle body: %q", body)
	}
}

func TestCompactWebVTTCueOverlaps_TrimsEarlierASSDialogueCue(t *testing.T) {
	input := []byte("WEBVTT\n\n" +
		"00:00:01.000 --> 00:00:04.000 align:middle\n" +
		"First line\n\n" +
		"00:00:03.000 --> 00:00:05.000 align:middle\n" +
		"Second line\n")

	got, err := compactWebVTTCueOverlaps(input)
	if err != nil {
		t.Fatalf("compactWebVTTCueOverlaps: %v", err)
	}
	want := "WEBVTT\n\n" +
		"00:00:01.000 --> 00:00:03.000 align:middle\n" +
		"First line\n\n" +
		"00:00:03.000 --> 00:00:05.000 align:middle\n" +
		"Second line\n"
	if string(got) != want {
		t.Fatalf("unexpected VTT:\n%s", got)
	}
}

func TestCompactWebVTTCueOverlaps_LeavesSameStartCuesTogether(t *testing.T) {
	input := []byte("WEBVTT\n\n" +
		"00:00:01.000 --> 00:00:04.000\n" +
		"First line\n\n" +
		"00:00:01.000 --> 00:00:05.000\n" +
		"Second line\n")

	got, err := compactWebVTTCueOverlaps(input)
	if err != nil {
		t.Fatalf("compactWebVTTCueOverlaps: %v", err)
	}
	if string(got) != string(input) {
		t.Fatalf("expected same-start cues to remain unchanged, got:\n%s", got)
	}
}

func TestListIdentifiableByLibrary_SkipsConfirmedEpisodes(t *testing.T) {
	dbConn := newTestDB(t)
	tvLibID := getLibraryID(t, dbConn, "tv")

	if _, err := dbConn.Exec(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, tmdb_id, metadata_confirmed, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tvLibID,
		"Confirmed Show - S01E01",
		"/tv/Confirmed Show/Season 1/Confirmed Show - S01E01.mkv",
		120,
		MatchStatusIdentified,
		123,
		true,
		1,
		1,
	); err != nil {
		t.Fatalf("insert confirmed episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		tvLibID,
		"Unmatched Show - S01E02",
		"/tv/Unmatched Show/Season 1/Unmatched Show - S01E02.mkv",
		120,
		MatchStatusUnmatched,
		1,
		2,
	); err != nil {
		t.Fatalf("insert unmatched episode: %v", err)
	}

	rows, err := ListIdentifiableByLibrary(dbConn, tvLibID)
	if err != nil {
		t.Fatalf("ListIdentifiableByLibrary: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 identifiable row, got %d", len(rows))
	}
	if rows[0].Title != "Unmatched Show - S01E02" {
		t.Fatalf("unexpected identifiable row: %#v", rows[0])
	}
}

func TestListEpisodeIdentifyRowsByLibrary_SkipsConfirmedEpisodes(t *testing.T) {
	dbConn := newTestDB(t)
	tvLibID := getLibraryID(t, dbConn, "tv")

	if _, err := dbConn.Exec(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, tmdb_id, metadata_confirmed, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tvLibID,
		"Confirmed Show - S01E01",
		"/tv/Confirmed Show/Season 1/Confirmed Show - S01E01.mkv",
		120,
		MatchStatusIdentified,
		123,
		true,
		1,
		1,
	); err != nil {
		t.Fatalf("insert confirmed episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, tmdb_id, tvdb_id, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tvLibID,
		"Repair Show - S01E02",
		"/tv/Repair Show/Season 1/Repair Show - S01E02.mkv",
		120,
		MatchStatusIdentified,
		456,
		"tvdb-456",
		1,
		2,
	); err != nil {
		t.Fatalf("insert repair episode: %v", err)
	}

	rows, err := ListEpisodeIdentifyRowsByLibrary(dbConn, tvLibID)
	if err != nil {
		t.Fatalf("ListEpisodeIdentifyRowsByLibrary: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 episodic row, got %d", len(rows))
	}
	if rows[0].Title != "Repair Show - S01E02" {
		t.Fatalf("unexpected episodic row: %#v", rows[0])
	}
	if rows[0].TMDBID != 456 || rows[0].TVDBID != "tvdb-456" {
		t.Fatalf("unexpected provider ids: tmdb=%d tvdb=%q", rows[0].TMDBID, rows[0].TVDBID)
	}
}

// TestHandleScanLibrary_RecursesSubdirectories verifies that HandleScanLibrary walks
// nested directories and inserts into the correct category tables (tv_episodes, movies).
func TestHandleScanLibrary_RecursesSubdirectories(t *testing.T) {
	db := newTestDB(t)
	tvLibID := getLibraryID(t, db, "tv")
	movieLibID := getLibraryID(t, db, "movie")

	tmp := t.TempDir()
	tvRoot := filepath.Join(tmp, "SomeShow")
	movieRoot := filepath.Join(tmp, "Movies")

	if err := os.MkdirAll(filepath.Join(tvRoot, "Season 1"), 0o755); err != nil {
		t.Fatalf("mkdir tv tree: %v", err)
	}
	if err := os.MkdirAll(movieRoot, 0o755); err != nil {
		t.Fatalf("mkdir movies tree: %v", err)
	}

	tvFile1 := filepath.Join(tvRoot, "Season 1", "episode.s01e01.mkv")
	tvFile2 := filepath.Join(tvRoot, "Season 1", "episode.s01e02.mkv")
	movieFile := filepath.Join(movieRoot, "movie1.mp4")

	for _, p := range []string{tvFile1, tvFile2, movieFile} {
		if err := os.WriteFile(p, []byte("test"), 0o644); err != nil {
			t.Fatalf("write fake media %s: %v", p, err)
		}
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	ctx := context.Background()

	addedTV, err := HandleScanLibrary(ctx, db, tvRoot, LibraryTypeTV, tvLibID, nil)
	if err != nil {
		t.Fatalf("scan tv library: %v", err)
	}
	if addedTV.Added != 2 {
		t.Fatalf("expected 2 tv items added, got %d", addedTV.Added)
	}

	addedMovies, err := HandleScanLibrary(ctx, db, movieRoot, LibraryTypeMovie, movieLibID, nil)
	if err != nil {
		t.Fatalf("scan movie library: %v", err)
	}
	if addedMovies.Added != 1 {
		t.Fatalf("expected 1 movie item added, got %d", addedMovies.Added)
	}

	var countTV, countMovies int
	_ = db.QueryRow(`SELECT COUNT(*) FROM tv_episodes WHERE library_id = ?`, tvLibID).Scan(&countTV)
	_ = db.QueryRow(`SELECT COUNT(*) FROM movies WHERE library_id = ?`, movieLibID).Scan(&countMovies)
	if countTV != 2 || countMovies != 1 {
		t.Fatalf("expected 2 tv_episodes and 1 movie; got tv=%d movie=%d", countTV, countMovies)
	}

	rows, err := db.Query(`SELECT m.path FROM tv_episodes m WHERE m.library_id = ? UNION ALL SELECT m.path FROM movies m WHERE m.library_id = ?`, tvLibID, movieLibID)
	if err != nil {
		t.Fatalf("query paths: %v", err)
	}
	defer rows.Close()
	var foundTV1, foundTV2, foundMovie bool
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			t.Fatalf("scan path: %v", err)
		}
		switch p {
		case tvFile1:
			foundTV1 = true
		case tvFile2:
			foundTV2 = true
		case movieFile:
			foundMovie = true
		}
	}
	if !foundTV1 || !foundTV2 || !foundMovie {
		t.Fatalf("expected paths for tv1=%v tv2=%v movie=%v", foundTV1, foundTV2, foundMovie)
	}
}

// TestHandleScanLibrary_IsIdempotent verifies that rescanning the same root does
// not create duplicate rows in the category table.
func TestHandleScanLibrary_IsIdempotent(t *testing.T) {
	db := newTestDB(t)
	tvLibID := getLibraryID(t, db, "tv")

	tmp := t.TempDir()
	root := filepath.Join(tmp, "SomeShow", "Season 1")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir tree: %v", err)
	}
	file := filepath.Join(root, "episode.s01e01.mkv")
	if err := os.WriteFile(file, []byte("test"), 0o644); err != nil {
		t.Fatalf("write fake media: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	ctx := context.Background()

	addedFirst, err := HandleScanLibrary(ctx, db, filepath.Join(tmp, "SomeShow"), LibraryTypeTV, tvLibID, nil)
	if err != nil {
		t.Fatalf("first scan: %v", err)
	}
	if addedFirst.Added != 1 {
		t.Fatalf("expected 1 item on first scan, got %d", addedFirst.Added)
	}

	addedSecond, err := HandleScanLibrary(ctx, db, filepath.Join(tmp, "SomeShow"), LibraryTypeTV, tvLibID, nil)
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if addedSecond.Added != 0 {
		t.Fatalf("expected 0 items on second scan, got %d", addedSecond.Added)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM tv_episodes WHERE library_id = ?`, tvLibID).Scan(&count); err != nil {
		t.Fatalf("count tv_episodes: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 tv_episode row after two scans, got %d", count)
	}
}

func TestHandleScanLibraryWithOptions_ProgressSeesImportedRowsBeforeCompletion(t *testing.T) {
	dbConn, err := InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"progress@test.com",
		"hash",
		time.Now().UTC(),
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	tvLibID := createLibraryForTest(t, dbConn, LibraryTypeTV, "/tv-progress")
	root := t.TempDir()
	showDir := filepath.Join(root, "Fast Show", "Season 1")
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		t.Fatalf("mkdir show dir: %v", err)
	}
	for i := 1; i <= 2; i++ {
		file := filepath.Join(showDir, fmt.Sprintf("Fast Show - S01E0%d.mkv", i))
		if err := os.WriteFile(file, []byte("not a real video"), 0o644); err != nil {
			t.Fatalf("write media file: %v", err)
		}
	}

	visibleDuringProgress := false
	result, err := HandleScanLibraryWithOptions(context.Background(), dbConn, root, LibraryTypeTV, tvLibID, ScanOptions{
		Progress: func(progress ScanProgress) {
			if progress.Processed != 1 || visibleDuringProgress {
				return
			}
			var count int
			if err := dbConn.QueryRow(`SELECT COUNT(1) FROM tv_episodes WHERE library_id = ?`, tvLibID).Scan(&count); err != nil {
				t.Fatalf("count imported rows during progress: %v", err)
			}
			if count > 0 {
				visibleDuringProgress = true
			}
		},
	})
	if err != nil {
		t.Fatalf("scan library: %v", err)
	}
	if result.Added != 2 {
		t.Fatalf("added = %d, want 2", result.Added)
	}
	if !visibleDuringProgress {
		t.Fatal("expected imported rows to be visible before scan completion")
	}
}

func TestHandleScanLibraryWithOptions_DeferredHashProgressDoesNotWaitForHash(t *testing.T) {
	dbConn := newTestDB(t)
	tvLibID := createLibraryForTest(t, dbConn, LibraryTypeTV, "/tv-progress")
	root := t.TempDir()
	showDir := filepath.Join(root, "Fast Show", "Season 1")
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		t.Fatalf("mkdir show dir: %v", err)
	}
	for i := 1; i <= 2; i++ {
		file := filepath.Join(showDir, fmt.Sprintf("Fast Show - S01E0%d.mkv", i))
		if err := os.WriteFile(file, []byte("not a real video"), 0o644); err != nil {
			t.Fatalf("write media file: %v", err)
		}
	}

	prevHash := computeMediaHash
	computeMediaHash = func(_ context.Context, path string) (string, error) {
		t.Fatalf("deferred scan should not hash inline: %s", path)
		return "", nil
	}
	defer func() { computeMediaHash = prevHash }()

	visibleDuringProgress := false
	result, err := HandleScanLibraryWithOptions(context.Background(), dbConn, root, LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
		Progress: func(progress ScanProgress) {
			if progress.Processed != 1 || visibleDuringProgress {
				return
			}
			var count int
			if err := dbConn.QueryRow(`SELECT COUNT(1) FROM tv_episodes WHERE library_id = ?`, tvLibID).Scan(&count); err != nil {
				t.Fatalf("count imported rows during progress: %v", err)
			}
			if count > 0 {
				visibleDuringProgress = true
			}
		},
	})
	if err != nil {
		t.Fatalf("scan library: %v", err)
	}
	if result.Added != 2 {
		t.Fatalf("added = %d, want 2", result.Added)
	}
	if !visibleDuringProgress {
		t.Fatal("expected imported rows to be visible before scan completion")
	}

	var hashes int
	if err := dbConn.QueryRow(`SELECT COUNT(1) FROM tv_episodes WHERE library_id = ? AND COALESCE(file_hash, '') != ''`, tvLibID).Scan(&hashes); err != nil {
		t.Fatalf("count deferred hashes: %v", err)
	}
	if hashes != 0 {
		t.Fatalf("expected no hashes during deferred scan, got %d", hashes)
	}
}

// mockIdentifier returns fixed metadata for tests.
type mockIdentifier struct {
	tvResult    *metadata.MatchResult
	animeResult *metadata.MatchResult
	movieResult *metadata.MatchResult
}

func (m *mockIdentifier) IdentifyTV(_ context.Context, _ metadata.MediaInfo) *metadata.MatchResult {
	return m.tvResult
}

func (m *mockIdentifier) IdentifyAnime(_ context.Context, _ metadata.MediaInfo) *metadata.MatchResult {
	return m.animeResult
}

func (m *mockIdentifier) IdentifyMovie(_ context.Context, _ metadata.MediaInfo) *metadata.MatchResult {
	return m.movieResult
}

type funcIdentifier struct {
	tv    func(metadata.MediaInfo) *metadata.MatchResult
	anime func(metadata.MediaInfo) *metadata.MatchResult
	movie func(metadata.MediaInfo) *metadata.MatchResult
}

func (f *funcIdentifier) IdentifyTV(_ context.Context, info metadata.MediaInfo) *metadata.MatchResult {
	if f.tv == nil {
		return nil
	}
	return f.tv(info)
}

func (f *funcIdentifier) IdentifyAnime(_ context.Context, info metadata.MediaInfo) *metadata.MatchResult {
	if f.anime == nil {
		return nil
	}
	return f.anime(info)
}

func (f *funcIdentifier) IdentifyMovie(_ context.Context, info metadata.MediaInfo) *metadata.MatchResult {
	if f.movie == nil {
		return nil
	}
	return f.movie(info)
}

// TestHandleScanLibrary_WithMockIdentifier verifies that when a mock identifier returns a match,
// the stored row has the expected title and overview.
func TestHandleScanLibrary_WithMockIdentifier(t *testing.T) {
	db := newTestDB(t)
	tvLibID := getLibraryID(t, db, "tv")

	tmp := t.TempDir()
	root := filepath.Join(tmp, "Show", "Season 1")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir tree: %v", err)
	}
	file := filepath.Join(root, "Show.S01E03.mkv")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fake media: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	ctx := context.Background()
	mock := &mockIdentifier{
		tvResult: &metadata.MatchResult{
			Title:       "Mock Show - S01E03 - The Episode",
			Overview:    "Mock overview for testing.",
			PosterURL:   "https://example.com/poster.jpg",
			BackdropURL: "https://example.com/backdrop.jpg",
			ReleaseDate: "2024-01-15",
			VoteAverage: 8.5,
			Provider:    "tmdb",
			ExternalID:  "12345",
		},
	}

	added, err := HandleScanLibrary(ctx, db, filepath.Join(tmp, "Show"), LibraryTypeTV, tvLibID, mock)
	if err != nil {
		t.Fatalf("scan with mock: %v", err)
	}
	if added.Added != 1 {
		t.Fatalf("expected 1 item added, got %d", added.Added)
	}

	var title, overview string
	err = db.QueryRow(`SELECT title, overview FROM tv_episodes WHERE library_id = ? AND path = ?`, tvLibID, file).Scan(&title, &overview)
	if err != nil {
		t.Fatalf("query stored row: %v", err)
	}
	if title != "Mock Show - S01E03 - The Episode" {
		t.Errorf("title: got %q", title)
	}
	if overview != "Mock overview for testing." {
		t.Errorf("overview: got %q", overview)
	}
}

func TestHandleScanLibrary_RefreshesMetadataWhenExplicitIDAppears(t *testing.T) {
	db := newTestDB(t)
	tvLibID := getLibraryID(t, db, "tv")

	tmp := t.TempDir()
	showRoot := filepath.Join(tmp, "Show")
	seasonRoot := filepath.Join(showRoot, "Season 1")
	if err := os.MkdirAll(seasonRoot, 0o755); err != nil {
		t.Fatalf("mkdir tree: %v", err)
	}
	file := filepath.Join(seasonRoot, "Show.S01E01.mkv")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fake media: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	ctx := context.Background()
	initial := &mockIdentifier{
		tvResult: &metadata.MatchResult{
			Title:      "Wrong Show - S01E01 - Wrong",
			Overview:   "wrong",
			Provider:   "tmdb",
			ExternalID: "111",
		},
	}
	if _, err := HandleScanLibrary(ctx, db, tmp, LibraryTypeTV, tvLibID, initial); err != nil {
		t.Fatalf("first scan: %v", err)
	}

	nfo := `<tvshow><uniqueid type="tmdb">222</uniqueid></tvshow>`
	if err := os.WriteFile(filepath.Join(showRoot, "tvshow.nfo"), []byte(nfo), 0o644); err != nil {
		t.Fatalf("write nfo: %v", err)
	}

	refresh := &funcIdentifier{
		tv: func(info metadata.MediaInfo) *metadata.MatchResult {
			if info.TMDBID != 222 {
				t.Fatalf("expected TMDBID 222 from nfo, got %d", info.TMDBID)
			}
			return &metadata.MatchResult{
				Title:      "Right Show - S01E01 - Pilot",
				Overview:   "right",
				Provider:   "tmdb",
				ExternalID: "222",
			}
		},
	}
	if _, err := HandleScanLibrary(ctx, db, tmp, LibraryTypeTV, tvLibID, refresh); err != nil {
		t.Fatalf("second scan: %v", err)
	}

	var title, overview string
	var tmdbID int
	err := db.QueryRow(`SELECT title, overview, COALESCE(tmdb_id, 0) FROM tv_episodes WHERE library_id = ? AND path = ?`, tvLibID, file).Scan(&title, &overview, &tmdbID)
	if err != nil {
		t.Fatalf("query stored row: %v", err)
	}
	if title != "Right Show - S01E01 - Pilot" {
		t.Fatalf("title = %q", title)
	}
	if overview != "right" {
		t.Fatalf("overview = %q", overview)
	}
	if tmdbID != 222 {
		t.Fatalf("tmdb_id = %d", tmdbID)
	}
}

func TestListIdentifiableByLibrary_IncludesRowsWithMetadata(t *testing.T) {
	db := newTestDB(t)
	tvLibID := getLibraryID(t, db, "tv")

	tmp := t.TempDir()
	showRoot := filepath.Join(tmp, "Show", "Season 1")
	if err := os.MkdirAll(showRoot, 0o755); err != nil {
		t.Fatalf("mkdir tree: %v", err)
	}
	file := filepath.Join(showRoot, "Show.S01E01.mkv")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fake media: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	ctx := context.Background()
	mock := &mockIdentifier{
		tvResult: &metadata.MatchResult{
			Title:      "Show - S01E01 - Pilot",
			Provider:   "tmdb",
			ExternalID: "333",
		},
	}
	if _, err := HandleScanLibrary(ctx, db, filepath.Join(tmp, "Show"), LibraryTypeTV, tvLibID, mock); err != nil {
		t.Fatalf("scan: %v", err)
	}

	rows, err := ListIdentifiableByLibrary(db, tvLibID)
	if err != nil {
		t.Fatalf("list rows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Path != file {
		t.Fatalf("path = %q", rows[0].Path)
	}
}

func TestListIdentifiableByLibrary_MovieOmitsIdentifiedWithoutImdb(t *testing.T) {
	db := newTestDB(t)
	movieLibID := getLibraryID(t, db, "movie")
	path := "/movies/identified-no-imdb.mkv"
	_, err := db.Exec(
		`INSERT INTO movies (library_id, title, path, duration, match_status, tmdb_id, poster_path, imdb_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		movieLibID, "Some Film", path, 100, MatchStatusIdentified, 999, "https://example.com/poster.jpg", "",
	)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	rows, err := ListIdentifiableByLibrary(db, movieLibID)
	if err != nil {
		t.Fatalf("list rows: %v", err)
	}
	for _, r := range rows {
		if r.Path == path {
			t.Fatalf("identified movie with tmdb+poster should not be listed solely due to missing imdb_id")
		}
	}
}

func TestCountTrackedUnidentifiedByLibrary_MusicReturnsZero(t *testing.T) {
	db := newTestDB(t)
	musicLibID := createLibraryForTest(t, db, LibraryTypeMusic, "/music-unidentified-count")
	n, err := CountTrackedUnidentifiedByLibrary(db, musicLibID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 for music library, got %d", n)
	}
}

func TestCountTrackedUnidentifiedByLibrary_UnmatchedMovie(t *testing.T) {
	db := newTestDB(t)
	movieLibID := getLibraryID(t, db, "movie")
	path := "/movies/unmatched-count.mkv"
	_, err := db.Exec(
		`INSERT INTO movies (library_id, title, path, duration, match_status, tmdb_id, poster_path) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		movieLibID, "Unmatched Count", path, 100, MatchStatusUnmatched, 0, "",
	)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	n, err := CountTrackedUnidentifiedByLibrary(db, movieLibID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected at least 1 unmatched movie, got %d", n)
	}
}

func TestCountTrackedUnidentifiedByLibrary_SkipsMissingMovies(t *testing.T) {
	db := newTestDB(t)
	movieLibID := getLibraryID(t, db, "movie")
	path := "/movies/missing-unmatched.mkv"
	_, err := db.Exec(
		`INSERT INTO movies (library_id, title, path, duration, match_status, tmdb_id, poster_path, missing_since) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		movieLibID, "Gone", path, 100, MatchStatusUnmatched, 0, "", "2026-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	n, err := CountTrackedUnidentifiedByLibrary(db, movieLibID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 0 {
		t.Fatalf("missing files are hidden from library browse; unidentified count should ignore them, got %d", n)
	}
}

func TestHandleScanLibrary_SkipsMovieExtrasAndSamples(t *testing.T) {
	db := newTestDB(t)
	movieLibID := getLibraryID(t, db, "movie")

	tmp := t.TempDir()
	movieRoot := filepath.Join(tmp, "Movie (2010)")
	if err := os.MkdirAll(filepath.Join(movieRoot, "Extras"), 0o755); err != nil {
		t.Fatalf("mkdir movie tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(movieRoot, "movie.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write movie: %v", err)
	}
	if err := os.WriteFile(filepath.Join(movieRoot, "Extras", "behind-the-scenes.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write extra: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	result, err := HandleScanLibrary(context.Background(), db, movieRoot, LibraryTypeMovie, movieLibID, nil)
	if err != nil {
		t.Fatalf("scan movies: %v", err)
	}
	if result.Added != 1 || result.Skipped != 1 {
		t.Fatalf("unexpected scan result: %+v", result)
	}
}

func TestEstimateLibraryFiles_CountsSupportedMediaAndSkipsMovieExtras(t *testing.T) {
	root := t.TempDir()
	movieRoot := filepath.Join(root, "Movie (2010)")
	if err := os.MkdirAll(filepath.Join(movieRoot, "Extras"), 0o755); err != nil {
		t.Fatalf("mkdir movie tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(movieRoot, "movie.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write movie: %v", err)
	}
	if err := os.WriteFile(filepath.Join(movieRoot, "Extras", "featurette.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write extra: %v", err)
	}
	if err := os.WriteFile(filepath.Join(movieRoot, "readme.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write text: %v", err)
	}

	count, err := EstimateLibraryFiles(context.Background(), root, LibraryTypeMovie)
	if err != nil {
		t.Fatalf("estimate files: %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d", count)
	}
}

func TestHandleScanLibrary_ImportsMovieLayouts(t *testing.T) {
	db := newTestDB(t)
	movieLibID := getLibraryID(t, db, "movie")

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	cases := []struct {
		name      string
		setup     func(root string) (string, error)
		wantTitle string
	}{
		{
			name: "file in root",
			setup: func(root string) (string, error) {
				path := filepath.Join(root, "Movie (2010).mkv")
				return path, os.WriteFile(path, []byte("x"), 0o644)
			},
			wantTitle: "Movie",
		},
		{
			name: "movie in own folder",
			setup: func(root string) (string, error) {
				dir := filepath.Join(root, "Movie (2010)")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return "", err
				}
				path := filepath.Join(dir, "movie.mkv")
				return path, os.WriteFile(path, []byte("x"), 0o644)
			},
			wantTitle: "Movie",
		},
		{
			name: "collection disc layout",
			setup: func(root string) (string, error) {
				dir := filepath.Join(root, "Collection", "Movie (2010)", "Disc 1")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return "", err
				}
				path := filepath.Join(dir, "movie.mkv")
				return path, os.WriteFile(path, []byte("x"), 0o644)
			},
			wantTitle: "Movie",
		},
		{
			name: "noisy release filename in folder",
			setup: func(root string) (string, error) {
				dir := filepath.Join(root, "Die My Love (2025)")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return "", err
				}
				path := filepath.Join(dir, "Die My Love 2025 BluRay 1080p DD 5 1 x264-BHDStudio.mp4")
				return path, os.WriteFile(path, []byte("x"), 0o644)
			},
			wantTitle: "Die My Love",
		},
		{
			name: "release prefix in root filename",
			setup: func(root string) (string, error) {
				path := filepath.Join(root, "[MrManager] Riding Bean (1989) BDRemux (Dual Audio, Special Features).mkv")
				return path, os.WriteFile(path, []byte("x"), 0o644)
			},
			wantTitle: "Riding Bean",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			path, err := tc.setup(tmp)
			if err != nil {
				t.Fatalf("setup: %v", err)
			}
			result, err := HandleScanLibrary(context.Background(), db, tmp, LibraryTypeMovie, movieLibID, nil)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			if result.Added != 1 {
				t.Fatalf("unexpected scan result: %+v", result)
			}
			var title string
			if err := db.QueryRow(`SELECT title FROM movies WHERE library_id = ? AND path = ?`, movieLibID, path).Scan(&title); err != nil {
				t.Fatalf("query row: %v", err)
			}
			if title != tc.wantTitle {
				t.Fatalf("title = %q", title)
			}
			if _, err := db.Exec(`DELETE FROM media_global`); err != nil {
				t.Fatalf("clear media_global: %v", err)
			}
			if _, err := db.Exec(`DELETE FROM movies WHERE library_id = ?`, movieLibID); err != nil {
				t.Fatalf("clear movies: %v", err)
			}
		})
	}
}

func TestHandleScanLibrary_ImportsAnimeSeasonFolderWithSuffix(t *testing.T) {
	db := newTestDB(t)
	animeLibID := createLibraryForTest(t, db, LibraryTypeAnime, "/anime")

	tmp := t.TempDir()
	root := filepath.Join(tmp, "D", "Season 01 [127]")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir anime tree: %v", err)
	}
	file := filepath.Join(root, "Dragon Ball (1986) - S01E01 - Secret of the Dragon Balls [SDTV][AAC 2.0][x265].mkv")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write anime file: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	result, err := HandleScanLibrary(context.Background(), db, filepath.Join(tmp, "D"), LibraryTypeAnime, animeLibID, nil)
	if err != nil {
		t.Fatalf("scan anime: %v", err)
	}
	if result.Added != 1 {
		t.Fatalf("unexpected scan result: %+v", result)
	}

	var title string
	var season, episode int
	if err := db.QueryRow(`SELECT title, season, episode FROM anime_episodes WHERE library_id = ? AND path = ?`, animeLibID, file).Scan(&title, &season, &episode); err != nil {
		t.Fatalf("query anime row: %v", err)
	}
	if title != "Dragon Ball - S01E01" {
		t.Fatalf("title = %q", title)
	}
	if season != 1 || episode != 1 {
		t.Fatalf("season=%d episode=%d", season, episode)
	}
}

func TestHandleScanLibrary_DoesNotTreatResolutionAsSeasonInStructuredAnimeFolder(t *testing.T) {
	db := newTestDB(t)
	animeLibID := createLibraryForTest(t, db, LibraryTypeAnime, "/anime")

	tmp := t.TempDir()
	root := filepath.Join(tmp, "Anime Show", "Season 01")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir anime tree: %v", err)
	}
	file := filepath.Join(root, "[Group] Anime Show - 01 [1440x1080 x264].mkv")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write anime file: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	result, err := HandleScanLibrary(context.Background(), db, filepath.Join(tmp, "Anime Show"), LibraryTypeAnime, animeLibID, nil)
	if err != nil {
		t.Fatalf("scan anime: %v", err)
	}
	if result.Added != 1 {
		t.Fatalf("unexpected scan result: %+v", result)
	}

	var title string
	var season, episode int
	if err := db.QueryRow(`SELECT title, season, episode FROM anime_episodes WHERE library_id = ? AND path = ?`, animeLibID, file).Scan(&title, &season, &episode); err != nil {
		t.Fatalf("query anime row: %v", err)
	}
	if season != 1 || episode != 1 {
		t.Fatalf("season=%d episode=%d", season, episode)
	}
	if title != "Anime Show - S01E01" {
		t.Fatalf("title = %q", title)
	}
}

func TestHandleScanLibrary_DefaultsSeasonOneForShowFolderEpisodeTitle(t *testing.T) {
	db := newTestDB(t)
	tvLibID := getLibraryID(t, db, LibraryTypeTV)

	tmp := t.TempDir()
	root := filepath.Join(tmp, "Show")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir tv tree: %v", err)
	}
	file := filepath.Join(root, "Pilot.mkv")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write tv file: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	result, err := HandleScanLibrary(context.Background(), db, tmp, LibraryTypeTV, tvLibID, nil)
	if err != nil {
		t.Fatalf("scan tv: %v", err)
	}
	if result.Added != 1 {
		t.Fatalf("unexpected scan result: %+v", result)
	}

	var title string
	var season, episode int
	if err := db.QueryRow(`SELECT title, season, episode FROM tv_episodes WHERE library_id = ? AND path = ?`, tvLibID, file).Scan(&title, &season, &episode); err != nil {
		t.Fatalf("query tv row: %v", err)
	}
	if title != "Show" {
		t.Fatalf("title = %q", title)
	}
	if season != 1 || episode != 0 {
		t.Fatalf("season=%d episode=%d", season, episode)
	}
}

func TestHandleScanLibrary_ImportsAbsoluteEpisodeAnimeAsUnmatched(t *testing.T) {
	db := newTestDB(t)
	animeLibID := createLibraryForTest(t, db, LibraryTypeAnime, "/anime")

	tmp := t.TempDir()
	root := filepath.Join(tmp, "Frieren")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir anime tree: %v", err)
	}
	file := filepath.Join(root, "[SubsPlease] Frieren - 12 [1080p].mkv")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write anime file: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	result, err := HandleScanLibrary(context.Background(), db, root, LibraryTypeAnime, animeLibID, &funcIdentifier{})
	if err != nil {
		t.Fatalf("scan anime: %v", err)
	}
	if result.Added != 1 || result.Unmatched != 1 {
		t.Fatalf("unexpected scan result: %+v", result)
	}

	var title, status string
	var season, episode int
	if err := db.QueryRow(`SELECT title, match_status, season, episode FROM anime_episodes WHERE library_id = ? AND path = ?`, animeLibID, file).Scan(&title, &status, &season, &episode); err != nil {
		t.Fatalf("query anime row: %v", err)
	}
	if title != "Frieren - S00E00" && title != "Frieren" {
		t.Fatalf("title = %q", title)
	}
	if status != MatchStatusUnmatched {
		t.Fatalf("status = %q", status)
	}
	if season != 0 || episode != 0 {
		t.Fatalf("season=%d episode=%d", season, episode)
	}
}

func TestHandleScanLibrary_UsesAnimeIdentifier(t *testing.T) {
	db := newTestDB(t)
	animeLibID := createLibraryForTest(t, db, LibraryTypeAnime, "/anime")

	tmp := t.TempDir()
	root := filepath.Join(tmp, "Frieren", "Season 1")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir anime tree: %v", err)
	}
	file := filepath.Join(root, "Frieren - S01E12.mkv")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write anime file: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	identifier := &funcIdentifier{
		anime: func(info metadata.MediaInfo) *metadata.MatchResult {
			if info.Title != "frieren" {
				t.Fatalf("title = %q", info.Title)
			}
			if info.Season != 1 || info.Episode != 12 {
				t.Fatalf("unexpected anime info: %+v", info)
			}
			return &metadata.MatchResult{
				Title:      "Frieren - S01E12 - Episode",
				Provider:   "tmdb",
				ExternalID: "777",
			}
		},
	}

	result, err := HandleScanLibrary(context.Background(), db, filepath.Join(tmp, "Frieren"), LibraryTypeAnime, animeLibID, identifier)
	if err != nil {
		t.Fatalf("scan anime: %v", err)
	}
	if result.Added != 1 {
		t.Fatalf("unexpected scan result: %+v", result)
	}

	var title string
	var tmdbID int
	if err := db.QueryRow(`SELECT title, COALESCE(tmdb_id, 0) FROM anime_episodes WHERE library_id = ? AND path = ?`, animeLibID, file).Scan(&title, &tmdbID); err != nil {
		t.Fatalf("query anime row: %v", err)
	}
	if title != "Frieren - S01E12 - Episode" {
		t.Fatalf("title = %q", title)
	}
	if tmdbID != 777 {
		t.Fatalf("tmdb_id = %d", tmdbID)
	}
}

func TestHandleScanLibrary_ReidentifiesAnimeRowsThatOnlyHaveTVDBMetadata(t *testing.T) {
	db := newTestDB(t)
	animeLibID := createLibraryForTest(t, db, LibraryTypeAnime, "/anime")

	tmp := t.TempDir()
	root := filepath.Join(tmp, "Frieren", "Season 1")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir anime tree: %v", err)
	}
	file := filepath.Join(root, "Frieren - S01E12.mkv")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write anime file: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	first := &funcIdentifier{
		anime: func(info metadata.MediaInfo) *metadata.MatchResult {
			return &metadata.MatchResult{
				Title:      "Frieren - S01E12 - Episode",
				Provider:   "tvdb",
				ExternalID: "series-55",
			}
		},
	}
	if _, err := HandleScanLibrary(context.Background(), db, filepath.Join(tmp, "Frieren"), LibraryTypeAnime, animeLibID, first); err != nil {
		t.Fatalf("first scan anime: %v", err)
	}

	second := &funcIdentifier{
		anime: func(info metadata.MediaInfo) *metadata.MatchResult {
			return &metadata.MatchResult{
				Title:      "Frieren - S01E12 - Episode",
				Provider:   "tmdb",
				ExternalID: "777",
			}
		},
	}
	if _, err := HandleScanLibrary(context.Background(), db, filepath.Join(tmp, "Frieren"), LibraryTypeAnime, animeLibID, second); err != nil {
		t.Fatalf("second scan anime: %v", err)
	}

	var tmdbID int
	var tvdbID sql.NullString
	if err := db.QueryRow(`SELECT COALESCE(tmdb_id, 0), tvdb_id FROM anime_episodes WHERE library_id = ? AND path = ?`, animeLibID, file).Scan(&tmdbID, &tvdbID); err != nil {
		t.Fatalf("query anime row: %v", err)
	}
	if tmdbID != 777 {
		t.Fatalf("tmdb_id = %d", tmdbID)
	}
	if tvdbID.Valid {
		t.Fatalf("tvdb_id = %q", tvdbID.String)
	}
}

func TestHandleScanLibrary_StoresUnmatchedStatus(t *testing.T) {
	db := newTestDB(t)
	tvLibID := getLibraryID(t, db, "tv")

	tmp := t.TempDir()
	root := filepath.Join(tmp, "Show", "Season 1")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir tree: %v", err)
	}
	file := filepath.Join(root, "Show.S01E01.mkv")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fake media: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	result, err := HandleScanLibrary(context.Background(), db, filepath.Join(tmp, "Show"), LibraryTypeTV, tvLibID, &funcIdentifier{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if result.Unmatched != 1 {
		t.Fatalf("result = %+v", result)
	}
	var status string
	if err := db.QueryRow(`SELECT match_status FROM tv_episodes WHERE library_id = ? AND path = ?`, tvLibID, file).Scan(&status); err != nil {
		t.Fatalf("query status: %v", err)
	}
	if status != MatchStatusUnmatched {
		t.Fatalf("status = %q", status)
	}
}

func TestHandleScanLibrary_PrunesRemovedFiles(t *testing.T) {
	db := newTestDB(t)
	tvLibID := getLibraryID(t, db, "tv")

	tmp := t.TempDir()
	root := filepath.Join(tmp, "Show", "Season 1")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir tree: %v", err)
	}
	file := filepath.Join(root, "Show.S01E01.mkv")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fake media: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	if _, err := HandleScanLibrary(context.Background(), db, filepath.Join(tmp, "Show"), LibraryTypeTV, tvLibID, nil); err != nil {
		t.Fatalf("first scan: %v", err)
	}
	if err := os.Remove(file); err != nil {
		t.Fatalf("remove file: %v", err)
	}
	result, err := HandleScanLibrary(context.Background(), db, filepath.Join(tmp, "Show"), LibraryTypeTV, tvLibID, nil)
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if result.Removed != 0 {
		t.Fatalf("expected zero hard removals, got %+v", result)
	}
	var count int
	var missingSince string
	if err := db.QueryRow(`SELECT COUNT(*), COALESCE(MAX(missing_since), '') FROM tv_episodes WHERE library_id = ?`, tvLibID).Scan(&count, &missingSince); err != nil {
		t.Fatalf("query rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 soft-missing row after rescan, got %d", count)
	}
	if missingSince == "" {
		t.Fatal("expected missing_since to be set")
	}
}

func TestHandleScanLibrary_StoresFileStateAndBackfillsMissingFileHashes(t *testing.T) {
	dbConn := newTestDB(t)
	tvLibID := getLibraryID(t, dbConn, "tv")

	tmp := t.TempDir()
	showDir := filepath.Join(tmp, "Show", "Season 1")
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		t.Fatalf("mkdir show dir: %v", err)
	}
	file := filepath.Join(showDir, "Show - S01E01.mkv")
	if err := os.WriteFile(file, []byte("video-bytes"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	prevHash := computeMediaHash
	SkipFFprobeInScan = true
	hashCalls := 0
	computeMediaHash = func(_ context.Context, path string) (string, error) {
		hashCalls++
		return filepath.Base(path), nil
	}
	defer func() {
		SkipFFprobeInScan = prevSkip
		computeMediaHash = prevHash
	}()

	if _, err := HandleScanLibrary(context.Background(), dbConn, filepath.Join(tmp, "Show"), LibraryTypeTV, tvLibID, nil); err != nil {
		t.Fatalf("first scan: %v", err)
	}
	if _, err := HandleScanLibrary(context.Background(), dbConn, filepath.Join(tmp, "Show"), LibraryTypeTV, tvLibID, nil); err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if hashCalls != 1 {
		t.Fatalf("hash calls = %d, want 1", hashCalls)
	}

	if _, err := dbConn.Exec(`UPDATE tv_episodes SET file_hash = NULL, file_hash_kind = NULL WHERE library_id = ?`, tvLibID); err != nil {
		t.Fatalf("clear file hash: %v", err)
	}
	if _, err := HandleScanLibrary(context.Background(), dbConn, filepath.Join(tmp, "Show"), LibraryTypeTV, tvLibID, nil); err != nil {
		t.Fatalf("third scan: %v", err)
	}
	if hashCalls != 2 {
		t.Fatalf("hash calls after clearing file hash = %d, want 2", hashCalls)
	}

	var fileSize int64
	var fileModTime, fileHash, lastSeenAt, missingSince string
	if err := dbConn.QueryRow(`SELECT file_size_bytes, COALESCE(file_mod_time, ''), COALESCE(file_hash, ''), COALESCE(last_seen_at, ''), COALESCE(missing_since, '') FROM tv_episodes WHERE library_id = ?`, tvLibID).
		Scan(&fileSize, &fileModTime, &fileHash, &lastSeenAt, &missingSince); err != nil {
		t.Fatalf("query file state: %v", err)
	}
	if fileSize != int64(len("video-bytes")) {
		t.Fatalf("file_size_bytes = %d", fileSize)
	}
	if fileModTime == "" || lastSeenAt == "" {
		t.Fatalf("expected file timestamps, got mod=%q seen=%q", fileModTime, lastSeenAt)
	}
	if fileHash != filepath.Base(file) {
		t.Fatalf("file_hash = %q", fileHash)
	}
	if missingSince != "" {
		t.Fatalf("missing_since = %q", missingSince)
	}
}

func TestHandleScanLibraryWithOptions_DeferredRescanBackfillsStableMissingHashes(t *testing.T) {
	dbConn := newTestDB(t)
	tvLibID := getLibraryID(t, dbConn, "tv")

	tmp := t.TempDir()
	showDir := filepath.Join(tmp, "Show", "Season 1")
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		t.Fatalf("mkdir show dir: %v", err)
	}
	file := filepath.Join(showDir, "Show - S01E01.mkv")
	if err := os.WriteFile(file, []byte("video-bytes"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	prevHash := computeMediaHash
	SkipFFprobeInScan = true
	hashCalls := 0
	computeMediaHash = func(_ context.Context, path string) (string, error) {
		hashCalls++
		return filepath.Base(path), nil
	}
	defer func() {
		SkipFFprobeInScan = prevSkip
		computeMediaHash = prevHash
	}()

	if _, err := HandleScanLibraryWithOptions(context.Background(), dbConn, filepath.Join(tmp, "Show"), LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
	}); err != nil {
		t.Fatalf("first deferred scan: %v", err)
	}
	if hashCalls != 0 {
		t.Fatalf("hash calls during deferred scan = %d, want 0", hashCalls)
	}

	if _, err := HandleScanLibraryWithOptions(context.Background(), dbConn, filepath.Join(tmp, "Show"), LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
	}); err != nil {
		t.Fatalf("second deferred scan: %v", err)
	}
	if hashCalls != 1 {
		t.Fatalf("hash calls during unchanged deferred rescan = %d, want 1", hashCalls)
	}

	var fileHash string
	if err := dbConn.QueryRow(`SELECT COALESCE(file_hash, '') FROM tv_episodes WHERE library_id = ? AND path = ?`, tvLibID, file).Scan(&fileHash); err != nil {
		t.Fatalf("query deferred hash: %v", err)
	}
	if fileHash != filepath.Base(file) {
		t.Fatalf("expected deferred rescan hash %q, got %q", filepath.Base(file), fileHash)
	}

	if _, err := HandleScanLibrary(context.Background(), dbConn, filepath.Join(tmp, "Show"), LibraryTypeTV, tvLibID, nil); err != nil {
		t.Fatalf("inline backfill scan: %v", err)
	}
	if hashCalls != 1 {
		t.Fatalf("hash calls after inline backfill = %d, want 1", hashCalls)
	}

	if err := dbConn.QueryRow(`SELECT COALESCE(file_hash, '') FROM tv_episodes WHERE library_id = ? AND path = ?`, tvLibID, file).Scan(&fileHash); err != nil {
		t.Fatalf("query backfilled hash: %v", err)
	}
	if fileHash != filepath.Base(file) {
		t.Fatalf("expected backfilled hash %q, got %q", filepath.Base(file), fileHash)
	}
}

func TestScanLibraryDiscovery_TVEpisodeRelocateAfterSeasonFolderRename(t *testing.T) {
	dbConn := newTestDB(t)
	tvLibID := getLibraryID(t, dbConn, LibraryTypeTV)

	tmp := t.TempDir()
	oldDir := filepath.Join(tmp, "My Show", "Season 1 Long Title")
	if err := os.MkdirAll(oldDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	oldFile := filepath.Join(oldDir, "My Show - S01E01.mkv")
	if err := os.WriteFile(oldFile, []byte("episode-bytes"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	t.Cleanup(func() { SkipFFprobeInScan = prevSkip })

	delta, err := ScanLibraryDiscovery(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
	})
	if err != nil {
		t.Fatalf("initial discovery: %v", err)
	}
	if delta.Result.Added != 1 {
		t.Fatalf("initial added = %d, want 1", delta.Result.Added)
	}

	var refID int
	var storedPath string
	if err := dbConn.QueryRow(`SELECT id, path FROM tv_episodes WHERE library_id = ? AND season = 1 AND episode = 1`, tvLibID).Scan(&refID, &storedPath); err != nil {
		t.Fatalf("select episode: %v", err)
	}
	if storedPath != oldFile {
		t.Fatalf("stored path = %q, want %q", storedPath, oldFile)
	}

	newDir := filepath.Join(tmp, "My Show", "Season 1")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatalf("mkdir new season: %v", err)
	}
	newFile := filepath.Join(newDir, "My Show - S01E01.mkv")
	if err := os.WriteFile(newFile, []byte("episode-bytes"), 0o644); err != nil {
		t.Fatalf("write new file: %v", err)
	}
	if err := os.RemoveAll(oldDir); err != nil {
		t.Fatalf("remove old season dir: %v", err)
	}

	// Subpath is the series folder (same scope widened automated scans use after a rename).
	delta, err = ScanLibraryDiscovery(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
		Subpaths: []string{filepath.Join("My Show")},
	})
	if err != nil {
		t.Fatalf("second discovery: %v", err)
	}
	if delta.Result.Added != 0 {
		t.Fatalf("relocate should not insert duplicate; added = %d", delta.Result.Added)
	}
	if delta.Result.Updated != 1 {
		t.Fatalf("updated = %d, want 1", delta.Result.Updated)
	}

	if err := dbConn.QueryRow(`SELECT path FROM tv_episodes WHERE id = ?`, refID).Scan(&storedPath); err != nil {
		t.Fatalf("select path after relocate: %v", err)
	}
	if storedPath != newFile {
		t.Fatalf("path after relocate = %q, want %q", storedPath, newFile)
	}

	var count int
	if err := dbConn.QueryRow(`SELECT COUNT(*) FROM tv_episodes WHERE library_id = ? AND season = 1 AND episode = 1`, tvLibID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("episode rows = %d, want 1", count)
	}
}

// Regression: a missing S01E01 for Show A must not be treated as a relocation target when scanning
// S01E01 for Show B under the same library (same season/episode, different show folder).
func TestScanLibraryDiscovery_TVEpisodeRelocateDoesNotMatchOtherShow(t *testing.T) {
	dbConn := newTestDB(t)
	tvLibID := getLibraryID(t, dbConn, LibraryTypeTV)

	tmp := t.TempDir()
	showAFile := filepath.Join(tmp, "ShowA", "Season 1", "ShowA.S01E01.mkv")
	if err := os.MkdirAll(filepath.Dir(showAFile), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(showAFile, []byte("a"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	t.Cleanup(func() { SkipFFprobeInScan = prevSkip })

	if _, err := ScanLibraryDiscovery(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
	}); err != nil {
		t.Fatalf("initial discovery: %v", err)
	}
	var showAID int
	if err := dbConn.QueryRow(`SELECT id FROM tv_episodes WHERE library_id = ? AND path = ?`, tvLibID, showAFile).Scan(&showAID); err != nil {
		t.Fatalf("select show A row: %v", err)
	}

	if err := os.Remove(showAFile); err != nil {
		t.Fatalf("remove show A file: %v", err)
	}
	if _, err := ScanLibraryDiscovery(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
	}); err != nil {
		t.Fatalf("rescan after remove: %v", err)
	}
	var missingSince string
	if err := dbConn.QueryRow(`SELECT COALESCE(missing_since, '') FROM tv_episodes WHERE id = ?`, showAID).Scan(&missingSince); err != nil {
		t.Fatalf("missing_since: %v", err)
	}
	if missingSince == "" {
		t.Fatal("expected show A episode to be marked missing")
	}

	showBFile := filepath.Join(tmp, "ShowB", "Season 1", "ShowB.S01E01.mkv")
	if err := os.MkdirAll(filepath.Dir(showBFile), 0o755); err != nil {
		t.Fatalf("mkdir show b: %v", err)
	}
	if err := os.WriteFile(showBFile, []byte("b"), 0o644); err != nil {
		t.Fatalf("write show b file: %v", err)
	}
	delta, err := ScanLibraryDiscovery(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
	})
	if err != nil {
		t.Fatalf("discovery show b: %v", err)
	}
	if delta.Result.Added != 1 {
		t.Fatalf("expected new row for show B; added = %d", delta.Result.Added)
	}

	var count int
	if err := dbConn.QueryRow(`SELECT COUNT(*) FROM tv_episodes WHERE library_id = ? AND season = 1 AND episode = 1`, tvLibID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("episode rows = %d, want 2 (show A missing + show B present)", count)
	}

	var pathAfter string
	if err := dbConn.QueryRow(`SELECT path FROM tv_episodes WHERE id = ?`, showAID).Scan(&pathAfter); err != nil {
		t.Fatalf("select show A after: %v", err)
	}
	if pathAfter != showAFile {
		t.Fatalf("show A row path was rewritten to %q; want unchanged %q", pathAfter, showAFile)
	}
}

func TestScanLibraryDiscovery_TargetedEnrichmentTouchesOnlyChangedFiles(t *testing.T) {
	dbConn := newTestDB(t)
	tvLibID := getLibraryID(t, dbConn, "tv")

	tmp := t.TempDir()
	showDir := filepath.Join(tmp, "Show", "Season 1")
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		t.Fatalf("mkdir show dir: %v", err)
	}
	firstFile := filepath.Join(showDir, "Show - S01E01.mkv")
	secondFile := filepath.Join(showDir, "Show - S01E02.mkv")
	for _, file := range []string{firstFile, secondFile} {
		if err := os.WriteFile(file, []byte("video-bytes"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	prevHash := computeMediaHash
	prevVideoProbe := readVideoMetadata
	computeMediaHash = func(_ context.Context, path string) (string, error) {
		return filepath.Base(path), nil
	}
	readVideoMetadata = func(_ context.Context, path string) (VideoProbeResult, error) {
		return VideoProbeResult{
			Duration: 42,
			EmbeddedSubtitles: []EmbeddedSubtitle{{
				StreamIndex: 2,
				Language:    "en",
				Title:       filepath.Base(path) + " subtitle",
			}},
			EmbeddedAudioTracks: []EmbeddedAudioTrack{{
				StreamIndex: 1,
				Language:    "en",
				Title:       filepath.Base(path) + " audio",
			}},
		}, nil
	}
	t.Cleanup(func() {
		computeMediaHash = prevHash
		readVideoMetadata = prevVideoProbe
	})

	delta, err := ScanLibraryDiscovery(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
	})
	if err != nil {
		t.Fatalf("initial discovery: %v", err)
	}
	if delta.Result.Added != 2 {
		t.Fatalf("initial added = %d, want 2", delta.Result.Added)
	}
	if len(delta.TouchedFiles) != 2 {
		t.Fatalf("initial touched files = %d, want 2", len(delta.TouchedFiles))
	}
	if err := EnrichLibraryTasks(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, delta.TouchedFiles, ScanOptions{
		ProbeMedia:             true,
		ProbeEmbeddedSubtitles: true,
	}); err != nil {
		t.Fatalf("initial enrichment: %v", err)
	}

	updatedAt := time.Now().Add(2 * time.Second)
	if err := os.WriteFile(firstFile, []byte("video-bytes-updated"), 0o644); err != nil {
		t.Fatalf("rewrite first file: %v", err)
	}
	if err := os.Chtimes(firstFile, updatedAt, updatedAt); err != nil {
		t.Fatalf("chtimes first file: %v", err)
	}

	delta, err = ScanLibraryDiscovery(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
	})
	if err != nil {
		t.Fatalf("second discovery: %v", err)
	}
	if delta.Result.Updated != 2 {
		t.Fatalf("second updated = %d, want 2", delta.Result.Updated)
	}
	if len(delta.TouchedFiles) != 1 {
		t.Fatalf("second touched files = %d, want 1", len(delta.TouchedFiles))
	}
	if delta.TouchedFiles[0].Path != firstFile {
		t.Fatalf("second touched path = %q, want %q", delta.TouchedFiles[0].Path, firstFile)
	}

	hashCalls := map[string]int{}
	probeCalls := map[string]int{}
	computeMediaHash = func(_ context.Context, path string) (string, error) {
		hashCalls[path]++
		return filepath.Base(path), nil
	}
	readVideoMetadata = func(_ context.Context, path string) (VideoProbeResult, error) {
		probeCalls[path]++
		return VideoProbeResult{
			Duration:            84,
			EmbeddedSubtitles:   []EmbeddedSubtitle{{StreamIndex: 3, Language: "en", Title: "subtitle"}},
			EmbeddedAudioTracks: []EmbeddedAudioTrack{{StreamIndex: 1, Language: "en", Title: "audio"}},
		}, nil
	}

	if err := EnrichLibraryTasks(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, delta.TouchedFiles, ScanOptions{
		ProbeMedia:             true,
		ProbeEmbeddedSubtitles: true,
	}); err != nil {
		t.Fatalf("targeted enrichment: %v", err)
	}
	if hashCalls[firstFile] != 1 || hashCalls[secondFile] != 0 {
		t.Fatalf("hash calls = %#v", hashCalls)
	}
	if probeCalls[firstFile] != 1 || probeCalls[secondFile] != 0 {
		t.Fatalf("probe calls = %#v", probeCalls)
	}

	var duration int
	if err := dbConn.QueryRow(`SELECT duration FROM tv_episodes WHERE library_id = ? AND path = ?`, tvLibID, firstFile).Scan(&duration); err != nil {
		t.Fatalf("query duration: %v", err)
	}
	if duration != 84 {
		t.Fatalf("duration = %d, want 84", duration)
	}
	var subtitleCount, audioCount int
	if err := dbConn.QueryRow(`SELECT COUNT(1) FROM embedded_subtitles`).Scan(&subtitleCount); err != nil {
		t.Fatalf("count subtitles: %v", err)
	}
	if err := dbConn.QueryRow(`SELECT COUNT(1) FROM embedded_audio_tracks`).Scan(&audioCount); err != nil {
		t.Fatalf("count audio tracks: %v", err)
	}
	if subtitleCount == 0 || audioCount == 0 {
		t.Fatalf("expected embedded streams to be stored, got subtitles=%d audio=%d", subtitleCount, audioCount)
	}
}

func TestScanLibraryDiscovery_UnchangedQueuesIntroProbeUntilProbed(t *testing.T) {
	dbConn := newTestDB(t)
	tvLibID := getLibraryID(t, dbConn, "tv")

	tmp := t.TempDir()
	showDir := filepath.Join(tmp, "Show", "Season 1")
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		t.Fatalf("mkdir show dir: %v", err)
	}
	episodeFile := filepath.Join(showDir, "Show - S01E01.mkv")
	if err := os.WriteFile(episodeFile, []byte("video-bytes"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	prevHash := computeMediaHash
	computeMediaHash = func(_ context.Context, path string) (string, error) {
		return filepath.Base(path), nil
	}
	t.Cleanup(func() { computeMediaHash = prevHash })

	delta1, err := ScanLibraryDiscovery(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
	})
	if err != nil {
		t.Fatalf("first discovery: %v", err)
	}
	if len(delta1.TouchedFiles) != 1 {
		t.Fatalf("first touched = %d, want 1", len(delta1.TouchedFiles))
	}

	delta2, err := ScanLibraryDiscovery(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
	})
	if err != nil {
		t.Fatalf("second discovery: %v", err)
	}
	if len(delta2.TouchedFiles) != 1 {
		t.Fatalf("second scan should queue intro backfill for unchanged file; touched = %d, want 1", len(delta2.TouchedFiles))
	}
	if delta2.TouchedFiles[0].Path != episodeFile {
		t.Fatalf("second touched path = %q, want %q", delta2.TouchedFiles[0].Path, episodeFile)
	}

	prevVideoProbe := readVideoMetadata
	readVideoMetadata = func(_ context.Context, path string) (VideoProbeResult, error) {
		return VideoProbeResult{Duration: 42}, nil
	}
	t.Cleanup(func() { readVideoMetadata = prevVideoProbe })

	if err := EnrichLibraryTasks(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, delta2.TouchedFiles, ScanOptions{
		ProbeMedia:             true,
		ProbeEmbeddedSubtitles: false,
	}); err != nil {
		t.Fatalf("enrichment: %v", err)
	}

	delta3, err := ScanLibraryDiscovery(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
	})
	if err != nil {
		t.Fatalf("third discovery: %v", err)
	}
	if len(delta3.TouchedFiles) != 0 {
		t.Fatalf("after intro probe, unchanged file should not re-queue; touched = %d, want 0", len(delta3.TouchedFiles))
	}
}

func TestHandleScanLibrary_PartialScanMarksOnlyScopedRowsMissing(t *testing.T) {
	dbConn := newTestDB(t)
	tvLibID := getLibraryID(t, dbConn, "tv")

	tmp := t.TempDir()
	showARoot := filepath.Join(tmp, "Show A", "Season 1")
	showBRoot := filepath.Join(tmp, "Show B", "Season 1")
	for _, dir := range []string{showARoot, showBRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir dir: %v", err)
		}
	}
	showAFile := filepath.Join(showARoot, "Show A - S01E01.mkv")
	showBFile := filepath.Join(showBRoot, "Show B - S01E01.mkv")
	for _, path := range []string{showAFile, showBFile} {
		if err := os.WriteFile(path, []byte("video"), 0o644); err != nil {
			t.Fatalf("write media file: %v", err)
		}
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	if _, err := HandleScanLibrary(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, nil); err != nil {
		t.Fatalf("full scan: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(tmp, "Show A")); err != nil {
		t.Fatalf("remove show A tree: %v", err)
	}

	if _, err := HandleScanLibraryWithOptions(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, ScanOptions{
		ProbeMedia:             true,
		ProbeEmbeddedSubtitles: true,
		ScanSidecarSubtitles:   true,
		Subpaths:               []string{"Show A"},
	}); err != nil {
		t.Fatalf("partial scan: %v", err)
	}

	var missingA, missingB string
	if err := dbConn.QueryRow(`SELECT COALESCE(missing_since, '') FROM tv_episodes WHERE path = ?`, showAFile).Scan(&missingA); err != nil {
		t.Fatalf("query show A: %v", err)
	}
	if err := dbConn.QueryRow(`SELECT COALESCE(missing_since, '') FROM tv_episodes WHERE path = ?`, showBFile).Scan(&missingB); err != nil {
		t.Fatalf("query show B: %v", err)
	}
	if missingA == "" {
		t.Fatal("expected scoped row to be marked missing")
	}
	if missingB != "" {
		t.Fatalf("expected sibling row to stay present, got missing_since=%q", missingB)
	}
}

func TestGetMediaByLibraryID_ExposesDuplicateStateAndFiltersMissingRows(t *testing.T) {
	dbConn := newTestDB(t)
	movieLibID := getLibraryID(t, dbConn, "movie")

	tmp := t.TempDir()
	fileA := filepath.Join(tmp, "Movie A (2024).mkv")
	fileB := filepath.Join(tmp, "Movie B (2024).mkv")
	for _, path := range []string{fileA, fileB} {
		if err := os.WriteFile(path, []byte("same-bytes"), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	if _, err := HandleScanLibrary(context.Background(), dbConn, tmp, LibraryTypeMovie, movieLibID, nil); err != nil {
		t.Fatalf("initial scan: %v", err)
	}
	items, err := GetMediaByLibraryID(dbConn, movieLibID)
	if err != nil {
		t.Fatalf("get media by library: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v", items)
	}
	for _, item := range items {
		if !item.Duplicate || item.DuplicateCount != 2 {
			t.Fatalf("expected duplicate state on %+v", item)
		}
	}

	if err := os.Remove(fileA); err != nil {
		t.Fatalf("remove fileA: %v", err)
	}
	if _, err := HandleScanLibrary(context.Background(), dbConn, tmp, LibraryTypeMovie, movieLibID, nil); err != nil {
		t.Fatalf("rescan: %v", err)
	}
	items, err = GetMediaByLibraryID(dbConn, movieLibID)
	if err != nil {
		t.Fatalf("get media by library after rescan: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items after rescan = %+v", items)
	}
	if items[0].Path != fileB {
		t.Fatalf("remaining item = %+v", items[0])
	}
	if items[0].Missing {
		t.Fatalf("expected remaining item to be present: %+v", items[0])
	}
	if items[0].Duplicate {
		t.Fatalf("duplicateCount = %+v", items[0])
	}
}

func TestQueryMediaByLibraryID_EpisodeShowPosterUsesLinkedShow(t *testing.T) {
	dbConn := newTestDB(t)
	libraryID := getLibraryID(t, dbConn, "tv")
	now := time.Now().UTC().Format(time.RFC3339)

	var showID int
	if err := dbConn.QueryRow(`INSERT INTO shows (
library_id, kind, tmdb_id, title, title_key, poster_path, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID, LibraryTypeTV, 321, "Slow Horses", "slowhorses", "/show-poster.jpg", now, now,
	).Scan(&showID); err != nil {
		t.Fatalf("insert show: %v", err)
	}
	var seasonID int
	if err := dbConn.QueryRow(`INSERT INTO seasons (
show_id, season_number, title, created_at, updated_at
) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		showID, 1, "Season 1", now, now,
	).Scan(&seasonID); err != nil {
		t.Fatalf("insert season: %v", err)
	}
	var episodeID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (
library_id, title, path, duration, match_status, tmdb_id, show_id, season_id, season, episode
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID, "Slow Horses - S01E01 - Pilot", "/tv/Slow Horses/S01E01.mkv", 1800, MatchStatusIdentified, 321, showID, seasonID, 1, 1,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, LibraryTypeTV, episodeID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}

	items, _, err := queryMediaByLibraryID(dbConn, libraryID, LibraryTypeTV, 0, 0)
	if err != nil {
		t.Fatalf("query media: %v", err)
	}
	if got := items[0].ShowPosterPath; got != "/show-poster.jpg" {
		t.Fatalf("show poster path = %q", got)
	}
	if got := items[0].ShowTitle; got != "Slow Horses" {
		t.Fatalf("show title = %q", got)
	}
}

func TestQueryMediaByLibraryID_EpisodeShowPosterFallsBackByTMDBID(t *testing.T) {
	dbConn := newTestDB(t)
	libraryID := getLibraryID(t, dbConn, "tv")
	now := time.Now().UTC().Format(time.RFC3339)

	if _, err := dbConn.Exec(`INSERT INTO shows (
library_id, kind, tmdb_id, title, title_key, poster_path, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		libraryID, LibraryTypeTV, 654, "Andor", "andor", "/andor-show.jpg", now, now,
	); err != nil {
		t.Fatalf("insert show: %v", err)
	}
	var episodeID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (
library_id, title, path, duration, match_status, tmdb_id, season, episode
) VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID, "Andor - S01E01 - Kassa", "/tv/Andor/S01E01.mkv", 1800, MatchStatusIdentified, 654, 1, 1,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, LibraryTypeTV, episodeID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}

	items, _, err := queryMediaByLibraryID(dbConn, libraryID, LibraryTypeTV, 0, 0)
	if err != nil {
		t.Fatalf("query media: %v", err)
	}
	if got := items[0].ShowPosterPath; got != "/andor-show.jpg" {
		t.Fatalf("show poster path = %q", got)
	}
	if got := items[0].ShowTitle; got != "Andor" {
		t.Fatalf("show title = %q", got)
	}
}

func TestQueryMediaByLibraryID_EpisodeShowPosterFallsBackByTitleKey(t *testing.T) {
	dbConn := newTestDB(t)
	libraryID := getLibraryID(t, dbConn, "tv")
	now := time.Now().UTC().Format(time.RFC3339)

	if _, err := dbConn.Exec(`INSERT INTO shows (
library_id, kind, title, title_key, poster_path, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		libraryID, LibraryTypeTV, "Battlestar Galactica (2004)", "battlestargalactica2004", "/bsg-2004.jpg", now, now,
	); err != nil {
		t.Fatalf("insert show: %v", err)
	}
	var episodeID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (
library_id, title, path, duration, match_status, season, episode
) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID, "Battlestar Galactica (2004) - S01E01 - 33", "/tv/Battlestar Galactica (2004)/S01E01.mkv", 1800, MatchStatusLocal, 1, 1,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, LibraryTypeTV, episodeID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}

	items, _, err := queryMediaByLibraryID(dbConn, libraryID, LibraryTypeTV, 0, 0)
	if err != nil {
		t.Fatalf("query media: %v", err)
	}
	if got := items[0].ShowPosterPath; got != "/bsg-2004.jpg" {
		t.Fatalf("show poster path = %q", got)
	}
	if got := items[0].ShowTitle; got != "Battlestar Galactica (2004)" {
		t.Fatalf("show title = %q", got)
	}
}

func TestGetLibraryShowEpisodesForUser(t *testing.T) {
	dbConn := newTestDB(t)
	libraryID := getLibraryID(t, dbConn, "tv")
	now := time.Now().UTC().Format(time.RFC3339)

	var showID int
	if err := dbConn.QueryRow(`INSERT INTO shows (
library_id, kind, tmdb_id, title, title_key, poster_path, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID, LibraryTypeTV, 321, "Slow Horses", "slowhorses", "/show-poster.jpg", now, now,
	).Scan(&showID); err != nil {
		t.Fatalf("insert show: %v", err)
	}
	var seasonID int
	if err := dbConn.QueryRow(`INSERT INTO seasons (
show_id, season_number, title, created_at, updated_at
) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		showID, 1, "Season 1", now, now,
	).Scan(&seasonID); err != nil {
		t.Fatalf("insert season: %v", err)
	}
	var episodeID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (
library_id, title, path, duration, match_status, tmdb_id, show_id, season_id, season, episode
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID, "Slow Horses - S01E01 - Pilot", "/tv/Slow Horses/S01E01.mkv", 1800, MatchStatusIdentified, 321, showID, seasonID, 1, 1,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, LibraryTypeTV, episodeID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}
	for _, episode := range []struct {
		title string
		path  string
	}{
		{title: "Slow Horses - Special Feature", path: "/tv/Slow Horses/S00E00-feature.mkv"},
		{title: "Slow Horses - Behind the Scenes", path: "/tv/Slow Horses/S00E00-bts.mkv"},
	} {
		var extraEpisodeID int
		if err := dbConn.QueryRow(`INSERT INTO tv_episodes (
library_id, title, path, duration, match_status, tmdb_id, show_id, season_id, season, episode
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			libraryID, episode.title, episode.path, 900, MatchStatusIdentified, 321, showID, seasonID, 1, 0,
		).Scan(&extraEpisodeID); err != nil {
			t.Fatalf("insert extra episode: %v", err)
		}
		if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, LibraryTypeTV, extraEpisodeID); err != nil {
			t.Fatalf("insert extra media_global: %v", err)
		}
	}

	var userID int
	if err := dbConn.QueryRow(`SELECT user_id FROM libraries WHERE id = ?`, libraryID).Scan(&userID); err != nil {
		t.Fatalf("library user: %v", err)
	}

	items, err := GetLibraryShowEpisodesForUser(dbConn, libraryID, userID, "tmdb-321")
	if err != nil {
		t.Fatalf("GetLibraryShowEpisodesForUser: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 episodes, got %d", len(items))
	}
	if items[0].Title != "Slow Horses - Behind the Scenes" || items[1].Title != "Slow Horses - Special Feature" || items[2].Title != "Slow Horses - S01E01 - Pilot" {
		t.Fatalf("unexpected order: %#v", []string{items[0].Title, items[1].Title, items[2].Title})
	}

	_, err = GetLibraryShowEpisodesForUser(dbConn, libraryID, userID, "tmdb-999999")
	if !errors.Is(err, ErrShowNotFound) {
		t.Fatalf("expected ErrShowNotFound, got %v", err)
	}
}

func TestGetMediaByLibraryID_RelinksMissingEpisodeShowAndSeason(t *testing.T) {
	dbConn := newTestDB(t)
	libraryID := getLibraryID(t, dbConn, "tv")

	var episodeID int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (
library_id, title, path, duration, match_status, tmdb_id, overview, poster_path, release_date, imdb_id, imdb_rating, season, episode
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID, "Severance - S01E01 - Good News About Hell", "/tv/Severance/S01E01.mkv", 1800, MatchStatusIdentified, 777, "overview", "/episode-poster.jpg", "2022-02-18", "tt11280740", 8.7, 1, 1,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert episode: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, LibraryTypeTV, episodeID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}

	items, err := GetMediaByLibraryID(dbConn, libraryID)
	if err != nil {
		t.Fatalf("get media by library: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}

	var showID, seasonID int
	if err := dbConn.QueryRow(`SELECT COALESCE(show_id, 0), COALESCE(season_id, 0) FROM tv_episodes WHERE id = ?`, episodeID).Scan(&showID, &seasonID); err != nil {
		t.Fatalf("query episode links: %v", err)
	}
	if showID == 0 || seasonID == 0 {
		t.Fatalf("expected relinked show/season, got show=%d season=%d", showID, seasonID)
	}

	var showPoster string
	if err := dbConn.QueryRow(`SELECT COALESCE(poster_path, '') FROM shows WHERE id = ?`, showID).Scan(&showPoster); err != nil {
		t.Fatalf("query show poster: %v", err)
	}
	if showPoster != "/episode-poster.jpg" {
		t.Fatalf("show poster = %q", showPoster)
	}
}

func TestHandleScanLibrary_ImportsMusicExtensionsAndTags(t *testing.T) {
	db := newTestDB(t)
	musicLibID := createLibraryForTest(t, db, LibraryTypeMusic, "/music")

	tmp := t.TempDir()
	root := filepath.Join(tmp, "Artist", "Album", "Disc 2")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir music tree: %v", err)
	}
	file := filepath.Join(root, "01 - Placeholder.flac")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fake audio: %v", err)
	}

	prevSkip := SkipFFprobeInScan
	prevReadAudio := readAudioMetadata
	SkipFFprobeInScan = false
	readAudioMetadata = func(_ context.Context, _ string) (metadata.MusicMetadata, int, error) {
		return metadata.MusicMetadata{
			Title:       "Tagged Track",
			Artist:      "Tagged Artist",
			Album:       "Tagged Album",
			AlbumArtist: "Tagged Album Artist",
			DiscNumber:  2,
			TrackNumber: 1,
			ReleaseYear: 2024,
		}, 245, nil
	}
	defer func() {
		SkipFFprobeInScan = prevSkip
		readAudioMetadata = prevReadAudio
	}()

	result, err := HandleScanLibrary(context.Background(), db, filepath.Join(tmp, "Artist"), LibraryTypeMusic, musicLibID, nil)
	if err != nil {
		t.Fatalf("scan music: %v", err)
	}
	if result.Added != 1 {
		t.Fatalf("unexpected scan result: %+v", result)
	}

	var title, artist, album, albumArtist, status string
	var duration, discNumber, trackNumber, releaseYear int
	if err := db.QueryRow(`SELECT title, artist, album, album_artist, duration, disc_number, track_number, release_year, match_status FROM music_tracks WHERE library_id = ?`, musicLibID).
		Scan(&title, &artist, &album, &albumArtist, &duration, &discNumber, &trackNumber, &releaseYear, &status); err != nil {
		t.Fatalf("query music row: %v", err)
	}
	if title != "Tagged Track" || artist != "Tagged Artist" || album != "Tagged Album" || albumArtist != "Tagged Album Artist" {
		t.Fatalf("unexpected music metadata: title=%q artist=%q album=%q albumArtist=%q", title, artist, album, albumArtist)
	}
	if duration != 245 || discNumber != 2 || trackNumber != 1 || releaseYear != 2024 {
		t.Fatalf("unexpected numeric metadata: duration=%d disc=%d track=%d year=%d", duration, discNumber, trackNumber, releaseYear)
	}
	if status != MatchStatusLocal {
		t.Fatalf("status = %q", status)
	}
}

func TestHandleScanLibrary_ImportsSupportedMusicExtensions(t *testing.T) {
	db := newTestDB(t)
	musicLibID := createLibraryForTest(t, db, LibraryTypeMusic, "/music")

	tmp := t.TempDir()
	root := filepath.Join(tmp, "Artist", "Album")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir music tree: %v", err)
	}
	extensions := []string{".mp3", ".flac", ".m4a", ".aac", ".ogg", ".opus", ".wav", ".alac"}
	for i, ext := range extensions {
		path := filepath.Join(root, fmt.Sprintf("Track-%d%s", i+1, ext))
		if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", ext, err)
		}
	}

	prevSkip := SkipFFprobeInScan
	SkipFFprobeInScan = true
	defer func() { SkipFFprobeInScan = prevSkip }()

	result, err := HandleScanLibrary(context.Background(), db, filepath.Join(tmp, "Artist"), LibraryTypeMusic, musicLibID, nil)
	if err != nil {
		t.Fatalf("scan music: %v", err)
	}
	if result.Added != len(extensions) {
		t.Fatalf("expected %d imported tracks, got %+v", len(extensions), result)
	}
}
