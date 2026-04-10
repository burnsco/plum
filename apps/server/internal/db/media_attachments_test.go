package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFFprobeJSONShim(t *testing.T, payload string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ffprobe")
	script := "#!/bin/sh\ncat <<'EOF'\n" + payload + "\nEOF\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write ffprobe shim: %v", err)
	}
	originalPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+originalPath); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
	})
}

func createMovieMediaForAttachmentTest(t *testing.T, dbConn *sql.DB, sourcePath string) (int, int) {
	t.Helper()
	libraryID := createLibraryForTest(t, dbConn, LibraryTypeMovie, filepath.Dir(sourcePath))
	var refID int
	if err := dbConn.QueryRow(
		`INSERT INTO movies (library_id, title, path, duration, match_status) VALUES (?, ?, ?, ?, ?) RETURNING id`,
		libraryID,
		"Attachment Movie",
		sourcePath,
		120,
		MatchStatusLocal,
	).Scan(&refID); err != nil {
		t.Fatalf("insert movie: %v", err)
	}
	var mediaID int
	if err := dbConn.QueryRow(`INSERT INTO media_global (kind, ref_id) VALUES (?, ?) RETURNING id`, LibraryTypeMovie, refID).Scan(&mediaID); err != nil {
		t.Fatalf("insert media_global: %v", err)
	}
	return mediaID, libraryID
}

func TestProbeVideoMetadataCapturesAttachmentStreams(t *testing.T) {
	writeFFprobeJSONShim(t, `{"format":{"duration":"120.5"},"streams":[{"index":0,"codec_type":"video","codec_name":"h264"},{"index":4,"codec_type":"attachment","codec_name":"ttf","tags":{"filename":"Fancy Font.ttf","mimetype":"application/x-truetype-font","comment":"Used by ASS subtitles"}}]}`)

	probed, err := probeVideoMetadata(context.Background(), filepath.Join(t.TempDir(), "episode.mkv"))
	if err != nil {
		t.Fatalf("probe video metadata: %v", err)
	}
	if len(probed.MediaAttachments) != 1 {
		t.Fatalf("media attachments = %#v", probed.MediaAttachments)
	}
	attachment := probed.MediaAttachments[0]
	if attachment.StreamIndex != 4 || attachment.FileName != "Fancy Font.ttf" || attachment.MimeType != "application/x-truetype-font" || attachment.Codec != "ttf" || attachment.Comment != "Used by ASS subtitles" {
		t.Fatalf("unexpected attachment metadata: %#v", attachment)
	}
}

func TestRefreshPlaybackTrackMetadataPersistsMediaAttachments(t *testing.T) {
	dbConn := newTestDB(t)
	sourcePath := filepath.Join(t.TempDir(), "movie.mkv")
	if err := os.WriteFile(sourcePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	mediaID, libraryID := createMovieMediaForAttachmentTest(t, dbConn, sourcePath)

	prevVideoProbe := readVideoMetadata
	readVideoMetadata = func(_ context.Context, path string) (VideoProbeResult, error) {
		if path != sourcePath {
			t.Fatalf("probe path = %q, want %q", path, sourcePath)
		}
		return VideoProbeResult{
			EmbeddedSubtitles: []EmbeddedSubtitle{{StreamIndex: 2, Language: "en", Title: "English", Codec: "ass"}},
			EmbeddedAudioTracks: []EmbeddedAudioTrack{{
				StreamIndex: 1,
				Language:    "ja",
				Title:       "Japanese",
			}},
			MediaAttachments: []MediaAttachment{{
				StreamIndex: 7,
				FileName:    "Fancy.otf",
				MimeType:    "font/otf",
				Codec:       "otf",
			}},
		}, nil
	}
	t.Cleanup(func() {
		readVideoMetadata = prevVideoProbe
	})

	item := &MediaItem{ID: mediaID, LibraryID: libraryID, Type: LibraryTypeMovie, Path: sourcePath}
	metadata, err := RefreshPlaybackTrackMetadata(context.Background(), dbConn, item)
	if err != nil {
		t.Fatalf("refresh playback metadata: %v", err)
	}
	if len(metadata.MediaAttachments) != 1 || metadata.MediaAttachments[0].MediaID != mediaID {
		t.Fatalf("metadata attachments = %#v", metadata.MediaAttachments)
	}
	if metadata.MediaAttachments[0].StreamIndex != 7 || metadata.MediaAttachments[0].FileName != "Fancy.otf" {
		t.Fatalf("metadata attachments = %#v", metadata.MediaAttachments)
	}

	persisted, err := getMediaAttachmentsForMedia(dbConn, mediaID)
	if err != nil {
		t.Fatalf("load persisted attachments: %v", err)
	}
	if len(persisted) != 1 || persisted[0].StreamIndex != 7 || persisted[0].MimeType != "font/otf" {
		t.Fatalf("persisted attachments = %#v", persisted)
	}
}

func writeFFmpegAttachmentShim(t *testing.T, fontBody string) string {
	t.Helper()
	dir := t.TempDir()
	countPath := filepath.Join(dir, "count")
	path := filepath.Join(dir, "ffmpeg")
	script := fmt.Sprintf(`#!/bin/sh
count_file='%s'
out=''
take_next=0
for arg in "$@"; do
  if [ "$take_next" = "1" ]; then
    out="$arg"
    take_next=0
  fi
  case "$arg" in
    -dump_attachment:*) take_next=1 ;;
  esac
done
if [ -z "$out" ]; then
  echo "missing dump_attachment output" >&2
  exit 2
fi
count=0
if [ -f "$count_file" ]; then
  count=$(cat "$count_file")
fi
count=$((count + 1))
printf '%%s' "$count" > "$count_file"
printf '%%s' '%s' > "$out"
`, countPath, fontBody)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("write ffmpeg shim: %v", err)
	}
	originalPath := os.Getenv("PATH")
	if err := os.Setenv("PATH", dir+string(os.PathListSeparator)+originalPath); err != nil {
		t.Fatalf("set PATH: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PATH", originalPath)
	})
	return countPath
}

func TestHandleStreamMediaAttachmentServesAndReusesCachedExtraction(t *testing.T) {
	dbConn := newTestDB(t)
	sourcePath := filepath.Join(t.TempDir(), "movie.mkv")
	if err := os.WriteFile(sourcePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	mediaID, _ := createMovieMediaForAttachmentTest(t, dbConn, sourcePath)
	if _, err := dbConn.Exec(
		`INSERT INTO media_attachments (media_id, stream_index, file_name, mime_type, codec) VALUES (?, ?, ?, ?, ?)`,
		mediaID,
		8,
		"CoolFont.ttf",
		"font/ttf",
		"ttf",
	); err != nil {
		t.Fatalf("insert media attachment: %v", err)
	}

	cacheDir := t.TempDir()
	previousCacheDir := os.Getenv("PLUM_ATTACHMENT_CACHE_DIR")
	if err := os.Setenv("PLUM_ATTACHMENT_CACHE_DIR", cacheDir); err != nil {
		t.Fatalf("set attachment cache dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Setenv("PLUM_ATTACHMENT_CACHE_DIR", previousCacheDir)
	})
	countPath := writeFFmpegAttachmentShim(t, "FAKEFONT")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/media/%d/attachments/%d", mediaID, 8), nil)
	rec := httptest.NewRecorder()
	if err := HandleStreamMediaAttachment(rec, req, dbConn, mediaID, 8); err != nil {
		t.Fatalf("stream media attachment: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", rec.Code, rec.Body.String())
	}
	if got := strings.TrimSpace(rec.Header().Get("Content-Type")); got != "font/ttf" {
		t.Fatalf("content-type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); !strings.Contains(got, "private") || !strings.Contains(got, "immutable") {
		t.Fatalf("cache-control = %q", got)
	}
	if rec.Body.String() != "FAKEFONT" {
		t.Fatalf("body = %q", rec.Body.String())
	}

	secondRec := httptest.NewRecorder()
	if err := HandleStreamMediaAttachment(secondRec, req, dbConn, mediaID, 8); err != nil {
		t.Fatalf("stream cached media attachment: %v", err)
	}
	if secondRec.Body.String() != "FAKEFONT" {
		t.Fatalf("cached body = %q", secondRec.Body.String())
	}
	countBytes, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatalf("read ffmpeg count: %v", err)
	}
	if strings.TrimSpace(string(countBytes)) != "1" {
		t.Fatalf("ffmpeg extract count = %q", string(countBytes))
	}

	missingRec := httptest.NewRecorder()
	if err := HandleStreamMediaAttachment(missingRec, req, dbConn, mediaID, 9); err != ErrNotFound {
		t.Fatalf("missing attachment error = %v", err)
	}
}
