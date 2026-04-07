package db

import (
	"context"
	"testing"
	"time"
)

func TestListEpisodesMissingThumbnails_FiltersPresentAndMissing(t *testing.T) {
	dbConn := newTestDB(t)
	tvLibID := getLibraryID(t, dbConn, LibraryTypeTV)
	ctx := context.Background()

	var epNoThumb, epThumb, epMissing int
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode) VALUES (?, 'A', '/tv/a.mkv', 60, ?, 1, 1) RETURNING id`,
		tvLibID, MatchStatusLocal).Scan(&epNoThumb); err != nil {
		t.Fatalf("insert episode: %v", err)
	}
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode, thumbnail_path) VALUES (?, 'B', '/tv/b.mkv', 60, ?, 1, 2, '99.jpg') RETURNING id`,
		tvLibID, MatchStatusLocal).Scan(&epThumb); err != nil {
		t.Fatalf("insert episode: %v", err)
	}
	missingAt := time.Now().UTC().Format(time.RFC3339)
	if err := dbConn.QueryRow(`INSERT INTO tv_episodes (library_id, title, path, duration, match_status, season, episode, missing_since) VALUES (?, 'C', '/tv/c.mkv', 60, ?, 1, 3, ?) RETURNING id`,
		tvLibID, MatchStatusLocal, missingAt).Scan(&epMissing); err != nil {
		t.Fatalf("insert episode: %v", err)
	}
	for _, refID := range []int{epNoThumb, epThumb, epMissing} {
		if _, err := dbConn.Exec(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?)`, LibraryTypeTV, refID); err != nil {
			t.Fatalf("insert media_global: %v", err)
		}
	}

	var globalNoThumb int
	if err := dbConn.QueryRow(`SELECT id FROM media_global WHERE kind = ? AND ref_id = ?`, LibraryTypeTV, epNoThumb).Scan(&globalNoThumb); err != nil {
		t.Fatalf("lookup global: %v", err)
	}

	tasks, err := ListEpisodesMissingThumbnails(ctx, dbConn, tvLibID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("want 1 missing-thumbnail task, got %d (%+v)", len(tasks), tasks)
	}
	if tasks[0].GlobalID != globalNoThumb || tasks[0].Path != "/tv/a.mkv" {
		t.Fatalf("unexpected task: %+v", tasks[0])
	}

	allLib, err := ListEpisodesMissingThumbnails(ctx, dbConn, 0)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(allLib) != 1 {
		t.Fatalf("libraryId=0: want 1 task, got %d", len(allLib))
	}
}

func TestListTVAndAnimeLibraryIDs(t *testing.T) {
	dbConn := newTestDB(t)
	ids, err := ListTVAndAnimeLibraryIDs(dbConn)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(ids) < 1 {
		t.Fatalf("expected at least tv library, got %v", ids)
	}
}
