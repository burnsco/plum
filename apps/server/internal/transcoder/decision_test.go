package transcoder

import "testing"

func TestDecidePlaybackKeepsDefaultAudioDirect(t *testing.T) {
	probe := playbackSourceProbe{
		Container: "mp4",
		Streams: []playbackStreamProbe{
			{Index: 0, CodecType: "video", CodecName: "h264"},
			{Index: 1, CodecType: "audio", CodecName: "aac"},
			{Index: 2, CodecType: "audio", CodecName: "aac"},
		},
	}
	capabilities := ClientPlaybackCapabilities{
		SupportsMSEHLS: true,
		Containers:     []string{"mp4"},
		VideoCodecs:    []string{"h264"},
		AudioCodecs:    []string{"aac"},
	}

	decision := decidePlayback(42, probe, capabilities, -1, nil)

	if decision.Delivery != "direct" {
		t.Fatalf("delivery = %q, want direct", decision.Delivery)
	}
}

func TestDecidePlaybackAvoidsDirectForAlternateAudioSelection(t *testing.T) {
	probe := playbackSourceProbe{
		Container: "mp4",
		Streams: []playbackStreamProbe{
			{Index: 0, CodecType: "video", CodecName: "h264"},
			{Index: 1, CodecType: "audio", CodecName: "aac"},
			{Index: 2, CodecType: "audio", CodecName: "aac"},
		},
	}
	capabilities := ClientPlaybackCapabilities{
		SupportsMSEHLS: true,
		Containers:     []string{"mp4"},
		VideoCodecs:    []string{"h264"},
		AudioCodecs:    []string{"aac"},
	}

	decision := decidePlayback(42, probe, capabilities, 2, nil)

	if decision.Delivery != "remux" {
		t.Fatalf("delivery = %q, want remux", decision.Delivery)
	}
	if !decision.VideoCopy {
		t.Fatal("expected video to stay copy-safe for remux")
	}
	if !decision.AudioCopy {
		t.Fatal("expected selected audio to stay copy-safe for remux")
	}
}

func TestDecidePlaybackBurnForcesTranscode(t *testing.T) {
	probe := playbackSourceProbe{
		Container: "mp4",
		Streams: []playbackStreamProbe{
			{Index: 0, CodecType: "video", CodecName: "h264"},
			{Index: 1, CodecType: "audio", CodecName: "aac"},
		},
	}
	capabilities := ClientPlaybackCapabilities{
		SupportsNativeHLS: true,
		Containers:        []string{"mp4"},
		VideoCodecs:       []string{"h264"},
		AudioCodecs:       []string{"aac"},
	}
	burn := 4
	decision := decidePlayback(42, probe, capabilities, -1, &burn)
	if decision.Delivery != "transcode" {
		t.Fatalf("delivery = %q, want transcode", decision.Delivery)
	}
	if decision.VideoCopy {
		t.Fatal("expected video transcode for burn-in")
	}
	if decision.BurnEmbeddedSubtitleStreamIdx != 4 {
		t.Fatalf("burn idx = %d", decision.BurnEmbeddedSubtitleStreamIdx)
	}
}
