package transcoder

import (
	"errors"
	"testing"

	"plum/internal/db"
)

func boolPtr(v bool) *bool { return &v }

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

func TestEmbeddedSubtitlesForPlaybackJSON_ClassifiesSubripTranscodePrefersAss(t *testing.T) {
	subs := embeddedSubtitlesForPlaybackJSON(
		db.MediaItem{
			EmbeddedSubtitles: []db.EmbeddedSubtitle{
				{
					StreamIndex: 2,
					Language:    "en",
					Title:       "English",
					Codec:       "subrip",
					Supported:   boolPtr(true),
				},
			},
		},
		"transcode",
	)
	if len(subs) != 1 {
		t.Fatalf("len(subs) = %d", len(subs))
	}
	got := subs[0]
	if !got.VttEligible || !got.AssEligible {
		t.Fatalf("expected vtt and ass eligibility for subrip, got %#v", got)
	}
	if got.PreferredWebDeliveryMode == nil || *got.PreferredWebDeliveryMode != PlaybackEmbeddedSubtitleDeliveryModeASS {
		t.Fatalf("preferredWebDeliveryMode = %#v", got.PreferredWebDeliveryMode)
	}
	if len(got.DeliveryModes) != 2 {
		t.Fatalf("deliveryModes = %#v", got.DeliveryModes)
	}
}

func TestEmbeddedSubtitlesForPlaybackJSON_ClassifiesDirectASS(t *testing.T) {
	subs := embeddedSubtitlesForPlaybackJSON(
		db.MediaItem{
			EmbeddedSubtitles: []db.EmbeddedSubtitle{
				{
					StreamIndex: 5,
					Language:    "en",
					Title:       "Styled English",
					Codec:       "ass",
					Supported:   boolPtr(true),
				},
			},
		},
		"direct",
	)
	if len(subs) != 1 {
		t.Fatalf("len(subs) = %d", len(subs))
	}
	got := subs[0]
	if got.LogicalID != "emb:5" {
		t.Fatalf("logicalId = %q", got.LogicalID)
	}
	if !got.VttEligible || !got.AssEligible {
		t.Fatalf("expected vttEligible and assEligible, got %#v", got)
	}
	if got.PreferredWebDeliveryMode == nil || *got.PreferredWebDeliveryMode != PlaybackEmbeddedSubtitleDeliveryModeASS {
		t.Fatalf("preferredWebDeliveryMode = %#v", got.PreferredWebDeliveryMode)
	}
	if got.PreferredAndroidDeliveryMode == nil || *got.PreferredAndroidDeliveryMode != PlaybackEmbeddedSubtitleDeliveryModeDirectVTT {
		t.Fatalf("preferredAndroidDeliveryMode = %#v", got.PreferredAndroidDeliveryMode)
	}
	if len(got.DeliveryModes) != 2 {
		t.Fatalf("deliveryModes = %#v", got.DeliveryModes)
	}
	if got.DeliveryModes[0].Mode != PlaybackEmbeddedSubtitleDeliveryModeDirectVTT {
		t.Fatalf("first delivery mode = %#v", got.DeliveryModes[0])
	}
	if got.DeliveryModes[1].Mode != PlaybackEmbeddedSubtitleDeliveryModeASS {
		t.Fatalf("second delivery mode = %#v", got.DeliveryModes[1])
	}
}

func TestEmbeddedSubtitlesForPlaybackJSON_ClassifiesHlsPGS(t *testing.T) {
	subs := embeddedSubtitlesForPlaybackJSON(
		db.MediaItem{
			EmbeddedSubtitles: []db.EmbeddedSubtitle{
				{
					StreamIndex: 8,
					Language:    "ja",
					Title:       "Japanese",
					Codec:       "hdmv_pgs_subtitle",
					Supported:   boolPtr(true),
				},
			},
		},
		"transcode",
	)
	if len(subs) != 1 {
		t.Fatalf("len(subs) = %d", len(subs))
	}
	got := subs[0]
	if got.VttEligible || got.AssEligible {
		t.Fatalf("expected no vtt/ass eligibility, got %#v", got)
	}
	if !got.PgsBinaryEligible {
		t.Fatalf("expected pgsBinaryEligible, got %#v", got)
	}
	if got.PreferredWebDeliveryMode == nil || *got.PreferredWebDeliveryMode != PlaybackEmbeddedSubtitleDeliveryModeBurnIn {
		t.Fatalf("preferredWebDeliveryMode = %#v", got.PreferredWebDeliveryMode)
	}
	if got.PreferredAndroidDeliveryMode == nil || *got.PreferredAndroidDeliveryMode != PlaybackEmbeddedSubtitleDeliveryModePgsBinary {
		t.Fatalf("preferredAndroidDeliveryMode = %#v", got.PreferredAndroidDeliveryMode)
	}
	if len(got.DeliveryModes) != 2 {
		t.Fatalf("deliveryModes = %#v", got.DeliveryModes)
	}
	if got.DeliveryModes[0].Mode != PlaybackEmbeddedSubtitleDeliveryModePgsBinary || got.DeliveryModes[0].RequiresReload {
		t.Fatalf("first delivery mode = %#v", got.DeliveryModes[0])
	}
	if got.DeliveryModes[1].Mode != PlaybackEmbeddedSubtitleDeliveryModeBurnIn || !got.DeliveryModes[1].RequiresReload {
		t.Fatalf("second delivery mode = %#v", got.DeliveryModes[1])
	}
}

func TestEmbeddedSubtitlesForPlaybackJSON_ClassifiesBurnOnlyBitmap(t *testing.T) {
	subs := embeddedSubtitlesForPlaybackJSON(
		db.MediaItem{
			EmbeddedSubtitles: []db.EmbeddedSubtitle{
				{
					StreamIndex: 4,
					Language:    "fr",
					Title:       "French",
					Codec:       "dvd_subtitle",
					Supported:   boolPtr(true),
				},
			},
		},
		"remux",
	)
	if len(subs) != 1 {
		t.Fatalf("len(subs) = %d", len(subs))
	}
	got := subs[0]
	if len(got.DeliveryModes) != 1 {
		t.Fatalf("deliveryModes = %#v", got.DeliveryModes)
	}
	if got.DeliveryModes[0].Mode != PlaybackEmbeddedSubtitleDeliveryModeBurnIn || !got.DeliveryModes[0].RequiresReload {
		t.Fatalf("deliveryModes[0] = %#v", got.DeliveryModes[0])
	}
	if got.PreferredWebDeliveryMode == nil || *got.PreferredWebDeliveryMode != PlaybackEmbeddedSubtitleDeliveryModeBurnIn {
		t.Fatalf("preferredWebDeliveryMode = %#v", got.PreferredWebDeliveryMode)
	}
	if got.PreferredAndroidDeliveryMode == nil || *got.PreferredAndroidDeliveryMode != PlaybackEmbeddedSubtitleDeliveryModeBurnIn {
		t.Fatalf("preferredAndroidDeliveryMode = %#v", got.PreferredAndroidDeliveryMode)
	}
}

func TestEmbeddedSubtitlesForPlaybackJSON_UnsupportedTrackHasNoModes(t *testing.T) {
	subs := embeddedSubtitlesForPlaybackJSON(
		db.MediaItem{
			EmbeddedSubtitles: []db.EmbeddedSubtitle{
				{
					StreamIndex: 9,
					Language:    "en",
					Title:       "Broken",
					Codec:       "subrip",
					Supported:   boolPtr(false),
				},
			},
		},
		"direct",
	)
	if len(subs) != 1 {
		t.Fatalf("len(subs) = %d", len(subs))
	}
	got := subs[0]
	if got.VttEligible || got.PgsBinaryEligible || got.AssEligible {
		t.Fatalf("unexpected eligibility on unsupported track: %#v", got)
	}
	if got.DeliveryModes != nil {
		t.Fatalf("deliveryModes = %#v", got.DeliveryModes)
	}
	if got.PreferredWebDeliveryMode != nil || got.PreferredAndroidDeliveryMode != nil {
		t.Fatalf("unexpected preferred modes: web=%#v android=%#v", got.PreferredWebDeliveryMode, got.PreferredAndroidDeliveryMode)
	}
}
