package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

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
	if err := GenerateThumbnail(r.Context(), sourcePath, absPath); err != nil {
		log.Printf("generate thumbnail for media %d: %v", globalID, err)
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
