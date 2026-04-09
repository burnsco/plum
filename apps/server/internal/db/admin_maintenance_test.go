package db

import (
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestDefaultAdminMaintenanceScheduleUsesPracticalIntervals(t *testing.T) {
	t.Parallel()

	s := defaultAdminMaintenanceSchedule()

	want := map[AdminMaintenanceTaskID]int{
		AdminTaskOptimizeDatabase:     24 * 7,
		AdminTaskCleanTranscode:       24,
		AdminTaskCleanLogs:            24,
		AdminTaskDeleteCache:          24,
		AdminTaskScanAllMedia:         12,
		AdminTaskExtractChapterImages: 24,
		AdminTaskCheckMetadataUpdates: 24,
	}

	for task, wantHours := range want {
		got, ok := s.Tasks[task]
		if !ok {
			t.Fatalf("missing default for %s", task)
		}
		if got.IntervalHours != wantHours {
			t.Fatalf("default interval for %s = %d, want %d", task, got.IntervalHours, wantHours)
		}
	}

	if s.SeededAt == "" {
		t.Fatal("expected seededAt to be set")
	}
	if _, err := time.Parse(time.RFC3339, s.SeededAt); err != nil {
		t.Fatalf("seededAt is not RFC3339: %v", err)
	}
}

func TestGetAdminMaintenanceScheduleSeedsDefaultsOnEmptyDatabase(t *testing.T) {
	t.Parallel()

	dbConn := newTestDB(t)
	defer dbConn.Close()

	s, err := GetAdminMaintenanceSchedule(dbConn)
	if err != nil {
		t.Fatalf("GetAdminMaintenanceSchedule: %v", err)
	}

	want := map[AdminMaintenanceTaskID]int{
		AdminTaskOptimizeDatabase:     24 * 7,
		AdminTaskCleanTranscode:       24,
		AdminTaskCleanLogs:            24,
		AdminTaskDeleteCache:          24,
		AdminTaskScanAllMedia:         12,
		AdminTaskExtractChapterImages: 24,
		AdminTaskCheckMetadataUpdates: 24,
	}
	for task, wantHours := range want {
		if got := s.Tasks[task].IntervalHours; got != wantHours {
			t.Fatalf("default interval for %s = %d, want %d", task, got, wantHours)
		}
	}
	if s.SeededAt == "" {
		t.Fatal("expected seededAt to be set")
	}

	var raw string
	if err := dbConn.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, adminMaintenanceScheduleKey).Scan(&raw); err != nil {
		t.Fatalf("schedule not persisted: %v", err)
	}
}
