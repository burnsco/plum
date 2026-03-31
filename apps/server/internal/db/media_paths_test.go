package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveMediaSourcePath_ResolvesRelativePathAgainstLibraryRoot(t *testing.T) {
	dbConn, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	root := t.TempDir()
	episodePath := filepath.Join(root, "Dragonball", "Season 01", "episode.mkv")
	if err := os.MkdirAll(filepath.Dir(episodePath), 0o755); err != nil {
		t.Fatalf("mkdir media dir: %v", err)
	}
	if err := os.WriteFile(episodePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media file: %v", err)
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
		"Anime",
		LibraryTypeAnime,
		root,
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	resolved, err := ResolveMediaSourcePath(dbConn, MediaItem{
		LibraryID: libraryID,
		Path:      filepath.Join("Dragonball", "Season 01", "episode.mkv"),
	})
	if err != nil {
		t.Fatalf("ResolveMediaSourcePath: %v", err)
	}
	if resolved != episodePath {
		t.Fatalf("resolved path = %q, want %q", resolved, episodePath)
	}
}

func TestEnsurePlaybackTrackMetadata_PreservesResolvedPath(t *testing.T) {
	dbConn, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	root := t.TempDir()
	episodePath := filepath.Join(root, "Dragonball", "Season 01", "episode.mkv")
	if err := os.MkdirAll(filepath.Dir(episodePath), 0o755); err != nil {
		t.Fatalf("mkdir media dir: %v", err)
	}
	if err := os.WriteFile(episodePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write media file: %v", err)
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
		"Anime",
		LibraryTypeAnime,
		root,
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	var episodeID int
	if err := dbConn.QueryRow(
		`INSERT INTO anime_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Dragon Ball - S01E11",
		filepath.Join("Dragonball", "Season 01", "episode.mkv"),
		0,
		MatchStatusLocal,
		1,
		11,
	).Scan(&episodeID); err != nil {
		t.Fatalf("insert episode: %v", err)
	}

	var globalID int
	if err := dbConn.QueryRow(
		`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`,
		LibraryTypeAnime,
		episodeID,
	).Scan(&globalID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}

	resolvedPath, err := ResolveMediaSourcePath(dbConn, MediaItem{
		ID:        globalID,
		LibraryID: libraryID,
		Path:      filepath.Join("Dragonball", "Season 01", "episode.mkv"),
	})
	if err != nil {
		t.Fatalf("ResolveMediaSourcePath: %v", err)
	}

	item := MediaItem{
		ID:        globalID,
		LibraryID: libraryID,
		Path:      resolvedPath,
	}
	if err := EnsurePlaybackTrackMetadata(context.Background(), dbConn, &item); err != nil {
		t.Fatalf("EnsurePlaybackTrackMetadata: %v", err)
	}
	if item.Path != resolvedPath {
		t.Fatalf("path = %q, want %q", item.Path, resolvedPath)
	}
}
