package transcoder

import (
	"os"
	"path/filepath"
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
