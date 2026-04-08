package db

import "testing"

func TestParseFontAttachment(t *testing.T) {
	attachment, ok := parseFontAttachment(2, 9, "ttf", "MyFont.ttf", "application/x-truetype-font")
	if !ok {
		t.Fatal("expected font attachment to be recognized")
	}
	if attachment.Index != 2 {
		t.Fatalf("index = %d", attachment.Index)
	}
	if attachment.StreamIndex != 9 {
		t.Fatalf("streamIndex = %d", attachment.StreamIndex)
	}
	if attachment.Filename != "MyFont.ttf" {
		t.Fatalf("filename = %q", attachment.Filename)
	}
}

func TestFindEmbeddedFontAttachment(t *testing.T) {
	attachments := []EmbeddedFontAttachment{
		{Index: 0, StreamIndex: 9, Filename: "A.ttf"},
		{Index: 1, StreamIndex: 10, Filename: "B.ttf"},
	}
	got := findEmbeddedFontAttachment(attachments, 1)
	if got == nil {
		t.Fatal("expected attachment")
	}
	if got.Filename != "B.ttf" {
		t.Fatalf("filename = %q", got.Filename)
	}
}

func TestPlaybackTrackMetadataNeedsEmbeddedFonts(t *testing.T) {
	t.Run("sidecar ass subtitle", func(t *testing.T) {
		if !playbackTrackMetadataNeedsEmbeddedFonts(
			[]Subtitle{{Format: "ass"}},
			nil,
		) {
			t.Fatal("expected ASS sidecar subtitles to request embedded fonts")
		}
	})

	t.Run("embedded ass subtitle", func(t *testing.T) {
		supported := true
		if !playbackTrackMetadataNeedsEmbeddedFonts(
			nil,
			[]EmbeddedSubtitle{{Codec: "ass", Supported: &supported}},
		) {
			t.Fatal("expected ASS-eligible embedded subtitles to request embedded fonts")
		}
	})

	t.Run("non ass subtitles", func(t *testing.T) {
		supported := true
		if playbackTrackMetadataNeedsEmbeddedFonts(
			[]Subtitle{{Format: "srt"}},
			[]EmbeddedSubtitle{{Codec: "subrip", Supported: &supported}},
		) {
			t.Fatal("did not expect non-ASS subtitles to request embedded fonts")
		}
	})
}
