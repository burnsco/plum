package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseSidecarSubtitleMetadata_StrictSiblingMatching(t *testing.T) {
	videoPath := filepath.Join(t.TempDir(), "Movie.mkv")
	cases := []struct {
		name     string
		filename string
		wantOK   bool
		wantLang string
		wantText string
		forced   bool
		hi       bool
		def      bool
	}{
		{name: "plain", filename: "Movie.srt", wantOK: true, wantLang: "und", wantText: ""},
		{name: "language", filename: "Movie.en.srt", wantOK: true, wantLang: "en", wantText: "English"},
		{name: "hyphenated language", filename: "Movie.en-US.srt", wantOK: true, wantLang: "en", wantText: "English"},
		{name: "forced", filename: "Movie.en.forced.srt", wantOK: true, wantLang: "en", wantText: "English • Forced", forced: true},
		{name: "hyphenated locale with qualifier", filename: "Movie.pt-BR.forced.srt", wantOK: true, wantLang: "pt", wantText: "Portuguese • Forced", forced: true},
		{name: "sdh", filename: "Movie.eng.sdh.srt", wantOK: true, wantLang: "en", wantText: "English • SDH", hi: true},
		{name: "default marker after language", filename: "Movie.en.default.srt", wantOK: true, wantLang: "en", wantText: "English", def: true},
		{name: "default marker only", filename: "Movie.default.srt", wantOK: true, wantLang: "und", wantText: "", def: true},
		{name: "neighbor title excluded", filename: "Movie 2.en.srt", wantOK: false},
		{name: "unrecognized suffix excluded", filename: "Movie.directors-cut.en.srt", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseSidecarSubtitleMetadata(videoPath, tc.filename)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want %v (%+v)", ok, tc.wantOK, got)
			}
			if !tc.wantOK {
				return
			}
			if got.Language != tc.wantLang || got.Title != tc.wantText || got.Forced != tc.forced || got.HI != tc.hi || got.Default != tc.def {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestScanForSubtitles_ReplacesStaleRowsAndOrdersDeterministically(t *testing.T) {
	dbConn, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	root := t.TempDir()
	videoPath := filepath.Join(root, "Movie.mkv")
	if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write video: %v", err)
	}
	files := map[string]string{
		"Movie.en.srt":        "one",
		"Movie.en.forced.srt": "two",
		"Movie 2.en.srt":      "bad",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write subtitle %s: %v", name, err)
		}
	}

	if _, err := dbConn.Exec(`INSERT INTO subtitles (media_id, title, language, format, path) VALUES (?, ?, ?, ?, ?)`, 99, "stale", "en", "srt", filepath.Join(root, "stale.srt")); err != nil {
		t.Fatalf("seed stale subtitle: %v", err)
	}

	if err := scanForSubtitles(context.Background(), dbConn, 99, videoPath); err != nil {
		t.Fatalf("scan subtitles: %v", err)
	}
	subs, err := getSubtitlesForMedia(dbConn, 99)
	if err != nil {
		t.Fatalf("get subtitles: %v", err)
	}
	if len(subs) != 2 {
		t.Fatalf("len=%d subs=%#v", len(subs), subs)
	}
	if subs[0].Title != "English • Forced" || !subs[0].Forced {
		t.Fatalf("first subtitle = %#v", subs[0])
	}
	if subs[1].Title != "English" || subs[1].Forced {
		t.Fatalf("second subtitle = %#v", subs[1])
	}

	if err := os.Remove(filepath.Join(root, "Movie.en.forced.srt")); err != nil {
		t.Fatalf("remove forced subtitle: %v", err)
	}
	if err := scanForSubtitles(context.Background(), dbConn, 99, videoPath); err != nil {
		t.Fatalf("rescan subtitles: %v", err)
	}
	subs, err = getSubtitlesForMedia(dbConn, 99)
	if err != nil {
		t.Fatalf("get subtitles after rescan: %v", err)
	}
	if len(subs) != 1 || subs[0].Title != "English" {
		t.Fatalf("after rescan subs=%#v", subs)
	}
}

func TestScanForSubtitles_PreservesIDsWhenPathsUnchanged(t *testing.T) {
	dbConn, err := InitDB(":memory:")
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { _ = dbConn.Close() })

	root := t.TempDir()
	videoPath := filepath.Join(root, "Movie.mkv")
	if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
		t.Fatalf("write video: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "Movie.en.srt"), []byte("one"), 0o644); err != nil {
		t.Fatalf("write subtitle: %v", err)
	}

	if err := scanForSubtitles(context.Background(), dbConn, 42, videoPath); err != nil {
		t.Fatalf("first scan: %v", err)
	}
	first, err := getSubtitlesForMedia(dbConn, 42)
	if err != nil {
		t.Fatalf("get subtitles: %v", err)
	}
	if len(first) != 1 || first[0].ID == 0 {
		t.Fatalf("first subs=%#v", first)
	}
	firstID := first[0].ID

	if err := scanForSubtitles(context.Background(), dbConn, 42, videoPath); err != nil {
		t.Fatalf("second scan: %v", err)
	}
	second, err := getSubtitlesForMedia(dbConn, 42)
	if err != nil {
		t.Fatalf("get subtitles after rescan: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("len=%d subs=%#v", len(second), second)
	}
	if second[0].ID != firstID {
		t.Fatalf("subtitle id changed: first=%d second=%d", firstID, second[0].ID)
	}
}
