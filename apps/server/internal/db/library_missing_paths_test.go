package db

import (
	"context"
	"testing"
)

func TestMarkMediaMissingForFilesystemPaths_MovieExactPath(t *testing.T) {
	dbConn := newTestDB(t)
	defer dbConn.Close()

	movieLibID := getLibraryID(t, dbConn, LibraryTypeMovie)
	path := "/movies/deleted.mkv"

	var refID int
	if err := dbConn.QueryRow(
		`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		movieLibID, "Deleted", path, 3600, MatchStatusLocal,
	).Scan(&refID); err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, LibraryTypeMovie, refID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}

	n, err := MarkMediaMissingForFilesystemPaths(context.Background(), dbConn, movieLibID, "/movies", []string{path})
	if err != nil {
		t.Fatalf("MarkMediaMissingForFilesystemPaths: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 row marked, got %d", n)
	}

	var missing string
	if err := dbConn.QueryRow(`SELECT COALESCE(missing_since, '') FROM movies WHERE id = ?`, refID).Scan(&missing); err != nil {
		t.Fatalf("select missing_since: %v", err)
	}
	if missing == "" {
		t.Fatal("expected missing_since to be set")
	}
}

func TestMarkMediaMissingForFilesystemPaths_ShowPrefix(t *testing.T) {
	dbConn := newTestDB(t)
	defer dbConn.Close()

	tvLibID := getLibraryID(t, dbConn, LibraryTypeTV)
	root := "/tv"
	prefix := root + "/Gone Show"

	paths := []string{
		prefix + "/S01E01.mkv",
		prefix + "/S02/S02E01.mkv",
	}
	var refIDs []int
	for _, p := range paths {
		var refID int
		if err := dbConn.QueryRow(
			`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			tvLibID, "Ep", p, 1800, MatchStatusLocal, 1, 1,
		).Scan(&refID); err != nil {
			t.Fatalf("insert episode: %v", err)
		}
		if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, LibraryTypeTV, refID); err != nil {
			t.Fatalf("insert media_global: %v", err)
		}
		refIDs = append(refIDs, refID)
	}

	n, err := MarkMediaMissingForFilesystemPaths(context.Background(), dbConn, tvLibID, root, []string{prefix})
	if err != nil {
		t.Fatalf("MarkMediaMissingForFilesystemPaths: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 rows marked, got %d", n)
	}

	for _, id := range refIDs {
		var missing string
		if err := dbConn.QueryRow(`SELECT COALESCE(missing_since, '') FROM tv_episodes WHERE id = ?`, id).Scan(&missing); err != nil {
			t.Fatalf("select missing_since: %v", err)
		}
		if missing == "" {
			t.Fatalf("expected missing_since for ref id %d", id)
		}
	}
}
