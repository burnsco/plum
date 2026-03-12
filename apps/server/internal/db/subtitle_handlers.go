package db

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

// HandleStreamSubtitle looks up a subtitle and serves it as VTT.
func HandleStreamSubtitle(w http.ResponseWriter, r *http.Request, dbConn *sql.DB, id int) error {
	s, err := GetSubtitleByID(dbConn, id)
	if err != nil {
		return err
	}
	if s == nil {
		return ErrNotFound
	}

	if s.Format == "vtt" {
		w.Header().Set("Content-Type", "text/vtt")
		http.ServeFile(w, r, s.Path)
		return nil
	}

	if s.Format == "srt" || s.Format == "ass" || s.Format == "ssa" {
		out, err := convertSubtitleToVTT(r.Context(), []string{"-i", s.Path, "-f", "webvtt", "-"}...)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "text/vtt")
		if _, err := w.Write(out); err != nil {
			return err
		}
		return nil
	}

	return fmt.Errorf("unsupported subtitle format: %s", s.Format)
}

// HandleStreamEmbeddedSubtitle extracts an embedded subtitle stream and serves it as VTT.
func HandleStreamEmbeddedSubtitle(w http.ResponseWriter, r *http.Request, dbConn *sql.DB, mediaID int, streamIndex int) error {
	item, err := GetMediaByID(dbConn, mediaID)
	if err != nil {
		return err
	}
	if item == nil {
		return ErrNotFound
	}
	sourcePath, err := ResolveMediaSourcePath(dbConn, *item)
	if err != nil {
		return err
	}
	out, err := convertSubtitleToVTT(r.Context(), []string{"-i", sourcePath, "-map", fmt.Sprintf("0:%d", streamIndex), "-f", "webvtt", "-"}...)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/vtt")
	if _, err := w.Write(out); err != nil {
		return err
	}
	return nil
}

func convertSubtitleToVTT(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		if len(msg) > 512 {
			msg = msg[len(msg)-512:]
		}
		return nil, fmt.Errorf("ffmpeg error: %s", msg)
	}
	return stdout.Bytes(), nil
}
