package transcoder

import (
	"errors"
	"testing"

	"plum/internal/db"
)

func TestValidateBurnEmbeddedSubtitle_AcceptsPGS(t *testing.T) {
	probe := playbackSourceProbe{
		Streams: []playbackStreamProbe{
			{Index: 0, CodecType: "video", CodecName: "h264"},
			{Index: 1, CodecType: "audio", CodecName: "aac"},
			{Index: 3, CodecType: "subtitle", CodecName: "hdmv_pgs_subtitle"},
		},
	}
	media := db.MediaItem{
		EmbeddedSubtitles: []db.EmbeddedSubtitle{
			{StreamIndex: 3, Codec: "hdmv_pgs_subtitle", Language: "en", Title: "English"},
		},
	}
	if err := ValidateBurnEmbeddedSubtitle(probe, media, 3); err != nil {
		t.Fatal(err)
	}
}

func TestValidateBurnEmbeddedSubtitle_RejectsTextCodec(t *testing.T) {
	probe := playbackSourceProbe{
		Streams: []playbackStreamProbe{
			{Index: 0, CodecType: "video", CodecName: "h264"},
			{Index: 2, CodecType: "subtitle", CodecName: "subrip"},
		},
	}
	media := db.MediaItem{
		EmbeddedSubtitles: []db.EmbeddedSubtitle{
			{StreamIndex: 2, Codec: "subrip"},
		},
	}
	err := ValidateBurnEmbeddedSubtitle(probe, media, 2)
	if err == nil {
		t.Fatal("expected error")
	}
	var be BurnSubtitleError
	if !errors.As(err, &be) {
		t.Fatalf("want BurnSubtitleError, got %T %v", err, err)
	}
}
