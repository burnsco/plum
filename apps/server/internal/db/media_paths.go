package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveMediaSourcePath returns the on-disk path for a media item.
// Absolute paths are used as-is. Relative paths are resolved against the library root.
func ResolveMediaSourcePath(dbConn *sql.DB, item MediaItem) (string, error) {
	rawPath := strings.TrimSpace(item.Path)
	if rawPath == "" {
		return "", fmt.Errorf("media path is empty: %w", ErrNotFound)
	}

	cleanPath := filepath.Clean(rawPath)
	candidates := make([]string, 0, 2)
	if filepath.IsAbs(cleanPath) {
		candidates = append(candidates, cleanPath)
	} else {
		libraryRoot, err := lookupLibraryPath(dbConn, item.LibraryID)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(libraryRoot) != "" {
			candidates = append(candidates, filepath.Join(libraryRoot, cleanPath))
		}
		candidates = append(candidates, cleanPath)
	}

	for _, candidate := range uniquePaths(candidates) {
		info, err := os.Stat(candidate)
		switch {
		case err == nil && info.IsDir():
			return "", fmt.Errorf("media path points to a directory: %s", candidate)
		case err == nil:
			return candidate, nil
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return "", fmt.Errorf("stat media path %q: %w", candidate, err)
		}
	}

	return "", fmt.Errorf(
		"media file not found on disk: %s. Rescan the library or verify the library path is mounted in the server/container: %w",
		cleanPath,
		ErrNotFound,
	)
}

func lookupLibraryPath(dbConn *sql.DB, libraryID int) (string, error) {
	var libraryPath string
	err := dbConn.QueryRow(`SELECT path FROM libraries WHERE id = ?`, libraryID).Scan(&libraryPath)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("library %d not found: %w", libraryID, ErrNotFound)
	}
	if err != nil {
		return "", err
	}
	return libraryPath, nil
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}
