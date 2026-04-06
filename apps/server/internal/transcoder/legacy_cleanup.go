package transcoder

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func CleanupLegacyTranscodes(root string) error {
	matches, err := filepath.Glob(filepath.Join(root, "plum_transcoded_*.mp4"))
	if err != nil {
		return err
	}
	for _, match := range matches {
		if err := os.Remove(match); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// SessionDirCleanupMinAgeAny is the maxAge value that removes every directory under root (ignores
// mtime). Only safe at startup before live sessions use that root.
const SessionDirCleanupMinAgeAny = time.Duration(0)

// CleanupOrphanedSessionDirs removes session temp directories that are older
// than maxAge. It is safe to call at startup before any sessions exist, in
// which case all directories in root are removed regardless of age.
func CleanupOrphanedSessionDirs(root string, maxAge time.Duration) error {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-maxAge)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if maxAge <= 0 || info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(root, entry.Name()))
		}
	}
	return nil
}

// CleanStaleTranscodeSessionDirs removes directories under playbackRoot that are older than
// minAge and whose names are not listed in activeSessionIDs (session folder names match session IDs).
func CleanStaleTranscodeSessionDirs(playbackRoot string, activeSessionIDs map[string]struct{}, minAge time.Duration) (removed int, err error) {
	if playbackRoot == "" || minAge <= 0 {
		return 0, nil
	}
	entries, err := os.ReadDir(playbackRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	cutoff := time.Now().Add(-minAge)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if _, active := activeSessionIDs[name]; active {
			continue
		}
		info, ierr := entry.Info()
		if ierr != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if rerr := os.RemoveAll(filepath.Join(playbackRoot, name)); rerr != nil && !os.IsNotExist(rerr) {
			return removed, rerr
		}
		removed++
	}
	return removed, nil
}

// CleanStaleLegacyTranscodes deletes plum_transcoded_*.mp4 files under root when older than minAge.
func CleanStaleLegacyTranscodes(tmpRoot string, minAge time.Duration) (removed int, err error) {
	if tmpRoot == "" || minAge <= 0 {
		return 0, nil
	}
	matches, err := filepath.Glob(filepath.Join(tmpRoot, "plum_transcoded_*.mp4"))
	if err != nil {
		return 0, err
	}
	cutoff := time.Now().Add(-minAge)
	for _, match := range matches {
		info, statErr := os.Stat(match)
		if statErr != nil {
			continue
		}
		if info.ModTime().After(cutoff) {
			continue
		}
		if rmErr := os.Remove(match); rmErr != nil && !os.IsNotExist(rmErr) {
			return removed, rmErr
		}
		removed++
	}
	return removed, nil
}

// CleanOldLogFiles removes regular files in logDir with extension .log that are older than maxAge.
func CleanOldLogFiles(logDir string, maxAge time.Duration) (removed int, err error) {
	if logDir == "" || maxAge <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-maxAge)
	err = filepath.WalkDir(logDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".log") {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		if info.ModTime().After(cutoff) {
			return nil
		}
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			return rmErr
		}
		removed++
		return nil
	})
	if err != nil {
		return removed, err
	}
	return removed, nil
}
