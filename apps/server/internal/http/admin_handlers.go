package httpapi

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"plum/internal/db"
	"plum/internal/transcoder"

	"github.com/go-chi/chi/v5"
)

// AdminHandler serves admin-only maintenance, activity, and log endpoints.
type AdminHandler struct {
	ShutdownCtx  context.Context
	DB           *sql.DB
	Lib          *LibraryHandler
	ScanJobs     *LibraryScanManager
	Sessions     *transcoder.PlaybackSessionManager
	PlaybackRoot string
	LogFile      string
	LogDir       string

	vacuumMu      sync.Mutex
	vacuumRunning bool
}

func (h *AdminHandler) GetMaintenanceSchedule(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DB == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	s, err := db.GetAdminMaintenanceSchedule(h.DB)
	if err != nil {
		http.Error(w, "failed to load schedule", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(adminScheduleJSON(s))
}

type adminSchedulePutBody struct {
	Tasks map[string]struct {
		IntervalHours int `json:"intervalHours"`
	} `json:"tasks"`
}

func (h *AdminHandler) PutMaintenanceSchedule(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DB == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var body adminSchedulePutBody
	if !decodeRequestJSON(w, r, &body) {
		return
	}
	current, err := db.GetAdminMaintenanceSchedule(h.DB)
	if err != nil {
		http.Error(w, "failed to load schedule", http.StatusInternalServerError)
		return
	}
	for _, id := range db.AllAdminMaintenanceTaskIDs {
		key := string(id)
		if in, ok := body.Tasks[key]; ok {
			if in.IntervalHours < 0 || in.IntervalHours > 24*365 {
				http.Error(w, "invalid intervalHours", http.StatusBadRequest)
				return
			}
			current.Tasks[id] = db.AdminMaintenanceScheduleTask{IntervalHours: in.IntervalHours}
		}
	}
	if err := db.SaveAdminMaintenanceSchedule(r.Context(), h.DB, current); err != nil {
		http.Error(w, "failed to save schedule", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(adminScheduleJSON(current))
}

func adminScheduleJSON(s db.AdminMaintenanceSchedule) map[string]any {
	tasksOut := make(map[string]any, len(s.Tasks))
	for k, v := range s.Tasks {
		tasksOut[string(k)] = map[string]any{"intervalHours": v.IntervalHours}
	}
	lastOut := make(map[string]string, len(s.LastRun))
	for k, v := range s.LastRun {
		lastOut[string(k)] = v
	}
	return map[string]any{
		"tasks":   tasksOut,
		"lastRun": lastOut,
	}
}

type adminRunBody struct {
	Task string `json:"task"`
}

func (h *AdminHandler) PostMaintenanceRun(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DB == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var body adminRunBody
	if !decodeRequestJSON(w, r, &body) {
		return
	}
	task := db.AdminMaintenanceTaskID(strings.TrimSpace(body.Task))
	accepted, status, payload := h.runMaintenanceTask(r.Context(), task, true)
	w.Header().Set("Content-Type", "application/json")
	if !accepted {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *AdminHandler) runMaintenanceTask(ctx context.Context, task db.AdminMaintenanceTaskID, _ bool) (accepted bool, status int, payload map[string]any) {
	payload = map[string]any{"task": string(task), "accepted": true}

	switch task {
	case db.AdminTaskOptimizeDatabase:
		h.vacuumMu.Lock()
		if h.vacuumRunning {
			h.vacuumMu.Unlock()
			return false, http.StatusConflict, map[string]any{
				"task": string(task), "accepted": false, "error": "optimize_database already running",
			}
		}
		h.vacuumRunning = true
		h.vacuumMu.Unlock()
		go func() {
			defer func() {
				h.vacuumMu.Lock()
				h.vacuumRunning = false
				h.vacuumMu.Unlock()
			}()
			runCtx := context.Background()
			if err := db.RunSQLiteVacuum(runCtx, h.DB); err != nil {
				slog.Warn("admin vacuum failed", "error", err)
				return
			}
			_ = db.TouchAdminMaintenanceLastRun(runCtx, h.DB, db.AdminTaskOptimizeDatabase)
			slog.Info("admin maintenance: optimize_database completed")
		}()
		payload["detail"] = "Database optimize started in the background."
		return true, http.StatusAccepted, payload

	case db.AdminTaskCleanTranscode:
		var active map[string]struct{}
		if h.Sessions != nil {
			active = h.Sessions.ActiveSessionIDSet()
		}
		nDir, err := transcoder.CleanStaleTranscodeSessionDirs(h.PlaybackRoot, active, 24*time.Hour)
		if err != nil {
			payload["accepted"] = false
			payload["error"] = err.Error()
			return true, http.StatusInternalServerError, payload
		}
		nLegacy, err := transcoder.CleanStaleLegacyTranscodes(os.TempDir(), 24*time.Hour)
		if err != nil {
			payload["accepted"] = false
			payload["error"] = err.Error()
			return true, http.StatusInternalServerError, payload
		}
		_ = db.TouchAdminMaintenanceLastRun(ctx, h.DB, db.AdminTaskCleanTranscode)
		payload["detail"] = fmt.Sprintf("Removed %d transcode session dirs and %d legacy transcode files older than one day.", nDir, nLegacy)
		return true, http.StatusOK, payload

	case db.AdminTaskCleanLogs:
		logDir := adminResolveLogDir(h.LogFile, h.LogDir)
		if logDir == "" {
			payload["detail"] = "No log directory configured (set PLUM_LOG_FILE or PLUM_LOG_DIR)."
			_ = db.TouchAdminMaintenanceLastRun(ctx, h.DB, db.AdminTaskCleanLogs)
			return true, http.StatusOK, payload
		}
		n, err := transcoder.CleanOldLogFiles(logDir, 72*time.Hour)
		if err != nil {
			payload["accepted"] = false
			payload["error"] = err.Error()
			return true, http.StatusInternalServerError, payload
		}
		_ = db.TouchAdminMaintenanceLastRun(ctx, h.DB, db.AdminTaskCleanLogs)
		payload["detail"] = fmt.Sprintf("Removed %d log files older than three days in %s.", n, logDir)
		return true, http.StatusOK, payload

	case db.AdminTaskDeleteCache:
		if err := db.RunMetadataProviderCacheCleanup(ctx, h.DB); err != nil {
			payload["accepted"] = false
			payload["error"] = err.Error()
			return true, http.StatusInternalServerError, payload
		}
		_ = db.TouchAdminMaintenanceLastRun(ctx, h.DB, db.AdminTaskDeleteCache)
		payload["detail"] = "Metadata provider cache pruned (expired rows and optional row-cap trim)."
		return true, http.StatusOK, payload

	case db.AdminTaskScanAllMedia:
		if h.ScanJobs == nil || h.Lib == nil {
			payload["accepted"] = false
			payload["error"] = "scan queue unavailable"
			return true, http.StatusServiceUnavailable, payload
		}
		rows, err := h.DB.QueryContext(ctx, `SELECT id, path, type FROM libraries ORDER BY id`)
		if err != nil {
			payload["accepted"] = false
			payload["error"] = err.Error()
			return true, http.StatusInternalServerError, payload
		}
		defer rows.Close()
		queued := 0
		var scanRowErr error
		for rows.Next() {
			var id int
			var path, typ string
			if err := rows.Scan(&id, &path, &typ); err != nil {
				scanRowErr = err
				break
			}
			h.ScanJobs.start(id, path, typ, true, nil)
			queued++
		}
		if err := rows.Err(); err != nil {
			payload["accepted"] = false
			payload["error"] = err.Error()
			payload["queuedLibraries"] = queued
			payload["detail"] = fmt.Sprintf("Iteration failed after queuing %d library scan(s).", queued)
			return true, http.StatusInternalServerError, payload
		}
		if scanRowErr != nil {
			payload["accepted"] = false
			payload["error"] = scanRowErr.Error()
			payload["queuedLibraries"] = queued
			payload["detail"] = fmt.Sprintf("Row scan failed after queuing %d library scan(s).", queued)
			return true, http.StatusInternalServerError, payload
		}
		_ = db.TouchAdminMaintenanceLastRun(ctx, h.DB, db.AdminTaskScanAllMedia)
		payload["detail"] = fmt.Sprintf("Queued library scans for %d libraries.", queued)
		return true, http.StatusOK, payload

	case db.AdminTaskExtractChapterImages:
		if h.Lib == nil {
			payload["accepted"] = false
			payload["error"] = "library handler unavailable"
			return true, http.StatusServiceUnavailable, payload
		}
		rows, err := h.DB.QueryContext(ctx, `SELECT id FROM libraries WHERE type IN ('movie','tv','anime') ORDER BY id`)
		if err != nil {
			payload["accepted"] = false
			payload["error"] = err.Error()
			return true, http.StatusInternalServerError, payload
		}
		var libIDs []int
		var scanRowErr error
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				scanRowErr = err
				break
			}
			libIDs = append(libIDs, id)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			payload["accepted"] = false
			payload["error"] = err.Error()
			payload["detail"] = fmt.Sprintf("Listed %d libraries before iteration error.", len(libIDs))
			return true, http.StatusInternalServerError, payload
		}
		if scanRowErr != nil {
			_ = rows.Close()
			payload["accepted"] = false
			payload["error"] = scanRowErr.Error()
			payload["detail"] = fmt.Sprintf("Listed %d libraries before row scan error.", len(libIDs))
			return true, http.StatusInternalServerError, payload
		}
		_ = rows.Close()
		started := 0
		skipped := 0
		for _, id := range libIDs {
			if h.Lib.startLibraryPlaybackRefreshAsync(id) {
				started++
			} else {
				skipped++
			}
		}
		_ = db.TouchAdminMaintenanceLastRun(ctx, h.DB, db.AdminTaskExtractChapterImages)
		payload["detail"] = fmt.Sprintf("Started playback metadata refresh (chapters/tracks) on %d libraries; %d skipped (already running or empty).", started, skipped)
		return true, http.StatusOK, payload

	case db.AdminTaskCheckMetadataUpdates:
		if h.Lib == nil {
			payload["accepted"] = false
			payload["error"] = "library handler unavailable"
			return true, http.StatusServiceUnavailable, payload
		}
		shutdownCtx := h.ShutdownCtx
		if shutdownCtx == nil {
			shutdownCtx = context.Background()
		}
		go func() {
			rows, err := h.DB.QueryContext(shutdownCtx, `SELECT id FROM libraries ORDER BY id`)
			if err != nil {
				slog.Warn("admin metadata check: list libraries", "error", err)
				return
			}
			defer rows.Close()
			for rows.Next() {
				if err := shutdownCtx.Err(); err != nil {
					return
				}
				var id int
				if err := rows.Scan(&id); err != nil {
					slog.Warn("admin metadata check: scan library row", "error", err)
					continue
				}
				if _, ierr := h.Lib.identifyLibrary(shutdownCtx, id); ierr != nil {
					slog.Warn("admin metadata check: identify library", "library_id", id, "error", ierr)
				}
			}
			if err := rows.Err(); err != nil {
				slog.Warn("admin metadata check: rows iteration", "error", err)
				return
			}
			touchCtx := shutdownCtx
			if touchCtx.Err() != nil {
				touchCtx = context.Background()
			}
			_ = db.TouchAdminMaintenanceLastRun(touchCtx, h.DB, db.AdminTaskCheckMetadataUpdates)
			slog.Info("admin maintenance: check_metadata_updates completed")
		}()
		payload["detail"] = "Metadata identify/refresh started in the background for all libraries."
		return true, http.StatusAccepted, payload

	default:
		return false, http.StatusBadRequest, map[string]any{
			"task": string(task), "accepted": false, "error": "unknown task",
		}
	}
}

// adminBatchUserEmails returns a map of user id -> email in one query (avoids N+1 on active playback).
func adminBatchUserEmails(dbConn *sql.DB, userIDs []int) (map[int]string, error) {
	if len(userIDs) == 0 {
		return map[int]string{}, nil
	}
	uniq := make([]int, 0, len(userIDs))
	seen := make(map[int]struct{}, len(userIDs))
	for _, id := range userIDs {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(uniq)), ",")
	q := `SELECT id, email FROM users WHERE id IN (` + placeholders + `)`
	args := make([]any, len(uniq))
	for i, id := range uniq {
		args[i] = id
	}
	rows, err := dbConn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int]string, len(uniq))
	for rows.Next() {
		var id int
		var email string
		if err := rows.Scan(&id, &email); err != nil {
			return nil, err
		}
		out[id] = email
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func adminResolveLogDir(logFile, logDir string) string {
	if strings.TrimSpace(logDir) != "" {
		return filepath.Clean(logDir)
	}
	if strings.TrimSpace(logFile) != "" {
		return filepath.Clean(filepath.Dir(logFile))
	}
	return ""
}

func (h *AdminHandler) GetActivePlayback(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.DB == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	var sessions []transcoder.ActivePlaybackSessionAdmin
	if h.Sessions != nil {
		sessions = h.Sessions.ListActiveSessionsForAdmin()
	}
	userIDs := make([]int, 0, len(sessions))
	for _, s := range sessions {
		userIDs = append(userIDs, s.UserID)
	}
	emails, err := adminBatchUserEmails(h.DB, userIDs)
	if err != nil {
		slog.Error("admin active playback: batch user emails", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, map[string]any{
			"sessionId":       s.SessionID,
			"userId":          s.UserID,
			"userEmail":       emails[s.UserID],
			"mediaId":         s.MediaID,
			"title":           s.Title,
			"libraryId":       s.LibraryID,
			"kind":            s.Kind,
			"delivery":        s.Delivery,
			"status":          s.Status,
			"durationSeconds": s.DurationSeconds,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"sessions": out})
}

func (h *AdminHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	maxLines := 400
	if raw := strings.TrimSpace(r.URL.Query().Get("lines")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 5000 {
			maxLines = n
		}
	}
	resp := map[string]any{
		"lines":  []string{},
		"source": "",
		"hint":   "",
	}
	path := strings.TrimSpace(h.LogFile)
	if path == "" {
		resp["hint"] = "Set PLUM_LOG_FILE so Plum writes JSON logs to a file this view can tail."
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}
	resp["source"] = path
	lines, err := tailLogFile(path, maxLines)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			resp["hint"] = "Log file does not exist yet; it is created when the server starts with PLUM_LOG_FILE set."
		} else {
			resp["hint"] = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
		return
	}
	resp["lines"] = lines
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func tailLogFile(path string, maxLines int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	const maxRead = 512 * 1024
	start := int64(0)
	if st.Size() > maxRead {
		start = st.Size() - maxRead
		if _, err := f.Seek(start, io.SeekStart); err != nil {
			return nil, err
		}
	}
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	if maxLines <= 0 {
		return nil, errors.New("tailLogFile: maxLines must be positive")
	}
	ring := make([]string, maxLines)
	var head, size int
	for sc.Scan() {
		line := sc.Text()
		if size < maxLines {
			ring[(head+size)%maxLines] = line
			size++
			continue
		}
		ring[head] = line
		head = (head + 1) % maxLines
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	out := make([]string, size)
	for i := 0; i < size; i++ {
		out[i] = ring[(head+i)%maxLines]
	}
	return out, nil
}

// StartAdminMaintenanceScheduler runs due scheduled tasks periodically.
func StartAdminMaintenanceScheduler(ctx context.Context, h *AdminHandler) {
	if ctx == nil || h == nil || h.DB == nil {
		return
	}
	ticker := time.NewTicker(time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.tickScheduledMaintenance(ctx)
			}
		}
	}()
}

func (h *AdminHandler) tickScheduledMaintenance(ctx context.Context) {
	s, err := db.GetAdminMaintenanceSchedule(h.DB)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	for _, task := range db.AllAdminMaintenanceTaskIDs {
		if !adminTaskDue(s, task, now) {
			continue
		}
		if task == db.AdminTaskOptimizeDatabase {
			h.vacuumMu.Lock()
			running := h.vacuumRunning
			h.vacuumMu.Unlock()
			if running {
				continue
			}
		}
		accepted, status, payload := h.runMaintenanceTask(ctx, task, false)
		if !accepted {
			slog.Debug("admin scheduled task skipped", "task", task, "status", status, "payload", payload)
			continue
		}
		slog.Info("admin scheduled task started", "task", task, "status", status)
	}
}

func adminTaskDue(s db.AdminMaintenanceSchedule, task db.AdminMaintenanceTaskID, now time.Time) bool {
	cfg := s.Tasks[task]
	if cfg.IntervalHours <= 0 {
		return false
	}
	lastRaw := s.LastRun[task]
	if lastRaw == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, lastRaw)
	if err != nil {
		return true
	}
	return now.Sub(last) >= time.Duration(cfg.IntervalHours)*time.Hour
}

func (h *AdminHandler) PostRegenerateThumbnails(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.ScanJobs == nil || h.DB == nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	libraryID := 0
	if q := strings.TrimSpace(r.URL.Query().Get("libraryId")); q != "" {
		v, err := strconv.Atoi(q)
		if err != nil || v < 0 {
			http.Error(w, "invalid libraryId", http.StatusBadRequest)
			return
		}
		libraryID = v
	} else {
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var body struct {
			LibraryID int `json:"libraryId"`
		}
		if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		libraryID = body.LibraryID
	}
	if libraryID == 0 {
		ids, err := db.ListTVAndAnimeLibraryIDs(h.DB)
		if err != nil {
			http.Error(w, "failed to list libraries", http.StatusInternalServerError)
			return
		}
		for _, id := range ids {
			go h.ScanJobs.StartThumbnails(id)
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}
	var libType string
	if err := h.DB.QueryRow(`SELECT type FROM libraries WHERE id = ?`, libraryID).Scan(&libType); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load library", http.StatusInternalServerError)
		return
	}
	if libType != db.LibraryTypeTV && libType != db.LibraryTypeAnime {
		http.Error(w, "library type does not support episode thumbnails", http.StatusBadRequest)
		return
	}
	go h.ScanJobs.StartThumbnails(libraryID)
	w.WriteHeader(http.StatusAccepted)
}

// MountAdminRoutes registers /api/admin/* on the given router (caller must apply RequireAdmin).
func MountAdminRoutes(r chi.Router, h *AdminHandler) {
	if h == nil {
		return
	}
	r.Get("/api/admin/maintenance/schedule", h.GetMaintenanceSchedule)
	r.Put("/api/admin/maintenance/schedule", h.PutMaintenanceSchedule)
	r.Post("/api/admin/maintenance/run", h.PostMaintenanceRun)
	r.Post("/api/admin/thumbnails/regenerate", h.PostRegenerateThumbnails)
	r.Get("/api/admin/playback/active", h.GetActivePlayback)
	r.Get("/api/admin/logs", h.GetLogs)
}
