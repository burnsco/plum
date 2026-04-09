package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

const adminMaintenanceScheduleKey = "admin_maintenance_schedule_v1"

// AdminMaintenanceTaskID identifies a scheduled maintenance task.
type AdminMaintenanceTaskID string

const (
	AdminTaskOptimizeDatabase     AdminMaintenanceTaskID = "optimize_database"
	AdminTaskCleanTranscode       AdminMaintenanceTaskID = "clean_transcode"
	AdminTaskCleanLogs            AdminMaintenanceTaskID = "clean_logs"
	AdminTaskDeleteCache          AdminMaintenanceTaskID = "delete_cache"
	AdminTaskScanAllMedia         AdminMaintenanceTaskID = "scan_all_media"
	AdminTaskExtractChapterImages AdminMaintenanceTaskID = "extract_chapter_images"
	AdminTaskCheckMetadataUpdates AdminMaintenanceTaskID = "check_metadata_updates"
)

// AllAdminMaintenanceTaskIDs is the canonical task list for schedule serialization.
var AllAdminMaintenanceTaskIDs = []AdminMaintenanceTaskID{
	AdminTaskOptimizeDatabase,
	AdminTaskCleanTranscode,
	AdminTaskCleanLogs,
	AdminTaskDeleteCache,
	AdminTaskScanAllMedia,
	AdminTaskExtractChapterImages,
	AdminTaskCheckMetadataUpdates,
}

// AdminMaintenanceScheduleTask holds per-task interval; 0 means disabled.
type AdminMaintenanceScheduleTask struct {
	IntervalHours int `json:"intervalHours"`
}

// AdminMaintenanceSchedule is persisted JSON for admin automation.
type AdminMaintenanceSchedule struct {
	Tasks    map[AdminMaintenanceTaskID]AdminMaintenanceScheduleTask `json:"tasks"`
	LastRun  map[AdminMaintenanceTaskID]string                       `json:"lastRun"`            // RFC3339 UTC
	SeededAt string                                                  `json:"seededAt,omitempty"` // RFC3339 UTC, internal scheduler baseline
}

func defaultAdminMaintenanceIntervalHours(task AdminMaintenanceTaskID) int {
	switch task {
	case AdminTaskOptimizeDatabase:
		// Jellyfin's OptimizeDatabaseTask defaults to every 6h; Plum uses SQLite VACUUM (blocking), so default weekly.
		return 24 * 7
	case AdminTaskCleanTranscode:
		// Jellyfin: startup + 24h. Intervals-only scheduler covers the daily pass; startup cleanup is in cmd/plum.
		return 24
	case AdminTaskCleanLogs:
		return 24
	case AdminTaskDeleteCache:
		return 24
	case AdminTaskScanAllMedia:
		// Jellyfin RefreshMediaLibraryTask: 12h. Full scans are heavy; 12h matches that cadence for new installs.
		return 12
	case AdminTaskExtractChapterImages:
		// Jellyfin ChapterImagesTask: daily (~2:00). One day is the closest single interval.
		return 24
	case AdminTaskCheckMetadataUpdates:
		// Jellyfin subtitle/lyric tasks: 24h; people refresh: 7d — default daily for a combined check.
		return 24
	default:
		return 0
	}
}

func defaultAdminMaintenanceSchedule() AdminMaintenanceSchedule {
	tasks := make(map[AdminMaintenanceTaskID]AdminMaintenanceScheduleTask, len(AllAdminMaintenanceTaskIDs))
	for _, id := range AllAdminMaintenanceTaskIDs {
		tasks[id] = AdminMaintenanceScheduleTask{IntervalHours: defaultAdminMaintenanceIntervalHours(id)}
	}
	return AdminMaintenanceSchedule{
		Tasks:    tasks,
		LastRun:  make(map[AdminMaintenanceTaskID]string),
		SeededAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func normalizeAdminMaintenanceSchedule(s AdminMaintenanceSchedule) AdminMaintenanceSchedule {
	now := time.Now().UTC().Format(time.RFC3339)
	if s.Tasks == nil {
		s.Tasks = defaultAdminMaintenanceSchedule().Tasks
	} else {
		for _, id := range AllAdminMaintenanceTaskIDs {
			if _, ok := s.Tasks[id]; !ok {
				s.Tasks[id] = AdminMaintenanceScheduleTask{IntervalHours: defaultAdminMaintenanceIntervalHours(id)}
			}
		}
	}
	if s.LastRun == nil {
		s.LastRun = make(map[AdminMaintenanceTaskID]string)
	}
	if s.SeededAt == "" {
		s.SeededAt = now
	} else if _, err := time.Parse(time.RFC3339, s.SeededAt); err != nil {
		s.SeededAt = now
	}
	return s
}

// GetAdminMaintenanceSchedule loads schedule from app_settings.
func GetAdminMaintenanceSchedule(dbConn *sql.DB) (AdminMaintenanceSchedule, error) {
	if dbConn == nil {
		return AdminMaintenanceSchedule{}, errors.New("nil db")
	}
	def := defaultAdminMaintenanceSchedule()
	var raw string
	err := dbConn.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, adminMaintenanceScheduleKey).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err := SaveAdminMaintenanceSchedule(context.Background(), dbConn, def); err != nil {
				return def, nil
			}
			return def, nil
		}
		return AdminMaintenanceSchedule{}, err
	}
	var parsed AdminMaintenanceSchedule
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		if err := SaveAdminMaintenanceSchedule(context.Background(), dbConn, def); err != nil {
			return def, nil
		}
		return def, nil
	}
	normalized := normalizeAdminMaintenanceSchedule(parsed)
	if normalized.SeededAt != parsed.SeededAt || len(normalized.Tasks) != len(parsed.Tasks) || len(normalized.LastRun) != len(parsed.LastRun) {
		if err := SaveAdminMaintenanceSchedule(context.Background(), dbConn, normalized); err != nil {
			return normalized, nil
		}
	}
	return normalized, nil
}

// SaveAdminMaintenanceSchedule replaces the stored schedule (tasks + last run times).
func SaveAdminMaintenanceSchedule(ctx context.Context, dbConn *sql.DB, s AdminMaintenanceSchedule) error {
	if dbConn == nil {
		return errors.New("nil db")
	}
	s = normalizeAdminMaintenanceSchedule(s)
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = dbConn.ExecContext(ctx, `
INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		adminMaintenanceScheduleKey, string(b), now)
	return err
}

// TouchAdminMaintenanceLastRun sets last run time for a single task (merged into stored schedule).
func TouchAdminMaintenanceLastRun(ctx context.Context, dbConn *sql.DB, task AdminMaintenanceTaskID) error {
	s, err := GetAdminMaintenanceSchedule(dbConn)
	if err != nil {
		return err
	}
	if s.LastRun == nil {
		s.LastRun = make(map[AdminMaintenanceTaskID]string)
	}
	s.LastRun[task] = time.Now().UTC().Format(time.RFC3339)
	return SaveAdminMaintenanceSchedule(ctx, dbConn, s)
}
