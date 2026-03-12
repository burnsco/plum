package transcoder

import (
	"os"
	"path/filepath"
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
