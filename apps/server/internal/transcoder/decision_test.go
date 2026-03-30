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

	decision := decidePlayback(42, probe, capabilities, -1)

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

	decision := decidePlayback(42, probe, capabilities, 2)

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
