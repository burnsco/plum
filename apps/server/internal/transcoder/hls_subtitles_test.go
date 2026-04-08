package transcoder

import (
	"encoding/json"
	"strings"
	"testing"

	"plum/internal/db"
)

func TestInjectHlsSubtitleRenditions_NoStreamInf(t *testing.T) {
	body := "#EXTM3U\n#EXTINF:2,\nseg.ts\n"
	got := InjectHlsSubtitleRenditions(body, []HlsWebSubtitle{
		{LogicalID: "emb:2", PlaylistFile: "plum_subs_656d623a32.m3u8", VTTPath: "/api/media/1/subtitles/embedded/2", DisplayName: "English", Language: "en"},
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
		{LogicalID: "emb:3", PlaylistFile: "plum_subs_656d623a33.m3u8", VTTPath: "/api/media/9/subtitles/embedded/3", DisplayName: `Eng "test"`, Language: "en"},
	}
	got := InjectHlsSubtitleRenditions(body, tracks)
	if !strings.Contains(got, `TYPE=SUBTITLES`) {
		t.Fatalf("missing subtitle media: %q", got)
	}
	if !strings.Contains(got, `SUBTITLES="subs"`) {
		t.Fatalf("missing SUBTITLES on stream inf: %q", got)
	}
	if !strings.Contains(got, `plum_subs_656d623a33.m3u8`) {
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
	tracks := []HlsWebSubtitle{{LogicalID: "emb:1", PlaylistFile: "plum_subs_656d623a31.m3u8", VTTPath: "/x", DisplayName: "A", Language: "en"}}
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
	if got[0].LogicalID != "ext:10" {
		t.Fatalf("sidecar logical id = %q", got[0].LogicalID)
	}
	if got[1].LogicalID != "emb:1" {
		t.Fatalf("embedded logical id = %q", got[1].LogicalID)
	}
}

func TestSidecarSubtitleMarshalJSON_EmitsLogicalID(t *testing.T) {
	body, err := json.Marshal(db.Subtitle{
		ID:       101,
		Language: "en",
		Format:   "srt",
		Title:    "English",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(body), `"logicalId":"ext:101"`) {
		t.Fatalf("expected logicalId in payload, got %s", body)
	}
}

func TestParseVirtualSubtitlePlaylistName(t *testing.T) {
	logicalID, ok := ParseVirtualSubtitlePlaylistName("plum_subs_656d623a3132.m3u8")
	if !ok || logicalID != "emb:12" {
		t.Fatalf("emb: ok=%v logicalID=%q", ok, logicalID)
	}
	logicalID, ok = ParseVirtualSubtitlePlaylistName("plum_subs_6578743a3939.m3u8")
	if !ok || logicalID != "ext:99" {
		t.Fatalf("ext: ok=%v logicalID=%q", ok, logicalID)
	}
	_, ok = ParseVirtualSubtitlePlaylistName("index.m3u8")
	if ok {
		t.Fatal("expected false for index.m3u8")
	}
}
