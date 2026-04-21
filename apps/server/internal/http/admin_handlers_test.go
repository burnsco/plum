package httpapi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"plum/internal/db"

	_ "modernc.org/sqlite"
)

func TestAdminBatchUserEmails_DedupesAndMaps(t *testing.T) {
	t.Parallel()
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	_, err = dbConn.Exec(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 0, ?), (?, ?, 0, ?)`,
		"a@example.com", "h1", now,
		"b@example.com", "h2", now,
	)
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}

	m, err := adminBatchUserEmails(dbConn, []int{2, 1, 2, 1})
	if err != nil {
		t.Fatalf("adminBatchUserEmails: %v", err)
	}
	if m[1] != "a@example.com" || m[2] != "b@example.com" {
		t.Fatalf("emails = %#v", m)
	}
}

func TestAdminBatchUserEmails_EmptyIDs(t *testing.T) {
	t.Parallel()
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })
	m, err := adminBatchUserEmails(dbConn, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map, got %#v", m)
	}
}

func TestRunMaintenanceTask_ScanAllMedia_RequiresScanQueue(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	h := &AdminHandler{DB: dbConn, ScanJobs: nil, Lib: nil}
	accepted, status, payload := h.runMaintenanceTask(context.Background(), db.AdminTaskScanAllMedia, true)
	// runMaintenanceTask returns accepted=true when the switch handled the task; check HTTP status and payload.
	if !accepted || status != 503 {
		t.Fatalf("accepted=%v status=%d payload=%#v", accepted, status, payload)
	}
	if payload["accepted"] != false || payload["error"] != "scan queue unavailable" {
		t.Fatalf("payload=%#v", payload)
	}
}

func TestRunMaintenanceTask_ScanAllMedia_QueuesRows(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"admin@example.com", "hash", now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := dbConn.Exec(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?)`,
		userID, "Movies", db.LibraryTypeMovie, "/movies", now,
	); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	h := &AdminHandler{
		DB:       dbConn,
		ScanJobs: scanJobs,
		Lib:      &LibraryHandler{DB: dbConn},
	}
	accepted, status, payload := h.runMaintenanceTask(context.Background(), db.AdminTaskScanAllMedia, true)
	if !accepted || status != 200 {
		t.Fatalf("accepted=%v status=%d payload=%#v", accepted, status, payload)
	}
	detail, _ := payload["detail"].(string)
	if detail != "Queued library scans for 1 libraries." {
		t.Fatalf("detail = %q", detail)
	}
}

func TestRunMaintenanceTask_ScanAllMedia_EmptyLibraryTable(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	scanJobs := NewLibraryScanManager(context.Background(), dbConn, nil, nil, "")
	h := &AdminHandler{DB: dbConn, ScanJobs: scanJobs, Lib: &LibraryHandler{DB: dbConn}}
	accepted, status, payload := h.runMaintenanceTask(context.Background(), db.AdminTaskScanAllMedia, true)
	if !accepted || status != 200 {
		t.Fatalf("accepted=%v status=%d payload=%#v", accepted, status, payload)
	}
	detail, _ := payload["detail"].(string)
	if detail != "Queued library scans for 0 libraries." {
		t.Fatalf("detail = %q", detail)
	}
}

func TestRunMaintenanceTask_CleanLogsWithOnlyLogFileSkipsDirectorySweep(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	dir := t.TempDir()
	plumLog := filepath.Join(dir, "plum.log")
	otherLog := filepath.Join(dir, "other.log")
	old := time.Now().Add(-96 * time.Hour)
	for _, path := range []string{plumLog, otherLog} {
		if err := os.WriteFile(path, []byte("log\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatalf("chtimes %s: %v", path, err)
		}
	}

	h := &AdminHandler{DB: dbConn, LogFile: plumLog}
	accepted, status, payload := h.runMaintenanceTask(context.Background(), db.AdminTaskCleanLogs, true)
	if !accepted || status != 200 {
		t.Fatalf("accepted=%v status=%d payload=%#v", accepted, status, payload)
	}
	detail, _ := payload["detail"].(string)
	if !strings.Contains(detail, "skipping recursive log cleanup") {
		t.Fatalf("detail = %q", detail)
	}
	for _, path := range []string{plumLog, otherLog} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to remain: %v", path, err)
		}
	}
}

func TestRunMaintenanceTask_CleanLogsWithLogDirRemovesOldLogFiles(t *testing.T) {
	dbConn, err := db.InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	dir := t.TempDir()
	oldLog := filepath.Join(dir, "old.log")
	freshLog := filepath.Join(dir, "fresh.log")
	oldText := filepath.Join(dir, "old.txt")
	for _, path := range []string{oldLog, freshLog, oldText} {
		if err := os.WriteFile(path, []byte("log\n"), 0o600); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	old := time.Now().Add(-96 * time.Hour)
	if err := os.Chtimes(oldLog, old, old); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(oldText, old, old); err != nil {
		t.Fatal(err)
	}

	h := &AdminHandler{DB: dbConn, LogDir: dir}
	accepted, status, payload := h.runMaintenanceTask(context.Background(), db.AdminTaskCleanLogs, true)
	if !accepted || status != 200 {
		t.Fatalf("accepted=%v status=%d payload=%#v", accepted, status, payload)
	}
	if _, err := os.Stat(oldLog); !os.IsNotExist(err) {
		t.Fatalf("expected old log to be removed, stat err=%v", err)
	}
	for _, path := range []string{freshLog, oldText} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to remain: %v", path, err)
		}
	}
}

func TestAdminTaskDue_UsesSeededAtBeforeLastRun(t *testing.T) {
	t.Parallel()

	seededAt := time.Date(2026, time.January, 1, 12, 0, 0, 0, time.UTC)
	s := db.AdminMaintenanceSchedule{
		Tasks: map[db.AdminMaintenanceTaskID]db.AdminMaintenanceScheduleTask{
			db.AdminTaskCleanLogs: {IntervalHours: 24},
		},
		LastRun:  map[db.AdminMaintenanceTaskID]string{},
		SeededAt: seededAt.Format(time.RFC3339),
	}

	if adminTaskDue(s, db.AdminTaskCleanLogs, seededAt.Add(23*time.Hour+59*time.Minute)) {
		t.Fatal("task should not be due before the seeded baseline interval passes")
	}
	if !adminTaskDue(s, db.AdminTaskCleanLogs, seededAt.Add(24*time.Hour)) {
		t.Fatal("task should be due once the seeded baseline interval passes")
	}
}
