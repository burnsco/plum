package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"plum/internal/db"
)

func seedLibraryForScanActivityTest(t *testing.T) (*sql.DB, int, int) {
	t.Helper()

	dbConn, err := db.InitDB(filepath.Join(t.TempDir(), "plum.db"))
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	now := time.Now().UTC()
	var userID int
	if err := dbConn.QueryRow(
		`INSERT INTO users (email, password_hash, is_admin, created_at) VALUES (?, ?, 1, ?) RETURNING id`,
		"test@test.com",
		"hash",
		now,
	).Scan(&userID); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	var libraryID int
	if err := dbConn.QueryRow(
		`INSERT INTO libraries (user_id, name, type, path, created_at) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		userID,
		"TV",
		db.LibraryTypeTV,
		"/library",
		now,
	).Scan(&libraryID); err != nil {
		t.Fatalf("insert library: %v", err)
	}

	return dbConn, userID, libraryID
}

func TestGetLibraryScanStatus_IncludesActivityDetails(t *testing.T) {
	dbConn, userID, libraryID := seedLibraryForScanActivityTest(t)

	manager := NewLibraryScanManager(dbConn, nil, nil)
	manager.mu.Lock()
	manager.jobs[libraryID] = libraryScanStatus{
		LibraryID:      libraryID,
		Phase:          libraryScanPhaseScanning,
		IdentifyPhase:  libraryIdentifyPhaseIdle,
		MaxRetries:     3,
		EstimatedItems: 12,
	}
	manager.paths[libraryID] = "/library"
	manager.activities[libraryID] = libraryScanActivity{
		Stage: "discovery",
		Current: &libraryScanActivityEntry{
			Phase:        "discovery",
			Target:       "file",
			RelativePath: "Shows/Example/episode01.mkv",
			At:           time.Now().UTC().Format(time.RFC3339),
		},
		Recent: []libraryScanActivityEntry{
			{
				Phase:        "discovery",
				Target:       "directory",
				RelativePath: "Shows/Example",
				At:           time.Now().UTC().Format(time.RFC3339),
			},
		},
	}
	manager.mu.Unlock()

	handler := &LibraryHandler{
		DB:       dbConn,
		ScanJobs: manager,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/libraries/"+strconv.Itoa(libraryID)+"/scan", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", strconv.Itoa(libraryID))
	req = req.WithContext(context.WithValue(withUser(req.Context(), &db.User{ID: userID, IsAdmin: true}), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	handler.GetLibraryScanStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload libraryScanStatus
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Activity == nil {
		t.Fatal("expected activity details in scan status")
	}
	if payload.Activity.Current == nil || payload.Activity.Current.RelativePath != "Shows/Example/episode01.mkv" {
		t.Fatalf("unexpected current activity: %+v", payload.Activity.Current)
	}
	if len(payload.Activity.Recent) != 1 || payload.Activity.Recent[0].RelativePath != "Shows/Example" {
		t.Fatalf("unexpected recent activity: %+v", payload.Activity.Recent)
	}
}

func TestLibraryScanManager_TracksRelativeActivityAndCapsRecent(t *testing.T) {
	manager := NewLibraryScanManager(nil, nil, nil)
	manager.mu.Lock()
	manager.jobs[7] = libraryScanStatus{
		LibraryID:     7,
		Phase:         libraryScanPhaseScanning,
		IdentifyPhase: libraryIdentifyPhaseIdle,
		MaxRetries:    3,
	}
	manager.paths[7] = "/library"
	manager.mu.Unlock()

	manager.recordActivity(7, "discovery", "directory", "/library/Shows")
	manager.recordActivity(7, "enrichment", "file", "/library/Shows/Example/episode01.mkv")
	manager.RecordIdentifyActivity(7, "/library/Shows/Example/episode01.mkv")
	for idx := 0; idx < 24; idx++ {
		manager.recordActivity(
			7,
			"discovery",
			"file",
			filepath.Join("/library", "Shows", "Example", "episode"+strconv.Itoa(idx)+".mkv"),
		)
	}

	status := manager.status(7)
	if status.Activity == nil {
		t.Fatal("expected activity details")
	}
	if status.Activity.Current == nil || status.Activity.Current.RelativePath != "Shows/Example/episode23.mkv" {
		t.Fatalf("unexpected current activity: %+v", status.Activity.Current)
	}
	if len(status.Activity.Recent) != libraryScanActivityMaxRecent {
		t.Fatalf("recent activity len = %d, want %d", len(status.Activity.Recent), libraryScanActivityMaxRecent)
	}
	if status.Activity.Recent[0].RelativePath != "Shows/Example/episode23.mkv" {
		t.Fatalf("recent[0] relative path = %q", status.Activity.Recent[0].RelativePath)
	}
	if status.Activity.Recent[len(status.Activity.Recent)-1].RelativePath != "Shows/Example/episode4.mkv" {
		t.Fatalf("recent tail relative path = %q", status.Activity.Recent[len(status.Activity.Recent)-1].RelativePath)
	}
}

func TestLibraryScanManager_FinalizeActivityClearsOnSuccessAndKeepsFailureWithoutPaths(t *testing.T) {
	manager := NewLibraryScanManager(nil, nil, nil)

	manager.mu.Lock()
	manager.jobs[9] = libraryScanStatus{
		LibraryID:     9,
		Phase:         libraryScanPhaseCompleted,
		IdentifyPhase: libraryIdentifyPhaseCompleted,
		MaxRetries:    3,
	}
	manager.paths[9] = "/library"
	manager.activities[9] = libraryScanActivity{
		Stage: "identify",
		Current: &libraryScanActivityEntry{
			Phase:        "identify",
			Target:       "file",
			RelativePath: "Shows/Example/episode01.mkv",
			At:           time.Now().UTC().Format(time.RFC3339),
		},
		Recent: []libraryScanActivityEntry{{
			Phase:        "identify",
			Target:       "file",
			RelativePath: "Shows/Example/episode01.mkv",
			At:           time.Now().UTC().Format(time.RFC3339),
		}},
	}
	manager.finalizeActivityLocked(9, manager.jobs[9])
	manager.mu.Unlock()

	if status := manager.status(9); status.Activity != nil {
		t.Fatalf("expected successful activity to clear, got %+v", status.Activity)
	}

	manager.mu.Lock()
	manager.jobs[9] = libraryScanStatus{
		LibraryID:     9,
		Phase:         libraryScanPhaseCompleted,
		IdentifyPhase: libraryIdentifyPhaseFailed,
		MaxRetries:    3,
	}
	manager.activities[9] = libraryScanActivity{
		Stage: "identify",
		Current: &libraryScanActivityEntry{
			Phase:        "identify",
			Target:       "file",
			RelativePath: "Shows/Example/episode02.mkv",
			At:           time.Now().UTC().Format(time.RFC3339),
		},
		Recent: []libraryScanActivityEntry{{
			Phase:        "identify",
			Target:       "file",
			RelativePath: "Shows/Example/episode02.mkv",
			At:           time.Now().UTC().Format(time.RFC3339),
		}},
	}
	manager.finalizeActivityLocked(9, manager.jobs[9])
	manager.mu.Unlock()

	status := manager.status(9)
	if status.Activity == nil {
		t.Fatal("expected failed activity details")
	}
	if status.Activity.Stage != "failed" {
		t.Fatalf("failed activity stage = %q", status.Activity.Stage)
	}
	if status.Activity.Current != nil || len(status.Activity.Recent) != 0 {
		t.Fatalf("expected failed activity to clear live paths, got %+v", status.Activity)
	}
}

func TestLibraryScanUpdatePayload_IncludesActivity(t *testing.T) {
	payload, err := libraryScanUpdatePayload(libraryScanStatus{
		LibraryID:     5,
		Phase:         libraryScanPhaseScanning,
		IdentifyPhase: libraryIdentifyPhaseIdle,
		Activity: &libraryScanActivity{
			Stage: "discovery",
			Current: &libraryScanActivityEntry{
				Phase:        "discovery",
				Target:       "file",
				RelativePath: "Movies/Example (2024)/Example.mkv",
				At:           time.Now().UTC().Format(time.RFC3339),
			},
			Recent: []libraryScanActivityEntry{},
		},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	scan, ok := decoded["scan"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected scan payload: %#v", decoded["scan"])
	}
	activity, ok := scan["activity"].(map[string]any)
	if !ok {
		t.Fatalf("expected activity payload, got %#v", scan["activity"])
	}
	if activity["stage"] != "discovery" {
		t.Fatalf("activity stage = %#v, want discovery", activity["stage"])
	}
}
