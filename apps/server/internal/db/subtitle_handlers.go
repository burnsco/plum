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

	"plum/internal/ffopts"
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
		return streamFFmpegWebVTT(
			w,
			r,
			[]string{"-i", s.Path, "-f", "webvtt", "-"}...,
		)
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
	err = streamFFmpegWebVTT(
		w,
		r,
		[]string{"-i", sourcePath, "-map", fmt.Sprintf("0:%d", streamIndex), "-f", "webvtt", "-"}...,
	)
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
		"stream embedded subtitle served media=%d stream=%d source=%q duration=%s",
		mediaID,
		streamIndex,
		sourcePath,
		time.Since(startedAt).Round(time.Millisecond),
	)
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

// trimFFmpegStderrProgress drops encoding progress lines ffmpeg writes to stderr so error
// summaries are not dominated by the last 512 bytes of "size=... time=... speed=..." spam.
func trimFFmpegStderrProgress(raw string) string {
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "size=") && strings.Contains(t, "time=") && strings.Contains(t, "bitrate=") {
			continue
		}
		if strings.HasPrefix(t, "frame=") && strings.Contains(t, "fps=") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return strings.TrimSpace(raw)
	}
	return out
}

// streamFFmpegWebVTT runs ffmpeg with stdout wired to the response so the client receives bytes
// while conversion runs (long embedded/sidecar extracts no longer sit behind a full-memory buffer).
func streamFFmpegWebVTT(w http.ResponseWriter, r *http.Request, args ...string) error {
	w.Header().Set("Content-Type", "text/vtt")
	// -nostats keeps stderr usable on failure (we were truncating to a tail that was often progress only).
	ffmpegArgs := []string{"-hide_banner", "-nostats", "-loglevel", "warning"}
	ffmpegArgs = append(ffmpegArgs, ffopts.InputProbeBeforeI...)
	ffmpegArgs = append(ffmpegArgs, args...)
	cmd := exec.CommandContext(r.Context(), "ffmpeg", ffmpegArgs...)
	cmd.Stdout = w
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		msg = trimFFmpegStderrProgress(msg)
		if msg == "" {
			msg = err.Error()
		}
		if len(msg) > 512 {
			msg = msg[len(msg)-512:]
		}
		log.Printf("ffmpeg subtitle stream stderr_tail=%s", msg)
		return fmt.Errorf("ffmpeg error: %s", msg)
	}
	return nil
}
