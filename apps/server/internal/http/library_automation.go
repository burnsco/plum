package httpapi

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"plum/internal/db"
)

type libraryAutomationConfig struct {
	LibraryID           int
	Path                string
	Type                string
	WatcherEnabled      bool
	WatcherMode         string
	ScanIntervalMinutes int
}

type libraryPollEntry struct {
	IsDir   bool
	ModTime int64
	Size    int64
}

const (
	libraryPollWatchInterval = 30 * time.Second
	libraryRetryBaseDelay    = 5 * time.Second
)

func loadLibraryAutomationConfigs(dbConn *sql.DB) ([]libraryAutomationConfig, error) {
	rows, err := dbConn.Query(
		`SELECT id, path, type, COALESCE(watcher_enabled, 0), COALESCE(watcher_mode, 'auto'), COALESCE(scan_interval_minutes, 0)
		   FROM libraries`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []libraryAutomationConfig
	for rows.Next() {
		var cfg libraryAutomationConfig
		var watcherEnabled int
		if err := rows.Scan(
			&cfg.LibraryID,
			&cfg.Path,
			&cfg.Type,
			&watcherEnabled,
			&cfg.WatcherMode,
			&cfg.ScanIntervalMinutes,
		); err != nil {
			return nil, err
		}
		cfg.WatcherEnabled = watcherEnabled != 0
		if cfg.WatcherMode == "" {
			cfg.WatcherMode = "auto"
		}
		out = append(out, cfg)
	}
	return out, rows.Err()
}

func (m *LibraryScanManager) ConfigureLibraryAutomation(
	libraryID int,
	path string,
	libraryType string,
	watcherEnabled bool,
	watcherMode string,
	scanIntervalMinutes int,
) {
	m.mu.Lock()
	m.types[libraryID] = libraryType
	m.paths[libraryID] = path
	if watcherMode == "" {
		watcherMode = "auto"
	}
	if _, ok := m.jobs[libraryID]; !ok {
		m.jobs[libraryID] = libraryScanStatus{
			LibraryID:     libraryID,
			Phase:         libraryScanPhaseIdle,
			IdentifyPhase: libraryIdentifyPhaseIdle,
			MaxRetries:    3,
		}
	}
	status := m.jobs[libraryID]
	if scanIntervalMinutes > 0 {
		status.NextScheduledAt = time.Now().UTC().Add(time.Duration(scanIntervalMinutes) * time.Minute).Format(time.RFC3339)
	} else {
		status.NextScheduledAt = ""
	}
	m.jobs[libraryID] = status
	if stop, ok := m.watcherStops[libraryID]; ok {
		stop()
		delete(m.watcherStops, libraryID)
	}
	if stop, ok := m.schedulerStops[libraryID]; ok {
		stop()
		delete(m.schedulerStops, libraryID)
	}
	m.mu.Unlock()
	m.flushStatus(libraryID, true)

	if watcherEnabled {
		if watcherMode == "poll" {
			m.startPollingWatcher(libraryID, path, libraryType)
		} else if err := m.startFSWatcher(libraryID, path, libraryType); err != nil {
			m.startPollingWatcher(libraryID, path, libraryType)
		}
	}
	if scanIntervalMinutes > 0 {
		m.startScheduler(libraryID, path, libraryType, scanIntervalMinutes)
	}
}

func (m *LibraryScanManager) startPollingWatcher(libraryID int, path, libraryType string) {
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.watcherStops[libraryID] = cancel
	m.mu.Unlock()
	go func() {
		previousSnapshot, err := snapshotLibraryPollState(path)
		if err != nil {
			previousSnapshot = nil
		}
		ticker := time.NewTicker(libraryPollWatchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				currentSnapshot, err := snapshotLibraryPollState(path)
				if err != nil {
					previousSnapshot = nil
					continue
				}
				if previousSnapshot == nil {
					previousSnapshot = currentSnapshot
					continue
				}
				changedPaths := detectLibraryPollChanges(path, previousSnapshot, currentSnapshot)
				previousSnapshot = currentSnapshot
				for _, changedPath := range changedPaths {
					m.queueAutomatedScan(libraryID, path, libraryType, changedPath)
				}
			}
		}
	}()
}

func (m *LibraryScanManager) startScheduler(libraryID int, path, libraryType string, intervalMinutes int) {
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.schedulerStops[libraryID] = cancel
	m.mu.Unlock()
	go func() {
		interval := time.Duration(intervalMinutes) * time.Minute
		timer := time.NewTimer(interval)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				m.start(libraryID, path, libraryType, false, nil)
				m.mu.Lock()
				if status, ok := m.jobs[libraryID]; ok {
					status.NextScheduledAt = time.Now().UTC().Add(interval).Format(time.RFC3339)
					m.jobs[libraryID] = status
				}
				m.mu.Unlock()
				m.flushStatus(libraryID, true)
				timer.Reset(interval)
			}
		}
	}()
}

func (m *LibraryScanManager) startFSWatcher(libraryID int, path, libraryType string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err := addWatchDirs(watcher, path); err != nil {
		_ = watcher.Close()
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.watcherStops[libraryID] = func() {
		cancel()
		_ = watcher.Close()
	}
	m.mu.Unlock()
	go func() {
		defer watcher.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Create) != 0 {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						_ = addWatchDirs(watcher, event.Name)
					}
				}
				if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
					continue
				}
				m.queueAutomatedScan(libraryID, path, libraryType, event.Name)
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()
	return nil
}

func addWatchDirs(watcher *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		return watcher.Add(path)
	})
}

func snapshotLibraryPollState(root string) (map[string]libraryPollEntry, error) {
	snapshot := make(map[string]libraryPollEntry)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.Clean(rel)
		if rel == "." {
			return nil
		}
		snapshot[rel] = libraryPollEntry{
			IsDir:   d.IsDir(),
			ModTime: info.ModTime().UTC().UnixNano(),
			Size:    info.Size(),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

func detectLibraryPollChanges(root string, previous, current map[string]libraryPollEntry) []string {
	if len(previous) == 0 && len(current) == 0 {
		return nil
	}
	changed := make(map[string]struct{})
	for rel, entry := range current {
		if previousEntry, ok := previous[rel]; ok && previousEntry == entry {
			continue
		}
		changed[filepath.Join(root, rel)] = struct{}{}
	}
	for rel := range previous {
		if _, ok := current[rel]; ok {
			continue
		}
		changed[filepath.Join(root, rel)] = struct{}{}
	}
	if len(changed) == 0 {
		return nil
	}
	out := make([]string, 0, len(changed))
	for path := range changed {
		out = append(out, path)
	}
	return out
}

// tvAutomationDiscoverySubpath returns the first path segment under the library root (series folder)
// so discovery and mark-missing cover sibling season directories after renames. When this returns
// false, callers should fall back to path-based subpaths or a full-library scan.
func tvAutomationDiscoverySubpath(rel string) (subpath string, ok bool) {
	rel = filepath.Clean(rel)
	if rel == "." || rel == "" {
		return "", false
	}
	var parts []string
	for _, p := range strings.Split(rel, string(filepath.Separator)) {
		if p != "" && p != "." {
			parts = append(parts, p)
		}
	}
	if len(parts) < 2 {
		return "", false
	}
	return parts[0], true
}

func (m *LibraryScanManager) queueAutomatedScan(libraryID int, root, libraryType, eventPath string) {
	m.tryMarkImmediateMissingFromPaths(libraryID, root, []string{eventPath})
	// Match default manual scan behavior (identify != false): new hardlinks / *arr imports
	// should get TMDB matching as soon as discovery sees them. Music skips identify when
	// no music identifier is configured (see LibraryScanManager.start).
	autoIdentify := true
	rel, err := filepath.Rel(root, eventPath)
	if err != nil {
		m.start(libraryID, root, libraryType, autoIdentify, nil)
		return
	}
	rel = filepath.Clean(rel)
	if rel == "." || rel == "" || strings.HasPrefix(rel, "..") {
		m.start(libraryID, root, libraryType, autoIdentify, nil)
		return
	}
	subpath := rel
	if info, statErr := os.Stat(eventPath); statErr != nil || !info.IsDir() {
		subpath = filepath.Dir(rel)
	}
	if (libraryType == db.LibraryTypeTV || libraryType == db.LibraryTypeAnime) && subpath != "." && subpath != "" {
		if showRoot, wide := tvAutomationDiscoverySubpath(rel); wide {
			subpath = showRoot
		}
	}
	if subpath == "." || subpath == "" {
		m.start(libraryID, root, libraryType, autoIdentify, nil)
		return
	}
	m.start(libraryID, root, libraryType, autoIdentify, []string{subpath})
}

func retryableLibraryError(errText string) bool {
	errText = strings.ToLower(strings.TrimSpace(errText))
	if errText == "" {
		return false
	}
	switch {
	case strings.Contains(errText, "path is required"),
		strings.Contains(errText, "path not found"),
		strings.Contains(errText, "path is not a directory"),
		strings.Contains(errText, "invalid scan subpath"),
		strings.Contains(errText, "forbidden"),
		strings.Contains(errText, "unauthorized"):
		return false
	default:
		return true
	}
}

func (m *LibraryScanManager) scheduleRetry(libraryID int, retryIdentify bool, errText string) bool {
	m.mu.Lock()
	status, ok := m.jobs[libraryID]
	if !ok {
		m.mu.Unlock()
		return false
	}
	if !retryableLibraryError(errText) {
		status.LastError = errText
		m.jobs[libraryID] = status
		m.mu.Unlock()
		m.flushStatus(libraryID, true)
		return false
	}
	if status.MaxRetries <= 0 {
		status.MaxRetries = 3
	}
	if status.RetryCount >= status.MaxRetries {
		status.LastError = errText
		m.jobs[libraryID] = status
		m.mu.Unlock()
		m.flushStatus(libraryID, true)
		return false
	}
	status.RetryCount++
	delay := time.Duration(status.RetryCount) * libraryRetryBaseDelay
	status.NextRetryAt = time.Now().UTC().Add(delay).Format(time.RFC3339)
	status.LastError = errText
	status.Error = errText
	m.jobs[libraryID] = status
	if timer, ok := m.retryTimers[libraryID]; ok {
		timer.Stop()
	}
	m.retryTimers[libraryID] = time.AfterFunc(delay, func() {
		m.mu.Lock()
		delete(m.retryTimers, libraryID)
		status, ok := m.jobs[libraryID]
		if ok {
			status.NextRetryAt = ""
			status.Error = ""
			m.jobs[libraryID] = status
		}
		path := m.paths[libraryID]
		libraryType := m.types[libraryID]
		subpaths := append([]string(nil), m.subpaths[libraryID]...)
		m.mu.Unlock()
		m.flushStatus(libraryID, true)
		if retryIdentify {
			m.startIdentify(libraryID)
			return
		}
		m.start(libraryID, path, libraryType, status.IdentifyRequested, subpaths)
	})
	m.mu.Unlock()
	m.flushStatus(libraryID, true)
	return true
}
