package transcoder

import (
	"strings"
	"testing"

	"plum/internal/db"
)

func TestInjectHlsSubtitleRenditions_NoStreamInf(t *testing.T) {
	body := "#EXTM3U\n#EXTINF:2,\nseg.ts\n"
	got := InjectHlsSubtitleRenditions(body, []HlsWebSubtitle{
		{PlaylistFile: "plum_subs_emb_2.m3u8", VTTPath: "/api/media/1/subtitles/embedded/2", DisplayName: "English", Language: "en"},
	})
	if got != body {
		t.Fatalf("expected unchanged media playlist, got %q", got)
	}
}

func TestInjectHlsSubtitleRenditions_PrependsMediaAndAddsGroup(t *testing.T) {
	body := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-STREAM-INF:BANDWIDTH=1000000,RESOLUTION=1280x720,CODECS="avc1.4d401f,mp4a.40.2"
variant_0/index.m3u8
`
	tracks := []HlsWebSubtitle{
		{PlaylistFile: "plum_subs_emb_3.m3u8", VTTPath: "/api/media/9/subtitles/embedded/3", DisplayName: `Eng "test"`, Language: "en"},
	}
	got := InjectHlsSubtitleRenditions(body, tracks)
	if !strings.Contains(got, `TYPE=SUBTITLES`) {
		t.Fatalf("missing subtitle media: %q", got)
	}
	if !strings.Contains(got, `SUBTITLES="subs"`) {
		t.Fatalf("missing SUBTITLES on stream inf: %q", got)
	}
	if !strings.Contains(got, `plum_subs_emb_3.m3u8`) {
		t.Fatalf("missing subtitle playlist uri: %q", got)
	}
	if strings.Count(got, `GROUP-ID="subs"`) < 1 {
		t.Fatalf("expected subs group: %q", got)
	}
}

func TestInjectHlsSubtitleRenditions_Idempotent(t *testing.T) {
	body := `#EXTM3U
#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID="subs",NAME="x",URI="y.m3u8"
#EXT-X-STREAM-INF:BANDWIDTH=1,SUBTITLES="subs"
v/index.m3u8
`
	tracks := []HlsWebSubtitle{{PlaylistFile: "plum_subs_emb_1.m3u8", VTTPath: "/x", DisplayName: "A", Language: "en"}}
	got := InjectHlsSubtitleRenditions(body, tracks)
	if got != body {
		t.Fatalf("expected no double inject, got %q", got)
	}
}

func TestCollectHlsWebSubtitles_FiltersUnsupportedEmbedded(t *testing.T) {
	falseVal := false
	media := db.MediaItem{
		ID: 1,
		EmbeddedSubtitles: []db.EmbeddedSubtitle{
			{StreamIndex: 1, Language: "en", Codec: "subrip"},
			{StreamIndex: 2, Language: "ja", Codec: "hdmv_pgs_subtitle", Supported: &falseVal},
			{StreamIndex: 3, Language: "de", Codec: "hdmv_pgs_subtitle"},
		},
		Subtitles: []db.Subtitle{{ID: 10, Language: "fr", Format: "srt"}},
	}
	got := CollectHlsWebSubtitles(media)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2: %#v", len(got), got)
	}
}

func TestParseVirtualSubtitlePlaylistName(t *testing.T) {
	k, id, ok := ParseVirtualSubtitlePlaylistName("plum_subs_emb_12.m3u8")
	if !ok || k != "emb" || id != 12 {
		t.Fatalf("emb: ok=%v k=%q id=%d", ok, k, id)
	}
	k, id, ok = ParseVirtualSubtitlePlaylistName("plum_subs_ext_99.m3u8")
	if !ok || k != "ext" || id != 99 {
		t.Fatalf("ext: ok=%v k=%q id=%d", ok, k, id)
	}
	_, _, ok = ParseVirtualSubtitlePlaylistName("index.m3u8")
	if ok {
		t.Fatal("expected false for index.m3u8")
	}
}
