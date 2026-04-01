package db

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
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
	if !hasEmbeddedSubtitleStream(*item, streamIndex) {
		return fmt.Errorf("embedded subtitle stream %d not found for media %d: %w", streamIndex, mediaID, ErrNotFound)
	}
	sourcePath, err := ResolveMediaSourcePath(dbConn, *item)
	if err != nil {
		return err
	}
	if subtitle, err := resolveEmbeddedSubtitleForPlayback(r.Context(), sourcePath, *item, streamIndex); err == nil {
		if subtitle.Supported != nil && !*subtitle.Supported {
			codec := subtitle.Codec
			if codec == "" {
				codec = "unknown"
			}
			return &StatusError{
				Status:  http.StatusUnprocessableEntity,
				Message: fmt.Sprintf("embedded subtitle codec %q is not supported for web playback", codec),
			}
		}
	}
	startedAt := time.Now()
	out, err := convertSubtitleToVTT(r.Context(), []string{"-i", sourcePath, "-map", fmt.Sprintf("0:%d", streamIndex), "-f", "webvtt", "-"}...)
	if err != nil {
		log.Printf(
			"stream embedded subtitle failed media=%d stream=%d source=%q duration=%s error=%v",
			mediaID,
			streamIndex,
			sourcePath,
			time.Since(startedAt).Round(time.Millisecond),
			err,
		)
		return err
	}
	log.Printf(
		"stream embedded subtitle served media=%d stream=%d source=%q duration=%s bytes=%d",
		mediaID,
		streamIndex,
		sourcePath,
		time.Since(startedAt).Round(time.Millisecond),
		len(out),
	)
	w.Header().Set("Content-Type", "text/vtt")
	if _, err := w.Write(out); err != nil {
		return err
	}
	return nil
}

func hasEmbeddedSubtitleStream(item MediaItem, streamIndex int) bool {
	return findEmbeddedSubtitleStream(item.EmbeddedSubtitles, streamIndex) != nil
}

func findEmbeddedSubtitleStream(subtitles []EmbeddedSubtitle, streamIndex int) *EmbeddedSubtitle {
	for i := range subtitles {
		if subtitles[i].StreamIndex == streamIndex {
			return &subtitles[i]
		}
	}
	return nil
}

func resolveEmbeddedSubtitleForPlayback(ctx context.Context, sourcePath string, item MediaItem, streamIndex int) (*EmbeddedSubtitle, error) {
	probed, err := readVideoMetadata(ctx, sourcePath)
	if err == nil {
		if subtitle := findEmbeddedSubtitleStream(probed.EmbeddedSubtitles, streamIndex); subtitle != nil {
			return subtitle, nil
		}
	}
	return findEmbeddedSubtitleStream(item.EmbeddedSubtitles, streamIndex), err
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
