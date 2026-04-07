package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// ThumbnailWorkerCount limits concurrent ffmpeg thumbnail extractions in batch jobs.
const ThumbnailWorkerCount = 2

var onDemandThumbSem = make(chan struct{}, 3)

// ThumbnailTask is one episode that needs a thumbnail (global media id + file path for logging/UI).
type ThumbnailTask struct {
	GlobalID int
	Path     string
}

// ListTVAndAnimeLibraryIDs returns library ids whose type is TV or anime.
func ListTVAndAnimeLibraryIDs(dbConn *sql.DB) ([]int, error) {
	rows, err := dbConn.Query(`SELECT id FROM libraries WHERE type IN ('tv', 'anime') ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ListEpisodesMissingThumbnails returns TV and anime episodes that have no thumbnail yet.
// Rows must be present in media_global. Missing-on-disk episodes (missing_since set) are excluded.
// If libraryID is 0, all libraries are included; otherwise only that library.
func ListEpisodesMissingThumbnails(ctx context.Context, dbConn *sql.DB, libraryID int) ([]ThumbnailTask, error) {
	const q = `
SELECT g.id, t.path FROM tv_episodes t
INNER JOIN media_global g ON g.kind = 'tv' AND g.ref_id = t.id
WHERE COALESCE(t.missing_since, '') = ''
  AND COALESCE(t.thumbnail_path, '') = ''
  AND ((? = 0) OR (t.library_id = ?))
UNION ALL
SELECT g.id, t.path FROM anime_episodes t
INNER JOIN media_global g ON g.kind = 'anime' AND g.ref_id = t.id
WHERE COALESCE(t.missing_since, '') = ''
  AND COALESCE(t.thumbnail_path, '') = ''
  AND ((? = 0) OR (t.library_id = ?))
ORDER BY 1`
	rows, err := dbConn.QueryContext(ctx, q, libraryID, libraryID, libraryID, libraryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []ThumbnailTask
	for rows.Next() {
		var t ThumbnailTask
		if err := rows.Scan(&t.GlobalID, &t.Path); err != nil {
			return nil, err
		}
		if t.Path == "" {
			continue
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ThumbnailBatchProgress is invoked after each task finishes (success or final failure after retries).
type ThumbnailBatchProgress func(completed, total int, task ThumbnailTask, err error)

// GenerateThumbnailsBatch generates JPEG thumbnails for the given tasks using a small worker pool.
// Each task is retried up to twice (three attempts) with a 1s delay between failures.
// Returns counts of successes and final failures. Respects ctx cancellation between tasks.
func GenerateThumbnailsBatch(ctx context.Context, dbConn *sql.DB, thumbDir string, tasks []ThumbnailTask, progressFn ThumbnailBatchProgress) (generated, failed int) {
	total := len(tasks)
	if total == 0 {
		return 0, 0
	}
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		slog.Warn("thumbnail batch mkdir", "dir", thumbDir, "error", err)
		for i, task := range tasks {
			if progressFn != nil {
				progressFn(i+1, total, task, err)
			}
			failed++
		}
		return 0, failed
	}

	jobs := make(chan ThumbnailTask)
	var wg sync.WaitGroup
	var countMu sync.Mutex
	completed := 0
	gen := 0
	fail := 0

	workerCount := ThumbnailWorkerCount
	if workerCount > total {
		workerCount = total
	}
	if workerCount < 1 {
		workerCount = 1
	}

	report := func(task ThumbnailTask, taskErr error) {
		countMu.Lock()
		completed++
		c := completed
		if taskErr != nil {
			fail++
		} else {
			gen++
		}
		countMu.Unlock()
		if progressFn != nil {
			progressFn(c, total, task, taskErr)
		}
	}

	processOne := func(task ThumbnailTask) {
		var lastErr error
		defer func() { report(task, lastErr) }()
		for attempt := 0; attempt < 3; attempt++ {
			if ctx.Err() != nil {
				lastErr = ctx.Err()
				return
			}
			if attempt > 0 {
				select {
				case <-time.After(1 * time.Second):
				case <-ctx.Done():
					lastErr = ctx.Err()
					return
				}
			}
			item, err := GetMediaByID(dbConn, task.GlobalID)
			if err != nil {
				lastErr = err
				slog.Warn("thumbnail batch get media", "global_id", task.GlobalID, "error", err)
				continue
			}
			if item == nil {
				lastErr = ErrNotFound
				slog.Warn("thumbnail batch missing media row", "global_id", task.GlobalID)
				continue
			}
			sourcePath, err := ResolveMediaSourcePath(dbConn, *item)
			if err != nil {
				lastErr = err
				slog.Warn("thumbnail batch resolve path", "global_id", task.GlobalID, "error", err)
				continue
			}
			relPath := fmt.Sprintf("%d.jpg", task.GlobalID)
			absPath := filepath.Join(thumbDir, relPath)
			if err := GenerateThumbnail(ctx, sourcePath, absPath); err != nil {
				lastErr = err
				slog.Warn("thumbnail batch ffmpeg", "global_id", task.GlobalID, "error", err)
				continue
			}
			if err := UpdateThumbnailPath(dbConn, task.GlobalID, relPath); err != nil {
				_ = os.Remove(absPath)
				lastErr = err
				slog.Warn("thumbnail batch update db", "global_id", task.GlobalID, "error", err)
				continue
			}
			lastErr = nil
			return
		}
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range jobs {
				if ctx.Err() != nil {
					return
				}
				processOne(task)
			}
		}()
	}

	for _, task := range tasks {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return gen, fail
		case jobs <- task:
		}
	}
	close(jobs)
	wg.Wait()
	return gen, fail
}

// GenerateThumbnail extracts a single frame from the video at ~1 minute (or start if shorter) and writes it to outputPath as JPEG.
func GenerateThumbnail(ctx context.Context, videoPath, outputPath string) error {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-ss", "10", "-i", videoPath, "-vframes", "1", "-q:v", "2", outputPath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg thumbnail: %w", err)
	}
	return nil
}

// UpdateThumbnailPath sets thumbnail_path on the category row for the given global media ID.
func UpdateThumbnailPath(dbConn *sql.DB, globalID int, relativePath string) error {
	var kind string
	var refID int
	err := dbConn.QueryRow(`SELECT kind, ref_id FROM media_global WHERE id = ?`, globalID).Scan(&kind, &refID)
	if err != nil {
		return err
	}
	table := mediaTableForKind(kind)
	if table != "tv_episodes" && table != "anime_episodes" {
		return fmt.Errorf("thumbnail only supported for tv/anime, got %s", kind)
	}
	_, err = dbConn.Exec(`UPDATE `+table+` SET thumbnail_path = ? WHERE id = ?`, relativePath, refID)
	return err
}

// HandleServeThumbnail serves the thumbnail image for a media item, generating it on demand if missing.
func HandleServeThumbnail(w http.ResponseWriter, r *http.Request, dbConn *sql.DB, globalID int, thumbDir string) error {
	item, err := GetMediaByID(dbConn, globalID)
	if err != nil || item == nil {
		return ErrNotFound
	}
	if item.Type == "music" || item.Type == "movie" {
		return ErrNotFound
	}
	relPath := fmt.Sprintf("%d.jpg", globalID)
	absPath := filepath.Join(thumbDir, relPath)
	if item.ThumbnailPath != "" {
		existing := filepath.Join(thumbDir, item.ThumbnailPath)
		if _, err := os.Stat(existing); err == nil {
			w.Header().Set("Content-Type", "image/jpeg")
			http.ServeFile(w, r, existing)
			return nil
		}
	}
	if err := os.MkdirAll(thumbDir, 0o755); err != nil {
		return fmt.Errorf("mkdir thumbnails: %w", err)
	}
	sourcePath, err := ResolveMediaSourcePath(dbConn, *item)
	if err != nil {
		return err
	}
	select {
	case onDemandThumbSem <- struct{}{}:
	case <-r.Context().Done():
		return r.Context().Err()
	}
	defer func() { <-onDemandThumbSem }()
	if err := GenerateThumbnail(r.Context(), sourcePath, absPath); err != nil {
		slog.Warn("generate thumbnail", "media_id", globalID, "error", err)
		return fmt.Errorf("thumbnail generation failed: %w", err)
	}
	if err := UpdateThumbnailPath(dbConn, globalID, relPath); err != nil {
		_ = os.Remove(absPath)
		return err
	}
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeFile(w, r, absPath)
	return nil
}
