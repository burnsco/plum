package db

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		// Sidecar file is tied to library path; reuse across playback sessions (If-None-Match via ServeFile).
		w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
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
	// Bitmap / unsupported subs: reject before cache locks or ffmpeg (matches HLS + session filtering).
	if stored := findEmbeddedSubtitleStream(item.EmbeddedSubtitles, streamIndex); stored != nil {
		if !EmbeddedSubtitleWebVTTDeliveryEligible(*stored) {
			codec := stored.Codec
			if codec == "" {
				codec = "unknown"
			}
			return &StatusError{
				Status:  http.StatusUnprocessableEntity,
				Message: fmt.Sprintf("embedded subtitle codec %q cannot be delivered as WebVTT", codec),
			}
		}
	}
	codec := ""
	if stored := findEmbeddedSubtitleStream(item.EmbeddedSubtitles, streamIndex); stored != nil {
		codec = stored.Codec
	}

	cachePath, cacheErr := embeddedSubtitleVTTCachePath(sourcePath, streamIndex)
	if cacheErr == nil && tryServeEmbeddedSubtitleFromCache(w, r, cachePath) {
		slog.Debug("embedded subtitle cache hit", "media_id", mediaID, "stream_index", streamIndex)
		return nil
	}

	lockKey := cachePath
	if lockKey == "" {
		lockKey = fmt.Sprintf("%s|%d", sourcePath, streamIndex)
	}
	mu := lockEmbeddedSubtitle(lockKey)
	mu.Lock()
	defer mu.Unlock()

	if cacheErr == nil && tryServeEmbeddedSubtitleFromCache(w, r, cachePath) {
		slog.Debug("embedded subtitle cache hit after lock", "media_id", mediaID, "stream_index", streamIndex)
		return nil
	}

	var teeFile *os.File
	var tee io.Writer
	partialPath := ""
	if cacheErr == nil && cachePath != "" {
		if mkErr := os.MkdirAll(filepath.Dir(cachePath), 0o755); mkErr == nil {
			p := cachePath + ".partial"
			f, oerr := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if oerr == nil {
				partialPath = p
				teeFile = f
				tee = f
			} else {
				slog.Warn("embedded subtitle cache open partial", "error", oerr)
			}
		}
	}

	startedAt := time.Now()
	transcodeErr := transcodeEmbeddedSubtitleToWebVTT(w, r, tee, sourcePath, streamIndex, codec, mediaID, startedAt)

	if teeFile != nil {
		_ = teeFile.Close()
	}
	if transcodeErr != nil {
		if partialPath != "" {
			_ = os.Remove(partialPath)
		}
		return transcodeErr
	}
	if partialPath != "" && cachePath != "" {
		if ren := os.Rename(partialPath, cachePath); ren != nil {
			slog.Warn("embedded subtitle cache rename", "error", ren)
			_ = os.Remove(partialPath)
		}
	}
	return nil
}

// HandleStreamEmbeddedSubtitleSup demuxes one Blu-ray / HDMV PGS stream to raw PG (Blu-ray .sup)
// for clients that decode MimeTypes.APPLICATION_PGS (Media3 ExoPlayer on Android TV; Jellyfin "pgssub").
func HandleStreamEmbeddedSubtitleSup(w http.ResponseWriter, r *http.Request, dbConn *sql.DB, mediaID int, streamIndex int) error {
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
	stored := findEmbeddedSubtitleStream(item.EmbeddedSubtitles, streamIndex)
	if stored == nil || !EmbeddedSubtitlePgsBinaryDeliveryEligible(*stored) {
		codec := "unknown"
		if stored != nil && strings.TrimSpace(stored.Codec) != "" {
			codec = stored.Codec
		}
		return &StatusError{
			Status:  http.StatusUnprocessableEntity,
			Message: fmt.Sprintf("embedded subtitle codec %q cannot be delivered as raw PGS", codec),
		}
	}
	sourcePath, err := ResolveMediaSourcePath(dbConn, *item)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/pgs")
	w.Header().Set("Cache-Control", "no-store")

	ffmpegArgs := []string{"-hide_banner", "-nostats", "-loglevel", "warning"}
	// Heavy Matroska probe: PGS streams after the first often fail with the light probe used for
	// temp demux (see ffopts.InputProbeSubtitleDemux). ExoPlayer then sees an empty/broken PGS track.
	ffmpegArgs = append(ffmpegArgs, ffopts.InputProbeEmbeddedExtract...)
	ffmpegArgs = append(ffmpegArgs,
		"-i", sourcePath,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-c", "copy",
		"-f", "sup",
		"-",
	)
	cmd := exec.CommandContext(r.Context(), "ffmpeg", ffmpegArgs...)
	cmd.Stdout = responseWriterForFFmpegStdout(w)
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
		slog.Warn("ffmpeg embedded pgs stream", "stderr_tail", msg)
		return fmt.Errorf("ffmpeg error: %s", msg)
	}
	return nil
}

// HandleStreamSubtitleAss serves a raw ASS/SSA sidecar file for clients that render ASS natively
// (e.g. JASSUB in web browsers). Only ASS and SSA format subtitles are eligible.
func HandleStreamSubtitleAss(w http.ResponseWriter, r *http.Request, dbConn *sql.DB, id int) error {
	s, err := GetSubtitleByID(dbConn, id)
	if err != nil {
		return err
	}
	if s == nil {
		return ErrNotFound
	}
	if s.Format != "ass" && s.Format != "ssa" {
		return &StatusError{
			Status:  http.StatusUnprocessableEntity,
			Message: fmt.Sprintf("subtitle format %q is not ASS/SSA", s.Format),
		}
	}
	w.Header().Set("Content-Type", "text/x-ssa; charset=utf-8")
	w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	http.ServeFile(w, r, s.Path)
	return nil
}

// HandleStreamEmbeddedSubtitleAss serves subtitles as ASS for clients that render ASS natively
// (e.g. JASSUB in web browsers). ASS/SSA streams are stream-copied; other text codecs are
// transcoded to ASS by ffmpeg.
func HandleStreamEmbeddedSubtitleAss(w http.ResponseWriter, r *http.Request, dbConn *sql.DB, mediaID int, streamIndex int) error {
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
	stored := findEmbeddedSubtitleStream(item.EmbeddedSubtitles, streamIndex)
	if stored == nil || !EmbeddedSubtitleAssDeliveryEligible(*stored) {
		codec := "unknown"
		if stored != nil && strings.TrimSpace(stored.Codec) != "" {
			codec = stored.Codec
		}
		return &StatusError{
			Status:  http.StatusUnprocessableEntity,
			Message: fmt.Sprintf("embedded subtitle codec %q cannot be delivered as raw ASS", codec),
		}
	}
	sourcePath, err := ResolveMediaSourcePath(dbConn, *item)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "text/x-ssa; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	ffmpegArgs := []string{"-hide_banner", "-nostats", "-loglevel", "warning"}
	ffmpegArgs = append(ffmpegArgs, ffopts.InputProbeSubtitleDemux...)
	codecLower := strings.ToLower(strings.TrimSpace(stored.Codec))
	streamCopy := codecLower == "ass" || codecLower == "ssa"
	ffmpegArgs = append(ffmpegArgs, "-i", sourcePath, "-map", fmt.Sprintf("0:%d", streamIndex))
	if streamCopy {
		ffmpegArgs = append(ffmpegArgs, "-c", "copy")
	} else {
		ffmpegArgs = append(ffmpegArgs, "-c:s", "ass")
	}
	ffmpegArgs = append(ffmpegArgs, "-f", "ass", "-")
	cmd := exec.CommandContext(r.Context(), "ffmpeg", ffmpegArgs...)
	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
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
		slog.Warn("ffmpeg embedded ass stream", "stderr_tail", msg)
		return fmt.Errorf("ffmpeg error: %s", msg)
	}
	if _, werr := w.Write(stdoutBuf.Bytes()); werr != nil {
		return werr
	}
	return nil
}

func transcodeEmbeddedSubtitleToWebVTT(
	w http.ResponseWriter,
	r *http.Request,
	tee io.Writer,
	sourcePath string,
	streamIndex int,
	codec string,
	mediaID int,
	startedAt time.Time,
) error {
	if demuxFmt, ok := subtitleDemuxFormat(codec); ok {
		tmpPath, cleanup, extractErr := extractEmbeddedSubtitleStreamToTemp(r.Context(), sourcePath, streamIndex, demuxFmt)
		if extractErr == nil {
			defer cleanup()
			err := streamFFmpegWebVTTWithOptionalTee(w, r, tee, []string{"-i", tmpPath, "-f", "webvtt", "-"}...)
			if err == nil {
				slog.Info("stream embedded subtitle served (demux+convert)",
					"media_id", mediaID,
					"stream_index", streamIndex,
					"source", sourcePath,
					"duration", time.Since(startedAt).Round(time.Millisecond).String(),
				)
				return nil
			}
			slog.Warn("embedded subtitle vtt convert failed after demux; trying direct transcode",
				"media_id", mediaID, "stream_index", streamIndex, "error", err)
		} else {
			slog.Warn("embedded subtitle demux extract failed; trying direct transcode",
				"media_id", mediaID, "stream_index", streamIndex, "codec", codec, "error", extractErr)
		}
	}
	ffmpegIn := append(append([]string{}, ffopts.InputProbeEmbeddedExtract...),
		"-i", sourcePath, "-map", fmt.Sprintf("0:%d", streamIndex), "-f", "webvtt", "-")
	err := streamFFmpegWebVTTWithOptionalTee(w, r, tee, ffmpegIn...)
	if err != nil {
		slog.Warn("stream embedded subtitle failed",
			"media_id", mediaID,
			"stream_index", streamIndex,
			"source", sourcePath,
			"duration", time.Since(startedAt).Round(time.Millisecond).String(),
			"error", err,
		)
		return err
	}
	slog.Info("stream embedded subtitle served (direct)",
		"media_id", mediaID,
		"stream_index", streamIndex,
		"source", sourcePath,
		"duration", time.Since(startedAt).Round(time.Millisecond).String(),
	)
	return nil
}

// ffmpegSubtitleTranscodeToWriter runs ffmpeg with stdout wired to out (disk cache materialization).
func ffmpegSubtitleTranscodeToWriter(ctx context.Context, out io.Writer, args ...string) error {
	ffmpegArgs := []string{"-hide_banner", "-nostats", "-loglevel", "warning"}
	ffmpegArgs = append(ffmpegArgs, args...)
	cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
	cmd.Stdout = out
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
		slog.Warn("ffmpeg subtitle cache materialize", "stderr_tail", msg)
		return fmt.Errorf("ffmpeg error: %s", msg)
	}
	return nil
}

func resetWriterFileForRetry(f *os.File) error {
	if err := f.Truncate(0); err != nil {
		return err
	}
	_, err := f.Seek(0, io.SeekStart)
	return err
}

// materializeEmbeddedSubtitleCacheFile writes a complete WebVTT for one embedded stream to cachePath
// (via partial + rename), using the same demux/direct strategy as transcodeEmbeddedSubtitleToWebVTT.
func materializeEmbeddedSubtitleCacheFile(ctx context.Context, sourcePath string, streamIndex int, codec string, cachePath string, mediaID int) error {
	partial := cachePath + ".partial"
	f, err := os.OpenFile(partial, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	abort := true
	defer func() {
		if f != nil {
			_ = f.Close()
		}
		if abort {
			_ = os.Remove(partial)
		}
	}()

	tryWrite := func(args ...string) error {
		return ffmpegSubtitleTranscodeToWriter(ctx, f, args...)
	}

	startedAt := time.Now()
	if demuxFmt, ok := subtitleDemuxFormat(codec); ok {
		tmpPath, cleanup, extractErr := extractEmbeddedSubtitleStreamToTemp(ctx, sourcePath, streamIndex, demuxFmt)
		if extractErr == nil {
			defer cleanup()
			convErr := tryWrite("-i", tmpPath, "-f", "webvtt", "-")
			if convErr == nil {
				slog.Info("subtitle cache warm (demux+convert)",
					"media_id", mediaID,
					"stream_index", streamIndex,
					"duration", time.Since(startedAt).Round(time.Millisecond).String(),
				)
				abort = false
				_ = f.Close()
				f = nil
				if ren := os.Rename(partial, cachePath); ren != nil {
					_ = os.Remove(partial)
					return ren
				}
				return nil
			}
			slog.Warn("subtitle cache warm vtt convert after demux failed; trying direct",
				"media_id", mediaID, "stream_index", streamIndex, "error", convErr)
		} else {
			slog.Warn("subtitle cache warm demux failed; trying direct",
				"media_id", mediaID, "stream_index", streamIndex, "codec", codec, "error", extractErr)
		}
	}
	if err := resetWriterFileForRetry(f); err != nil {
		return err
	}
	ffmpegIn := append(append([]string{}, ffopts.InputProbeEmbeddedExtract...),
		"-i", sourcePath, "-map", fmt.Sprintf("0:%d", streamIndex), "-f", "webvtt", "-")
	if err := tryWrite(ffmpegIn...); err != nil {
		return err
	}
	slog.Info("subtitle cache warm (direct)",
		"media_id", mediaID,
		"stream_index", streamIndex,
		"duration", time.Since(startedAt).Round(time.Millisecond).String(),
	)
	abort = false
	_ = f.Close()
	f = nil
	if ren := os.Rename(partial, cachePath); ren != nil {
		_ = os.Remove(partial)
		return ren
	}
	return nil
}

var warmEmbeddedSubtitleMediaMu sync.Mutex
var warmEmbeddedSubtitleMediaInFlight = make(map[int]struct{})

// WarmEmbeddedSubtitleCachesForMedia pre-materializes on-disk VTT caches for embedded subtitle tracks
// so the first client request often hits ServeFile instead of waiting on ffmpeg.
func WarmEmbeddedSubtitleCachesForMedia(ctx context.Context, dbConn *sql.DB, mediaID int) {
	if ctx == nil {
		ctx = context.Background()
	}

	warmEmbeddedSubtitleMediaMu.Lock()
	if _, busy := warmEmbeddedSubtitleMediaInFlight[mediaID]; busy {
		warmEmbeddedSubtitleMediaMu.Unlock()
		return
	}
	warmEmbeddedSubtitleMediaInFlight[mediaID] = struct{}{}
	warmEmbeddedSubtitleMediaMu.Unlock()
	defer func() {
		warmEmbeddedSubtitleMediaMu.Lock()
		delete(warmEmbeddedSubtitleMediaInFlight, mediaID)
		warmEmbeddedSubtitleMediaMu.Unlock()
	}()

	item, err := GetMediaByID(dbConn, mediaID)
	if err != nil || item == nil {
		return
	}
	sourcePath, err := ResolveMediaSourcePath(dbConn, *item)
	if err != nil {
		slog.Debug("subtitle cache warm skip", "media_id", mediaID, "error", err)
		return
	}
	for _, sub := range item.EmbeddedSubtitles {
		if !EmbeddedSubtitleWebVTTDeliveryEligible(sub) {
			continue
		}
		cachePath, cerr := embeddedSubtitleVTTCachePath(sourcePath, sub.StreamIndex)
		if cerr != nil {
			continue
		}
		if fi, statErr := os.Stat(cachePath); statErr == nil && fi.Size() > 0 {
			continue
		}
		lockKey := cachePath
		mu := lockEmbeddedSubtitle(lockKey)
		mu.Lock()
		if fi, statErr := os.Stat(cachePath); statErr == nil && fi.Size() > 0 {
			mu.Unlock()
			continue
		}
		if mkErr := os.MkdirAll(filepath.Dir(cachePath), 0o755); mkErr != nil {
			slog.Warn("subtitle cache warm mkdir", "media_id", mediaID, "stream_index", sub.StreamIndex, "error", mkErr)
			mu.Unlock()
			continue
		}
		if matErr := materializeEmbeddedSubtitleCacheFile(ctx, sourcePath, sub.StreamIndex, sub.Codec, cachePath, mediaID); matErr != nil {
			slog.Warn("subtitle cache warm failed", "media_id", mediaID, "stream_index", sub.StreamIndex, "error", matErr)
		}
		mu.Unlock()
	}
}

var embeddedSubtitleLocks sync.Map // key string -> *sync.Mutex

func lockEmbeddedSubtitle(key string) *sync.Mutex {
	v, _ := embeddedSubtitleLocks.LoadOrStore(key, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func subtitleVTTCacheRoot() string {
	if d := strings.TrimSpace(os.Getenv("PLUM_SUBTITLE_CACHE_DIR")); d != "" {
		return d
	}
	return filepath.Join(os.TempDir(), "plum_subtitle_cache")
}

func embeddedSubtitleVTTCachePath(sourcePath string, streamIndex int) (string, error) {
	st, err := os.Stat(sourcePath)
	if err != nil {
		return "", err
	}
	payload := fmt.Sprintf(
		"%s\x1e%d\x1e%d\x1e%d",
		filepath.Clean(sourcePath),
		st.Size(),
		st.ModTime().UnixNano(),
		streamIndex,
	)
	sum := sha256.Sum256([]byte(payload))
	name := hex.EncodeToString(sum[:]) + ".vtt"
	return filepath.Join(subtitleVTTCacheRoot(), name), nil
}

func mediaAttachmentCacheRoot() string {
	if d := strings.TrimSpace(os.Getenv("PLUM_ATTACHMENT_CACHE_DIR")); d != "" {
		return d
	}
	return filepath.Join(os.TempDir(), "plum_attachment_cache")
}

func mediaAttachmentCachePath(sourcePath string, attachment MediaAttachment) (string, error) {
	st, err := os.Stat(sourcePath)
	if err != nil {
		return "", err
	}
	payload := fmt.Sprintf(
		"%s\x1e%d\x1e%d\x1e%d\x1e%s",
		filepath.Clean(sourcePath),
		st.Size(),
		st.ModTime().UnixNano(),
		attachment.StreamIndex,
		attachment.FileName,
	)
	sum := sha256.Sum256([]byte(payload))
	ext := strings.ToLower(filepath.Ext(attachment.FileName))
	name := hex.EncodeToString(sum[:]) + ext
	return filepath.Join(mediaAttachmentCacheRoot(), name), nil
}

func tryServeMediaAttachmentFromCache(w http.ResponseWriter, r *http.Request, cachePath string, mimeType string) bool {
	if cachePath == "" {
		return false
	}
	fi, err := os.Stat(cachePath)
	if err != nil || fi.Size() == 0 {
		return false
	}
	contentType := strings.TrimSpace(mimeType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	http.ServeFile(w, r, cachePath)
	return true
}

func extractMediaAttachmentToCache(ctx context.Context, sourcePath string, attachment MediaAttachment, cachePath string) error {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return err
	}
	partialPath := cachePath + ".partial"
	_ = os.Remove(partialPath)
	ffmpegArgs := []string{"-hide_banner", "-nostats", "-loglevel", "warning"}
	ffmpegArgs = append(ffmpegArgs, ffopts.InputProbeEmbeddedExtract...)
	ffmpegArgs = append(ffmpegArgs,
		fmt.Sprintf("-dump_attachment:%d", attachment.StreamIndex),
		partialPath,
		"-i", sourcePath,
		"-t", "0",
		"-f", "null",
		"null",
	)
	cmd := exec.CommandContext(ctx, "ffmpeg", ffmpegArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(partialPath)
		msg := strings.TrimSpace(stderr.String())
		msg = trimFFmpegStderrProgress(msg)
		if msg == "" {
			msg = err.Error()
		}
		if len(msg) > 512 {
			msg = msg[len(msg)-512:]
		}
		slog.Warn("ffmpeg media attachment extract", "stderr_tail", msg)
		return fmt.Errorf("ffmpeg error: %s", msg)
	}
	if fi, err := os.Stat(partialPath); err != nil || fi.Size() == 0 {
		_ = os.Remove(partialPath)
		if err != nil {
			return err
		}
		return fmt.Errorf("ffmpeg produced empty attachment stream %d", attachment.StreamIndex)
	}
	if err := os.Rename(partialPath, cachePath); err != nil {
		_ = os.Remove(partialPath)
		return err
	}
	return nil
}

func HandleStreamMediaAttachment(w http.ResponseWriter, r *http.Request, dbConn *sql.DB, mediaID int, streamIndex int) error {
	item, err := GetMediaByID(dbConn, mediaID)
	if err != nil {
		return err
	}
	if item == nil {
		return ErrNotFound
	}
	attachment, err := GetMediaAttachmentForMedia(dbConn, mediaID, streamIndex)
	if err != nil {
		return err
	}
	if attachment == nil {
		return ErrNotFound
	}
	sourcePath, err := ResolveMediaSourcePath(dbConn, *item)
	if err != nil {
		return err
	}

	cachePath, cacheErr := mediaAttachmentCachePath(sourcePath, *attachment)
	if cacheErr == nil && tryServeMediaAttachmentFromCache(w, r, cachePath, attachment.MimeType) {
		return nil
	}

	lockKey := cachePath
	if lockKey == "" {
		lockKey = fmt.Sprintf("attachment|%s|%d", sourcePath, streamIndex)
	}
	mu := lockEmbeddedSubtitle(lockKey)
	mu.Lock()
	defer mu.Unlock()

	if cacheErr == nil && tryServeMediaAttachmentFromCache(w, r, cachePath, attachment.MimeType) {
		return nil
	}
	if cacheErr != nil {
		return cacheErr
	}
	if err := extractMediaAttachmentToCache(r.Context(), sourcePath, *attachment, cachePath); err != nil {
		return err
	}
	if !tryServeMediaAttachmentFromCache(w, r, cachePath, attachment.MimeType) {
		return fmt.Errorf("media attachment stream %d was not cached", streamIndex)
	}
	return nil
}

func tryServeEmbeddedSubtitleFromCache(w http.ResponseWriter, r *http.Request, cachePath string) bool {
	if cachePath == "" {
		return false
	}
	fi, err := os.Stat(cachePath)
	if err != nil || fi.Size() == 0 {
		return false
	}
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	// Hash filename tracks source path + mtime + size + stream index; immutable until media changes.
	w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	http.ServeFile(w, r, cachePath)
	return true
}

// subtitleDemuxFormat picks an ffmpeg muxer for a stream copy extract from the container.
// Empty codec defaults to srt (common for Blu-ray remuxes labeled subrip).
func subtitleDemuxFormat(codec string) (format string, ok bool) {
	c := strings.ToLower(strings.TrimSpace(codec))
	switch c {
	case "", "subrip", "srt":
		return "srt", true
	case "ass", "ssa":
		return "ass", true
	case "webvtt":
		return "webvtt", true
	case "mov_text", "text", "ttml", "tx3g", "hdmv_text_subtitle":
		return "srt", true
	case "eia_608", "eia_708":
		return "srt", true
	case "sami":
		return "srt", true
	default:
		return "", false
	}
}

func demuxTempSuffix(demuxFormat string) string {
	switch demuxFormat {
	case "srt":
		return ".srt"
	case "ass":
		return ".ass"
	case "webvtt":
		return ".vtt"
	default:
		return ".sub"
	}
}

// extractEmbeddedSubtitleStreamToTemp demuxes one subtitle stream with codec copy into a small sidecar file.
// This is usually much faster than decoding the entire MKV timeline straight to WebVTT in one ffmpeg process.
func extractEmbeddedSubtitleStreamToTemp(ctx context.Context, sourcePath string, streamIndex int, demuxFormat string) (tmpPath string, cleanup func(), err error) {
	suffix := demuxTempSuffix(demuxFormat)
	f, err := os.CreateTemp("", "plum-embsub-*"+suffix)
	if err != nil {
		return "", nil, err
	}
	tmpPath = f.Name()
	_ = f.Close()
	cleanup = func() { _ = os.Remove(tmpPath) }

	args := []string{
		"-hide_banner", "-nostats", "-loglevel", "error",
	}
	args = append(args, ffopts.InputProbeSubtitleDemux...)
	args = append(args,
		"-i", sourcePath,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-c", "copy",
		"-f", demuxFormat,
		"-y",
		tmpPath,
	)
	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("ffmpeg demux: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	info, statErr := os.Stat(tmpPath)
	if statErr != nil || info.Size() == 0 {
		cleanup()
		return "", nil, fmt.Errorf("ffmpeg demux produced empty output")
	}
	return tmpPath, cleanup, nil
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

// trimFFmpegStderrProgress drops encoding progress lines ffmpeg writes to stderr so error
// summaries are not dominated by the last 512 bytes of "size=... time=... speed=..." spam.
// flushOnWrite calls http.Flusher after each successful Write so chunked subtitle responses
// reach the browser (and dev proxies) promptly; otherwise the first bytes can sit in buffers
// until ffmpeg exits or the buffer fills.
type flushOnWrite struct {
	http.ResponseWriter
	flush http.Flusher
}

func (f *flushOnWrite) Write(p []byte) (int, error) {
	n, err := f.ResponseWriter.Write(p)
	if n > 0 && f.flush != nil {
		f.flush.Flush()
	}
	return n, err
}

func responseWriterForFFmpegStdout(w http.ResponseWriter) io.Writer {
	if fl, ok := w.(http.Flusher); ok {
		return &flushOnWrite{ResponseWriter: w, flush: fl}
	}
	return w
}

// flushTeeWriter writes to the HTTP response and a side file (subtitle disk cache), flushing the
// response after each chunk so clients do not sit on “loading” until the file is complete.
type flushTeeWriter struct {
	w http.ResponseWriter
	t io.Writer
	f http.Flusher
}

func (x *flushTeeWriter) Write(p []byte) (int, error) {
	n, err := x.w.Write(p)
	if n <= 0 {
		return n, err
	}
	if _, terr := x.t.Write(p[:n]); terr != nil && err == nil {
		err = terr
	}
	if x.f != nil {
		x.f.Flush()
	}
	return n, err
}

func responseWriterForFFmpegStdoutAndTee(w http.ResponseWriter, tee io.Writer) io.Writer {
	if tee == nil {
		return responseWriterForFFmpegStdout(w)
	}
	if fl, ok := w.(http.Flusher); ok {
		return &flushTeeWriter{w: w, t: tee, f: fl}
	}
	return io.MultiWriter(w, tee)
}

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
	return streamFFmpegWebVTTWithOptionalTee(w, r, nil, args...)
}

func streamFFmpegWebVTTWithOptionalTee(w http.ResponseWriter, r *http.Request, tee io.Writer, args ...string) error {
	w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
	// Response is generated on the fly (conversion or first-fill of disk cache); avoid storing partial streams.
	w.Header().Set("Cache-Control", "no-store")
	// -nostats keeps stderr usable on failure (we were truncating to a tail that was often progress only).
	ffmpegArgs := []string{"-hide_banner", "-nostats", "-loglevel", "warning"}
	ffmpegArgs = append(ffmpegArgs, args...)
	cmd := exec.CommandContext(r.Context(), "ffmpeg", ffmpegArgs...)
	cmd.Stdout = responseWriterForFFmpegStdoutAndTee(w, tee)
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
		slog.Warn("ffmpeg subtitle stream", "stderr_tail", msg)
		return fmt.Errorf("ffmpeg error: %s", msg)
	}
	return nil
}
