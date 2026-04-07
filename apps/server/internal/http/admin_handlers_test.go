package httpapi

import (
	"context"
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
