package db

import (
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
