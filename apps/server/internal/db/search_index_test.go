package db

import (
	"database/sql"
	"fmt"
	"testing"
	"time"
)

func testUserID(t *testing.T, dbConn *sql.DB) int {
	t.Helper()
	var userID int
	if err := dbConn.QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&userID); err != nil {
		t.Fatalf("get test user id: %v", err)
	}
	return userID
}

func insertSearchDocumentForTest(t *testing.T, dbConn *sql.DB, libraryID int, docKey string, title string) {
	t.Helper()
	var libraryName, libraryType string
	if err := dbConn.QueryRow(`SELECT name, type FROM libraries WHERE id = ?`, libraryID).Scan(&libraryName, &libraryType); err != nil {
		t.Fatalf("get library metadata: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	href := fmt.Sprintf("/library/%d/movie/%s", libraryID, docKey)
	if _, err := dbConn.Exec(`INSERT INTO search_documents (
doc_key, kind, library_id, library_name, library_type, title, normalized_title, subtitle,
poster_path, poster_url, imdb_rating, href, show_key, media_id, title_ref_id, updated_at
) VALUES (?, 'movie', ?, ?, ?, ?, ?, '', '', '', 0, ?, '', 0, 0, ?)`,
		docKey,
		libraryID,
		libraryName,
		libraryType,
		title,
		normalizeSearchText(title),
		href,
		now,
	); err != nil {
		t.Fatalf("insert search document %s: %v", docKey, err)
	}
	if _, err := dbConn.Exec(`INSERT INTO search_documents_fts (doc_key, title, normalized_title) VALUES (?, ?, ?)`,
		docKey, title, normalizeSearchText(title)); err != nil {
		t.Fatalf("insert search document fts %s: %v", docKey, err)
	}
}

func TestSearchLibraryMedia_AppliesLibraryFilterBeforeFTSLimit(t *testing.T) {
	dbConn := newTestDB(t)
	defer dbConn.Close()

	targetLibraryID := getLibraryID(t, dbConn, LibraryTypeMovie)
	otherLibraryID := createLibraryForTest(t, dbConn, LibraryTypeMovie, "/other-movies")

	for i := 0; i < 3; i++ {
		insertSearchDocumentForTest(t, dbConn, otherLibraryID, fmt.Sprintf("other-%d", i), fmt.Sprintf("Matrix Clone %d", i))
	}
	insertSearchDocumentForTest(t, dbConn, targetLibraryID, "target", "The Matrix")

	response, err := SearchLibraryMedia(dbConn, SearchQuery{
		UserID:    testUserID(t, dbConn),
		Query:     "matrix",
		LibraryID: targetLibraryID,
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("search library media: %v", err)
	}
	if len(response.Results) != 1 {
		t.Fatalf("expected one filtered search result, got %d", len(response.Results))
	}
	if response.Results[0].LibraryID != targetLibraryID {
		t.Fatalf("expected result from library %d, got %d", targetLibraryID, response.Results[0].LibraryID)
	}
	if response.Results[0].Title != "The Matrix" {
		t.Fatalf("expected target title, got %q", response.Results[0].Title)
	}
}

func TestGetLibraryMovieDetails_ConvertsRuntimeToMinutes(t *testing.T) {
	dbConn := newTestDB(t)
	defer dbConn.Close()

	libraryID := getLibraryID(t, dbConn, LibraryTypeMovie)
	var refID int
	if err := dbConn.QueryRow(`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		libraryID, "Runtime Test", "/movies/runtime-test.mp4", 7200, MatchStatusIdentified).Scan(&refID); err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	var mediaID int
	if err := dbConn.QueryRow(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`, LibraryTypeMovie, refID).Scan(&mediaID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}

	details, err := GetLibraryMovieDetails(dbConn, libraryID, mediaID)
	if err != nil {
		t.Fatalf("get library movie details: %v", err)
	}
	if details == nil {
		t.Fatal("expected movie details")
	}
	if details.Runtime != 120 {
		t.Fatalf("expected 120 minute runtime, got %d", details.Runtime)
	}
}
