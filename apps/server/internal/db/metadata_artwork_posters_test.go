package db

import (
	"database/sql"
	"testing"
	"time"
)

func TestSetMoviePosterSelectionUpdatesMoviesRow(t *testing.T) {
	dbConn := newTestDB(t)
	defer dbConn.Close()

	libID := getLibraryID(t, dbConn, "movie")
	var movieID int
	if err := dbConn.QueryRow(
		`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, 0, ?) RETURNING id`,
		libID, "Test Film", "/movies/test.mkv", MatchStatusIdentified,
	).Scan(&movieID); err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES ('movie', ?)`, movieID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}

	source := "https://image.tmdb.org/t/p/w500/example.jpg"
	if err := SetMoviePosterSelection(dbConn, movieID, source, true); err != nil {
		t.Fatalf("SetMoviePosterSelection: %v", err)
	}

	var posterPath string
	var locked int
	var updatedAt sql.NullString
	err := dbConn.QueryRow(
		`SELECT poster_path, poster_locked, updated_at FROM movies WHERE id = ?`,
		movieID,
	).Scan(&posterPath, &locked, &updatedAt)
	if err != nil {
		t.Fatalf("select movie: %v", err)
	}
	if posterPath != source {
		t.Fatalf("poster_path = %q, want %q", posterPath, source)
	}
	if locked != 1 {
		t.Fatalf("poster_locked = %d, want 1", locked)
	}
	if !updatedAt.Valid || updatedAt.String == "" {
		t.Fatalf("updated_at should be set, got valid=%v string=%q", updatedAt.Valid, updatedAt.String)
	}
	if _, err := time.Parse(time.RFC3339, updatedAt.String); err != nil {
		t.Fatalf("updated_at not RFC3339: %q: %v", updatedAt.String, err)
	}
}
