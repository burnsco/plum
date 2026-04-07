package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdateMediaFileIntroFromProbe_UsesPrimaryRowNotPath(t *testing.T) {
	dbConn := newTestDB(t)
	tvLibID := getLibraryID(t, dbConn, "tv")

	tmp := t.TempDir()
	showDir := filepath.Join(tmp, "Show", "Season 1")
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mediaPath := filepath.Join(showDir, "Show - S01E01.mkv")
	if err := os.WriteFile(mediaPath, []byte("video-bytes"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	prevHash := computeMediaHash
	prevVideoProbe := readVideoMetadata
	computeMediaHash = func(_ context.Context, path string) (string, error) {
		return filepath.Base(path), nil
	}
	readVideoMetadata = func(_ context.Context, path string) (VideoProbeResult, error) {
		return VideoProbeResult{Duration: 42}, nil
	}
	t.Cleanup(func() {
		computeMediaHash = prevHash
		readVideoMetadata = prevVideoProbe
	})

	delta, err := ScanLibraryDiscovery(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
	})
	if err != nil {
		t.Fatalf("discovery: %v", err)
	}
	if len(delta.TouchedFiles) != 1 {
		t.Fatalf("touched = %d, want 1", len(delta.TouchedFiles))
	}
	if err := EnrichLibraryTasks(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, delta.TouchedFiles, ScanOptions{
		ProbeMedia:             true,
		ProbeEmbeddedSubtitles: false,
	}); err != nil {
		t.Fatalf("enrich: %v", err)
	}

	var mediaID int
	if err := dbConn.QueryRow(`
SELECT g.id FROM media_global g
JOIN tv_episodes e ON e.id = g.ref_id AND g.kind = 'tv'
WHERE e.library_id = ? AND e.path = ?`, tvLibID, mediaPath).Scan(&mediaID); err != nil {
		t.Fatalf("lookup media id: %v", err)
	}

	if _, err := dbConn.Exec(`UPDATE media_files SET intro_start_sec = NULL, intro_end_sec = NULL, intro_probed_at = NULL WHERE media_id = ?`, mediaID); err != nil {
		t.Fatalf("clear intro: %v", err)
	}

	s, e := 10.0, 120.0
	if err := UpdateMediaFileIntroFromProbe(context.Background(), dbConn, mediaID, "/wrong/path/does-not-match-db.mkv", VideoProbeResult{
		IntroStartSeconds: &s,
		IntroEndSeconds:   &e,
	}); err != nil {
		t.Fatalf("UpdateMediaFileIntroFromProbe: %v", err)
	}

	var gotStart float64
	var probedAt string
	if err := dbConn.QueryRow(`SELECT intro_start_sec, intro_probed_at FROM media_files WHERE media_id = ? AND is_primary = 1`, mediaID).
		Scan(&gotStart, &probedAt); err != nil {
		t.Fatalf("select intro: %v", err)
	}
	if gotStart != s {
		t.Fatalf("intro_start_sec = %v, want %v", gotStart, s)
	}
	if probedAt == "" {
		t.Fatalf("expected intro_probed_at to be set")
	}
}

func TestUpdateMediaFileIntroFromProbe_RespectsLocked(t *testing.T) {
	dbConn := newTestDB(t)
	tvLibID := getLibraryID(t, dbConn, "tv")

	tmp := t.TempDir()
	showDir := filepath.Join(tmp, "Show", "Season 1")
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mediaPath := filepath.Join(showDir, "Show - S01E01.mkv")
	if err := os.WriteFile(mediaPath, []byte("video-bytes"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	prevHash := computeMediaHash
	prevVideoProbe := readVideoMetadata
	computeMediaHash = func(_ context.Context, path string) (string, error) {
		return filepath.Base(path), nil
	}
	readVideoMetadata = func(_ context.Context, path string) (VideoProbeResult, error) {
		return VideoProbeResult{Duration: 42}, nil
	}
	t.Cleanup(func() {
		computeMediaHash = prevHash
		readVideoMetadata = prevVideoProbe
	})

	delta, err := ScanLibraryDiscovery(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, ScanOptions{
		HashMode: ScanHashModeDefer,
	})
	if err != nil {
		t.Fatalf("discovery: %v", err)
	}
	if err := EnrichLibraryTasks(context.Background(), dbConn, tmp, LibraryTypeTV, tvLibID, delta.TouchedFiles, ScanOptions{
		ProbeMedia:             true,
		ProbeEmbeddedSubtitles: false,
	}); err != nil {
		t.Fatalf("enrich: %v", err)
	}

	var mediaID int
	if err := dbConn.QueryRow(`
SELECT g.id FROM media_global g
JOIN tv_episodes e ON e.id = g.ref_id AND g.kind = 'tv'
WHERE e.library_id = ? AND e.path = ?`, tvLibID, mediaPath).Scan(&mediaID); err != nil {
		t.Fatalf("lookup media id: %v", err)
	}

	if _, err := dbConn.Exec(`UPDATE media_files SET intro_locked = 1, intro_start_sec = 1, intro_end_sec = 2 WHERE media_id = ? AND is_primary = 1`, mediaID); err != nil {
		t.Fatalf("lock intro: %v", err)
	}

	s, e := 99.0, 120.0
	if err := UpdateMediaFileIntroFromProbe(context.Background(), dbConn, mediaID, mediaPath, VideoProbeResult{
		IntroStartSeconds: &s,
		IntroEndSeconds:   &e,
	}); err != nil {
		t.Fatalf("UpdateMediaFileIntroFromProbe: %v", err)
	}

	var gotStart, gotEnd float64
	if err := dbConn.QueryRow(`SELECT intro_start_sec, intro_end_sec FROM media_files WHERE media_id = ? AND is_primary = 1`, mediaID).
		Scan(&gotStart, &gotEnd); err != nil {
		t.Fatalf("select intro: %v", err)
	}
	if gotStart != 1 || gotEnd != 2 {
		t.Fatalf("intro was overwritten: got %v,%v want 1,2", gotStart, gotEnd)
	}
}
