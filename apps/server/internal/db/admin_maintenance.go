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
	Tasks   map[AdminMaintenanceTaskID]AdminMaintenanceScheduleTask `json:"tasks"`
	LastRun map[AdminMaintenanceTaskID]string                       `json:"lastRun"` // RFC3339 UTC
}

func defaultAdminMaintenanceSchedule() AdminMaintenanceSchedule {
	tasks := make(map[AdminMaintenanceTaskID]AdminMaintenanceScheduleTask, len(AllAdminMaintenanceTaskIDs))
	for _, id := range AllAdminMaintenanceTaskIDs {
		tasks[id] = AdminMaintenanceScheduleTask{IntervalHours: 0}
	}
	return AdminMaintenanceSchedule{
		Tasks:   tasks,
		LastRun: make(map[AdminMaintenanceTaskID]string),
	}
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
			return def, nil
		}
		return AdminMaintenanceSchedule{}, err
	}
	var parsed AdminMaintenanceSchedule
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return def, nil
	}
	if parsed.Tasks == nil {
		parsed.Tasks = def.Tasks
	} else {
		for _, id := range AllAdminMaintenanceTaskIDs {
			if _, ok := parsed.Tasks[id]; !ok {
				parsed.Tasks[id] = AdminMaintenanceScheduleTask{IntervalHours: 0}
			}
		}
	}
	if parsed.LastRun == nil {
		parsed.LastRun = make(map[AdminMaintenanceTaskID]string)
	}
	return parsed, nil
}

// SaveAdminMaintenanceSchedule replaces the stored schedule (tasks + last run times).
func SaveAdminMaintenanceSchedule(ctx context.Context, dbConn *sql.DB, s AdminMaintenanceSchedule) error {
	if dbConn == nil {
		return errors.New("nil db")
	}
	if s.Tasks == nil {
		s.Tasks = defaultAdminMaintenanceSchedule().Tasks
	}
	if s.LastRun == nil {
		s.LastRun = make(map[AdminMaintenanceTaskID]string)
	}
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
