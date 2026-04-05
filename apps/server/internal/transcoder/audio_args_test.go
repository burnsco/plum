package transcoder

import (
	"testing"

	"plum/internal/db"
)

func TestAppendTranscodedStreamingAACArgs_SingleStream(t *testing.T) {
	settings := db.DefaultTranscodingSettings()
	settings.AudioBitrate = "160k"
	settings.AudioChannels = 2

	args := appendTranscodedStreamingAACArgs(nil, settings, -1)
	if !containsArgPair(args, "-c:a", "aac") || !containsArgPair(args, "-aac_coder", "fast") {
		t.Fatalf("unexpected args: %v", args)
	}
	if !containsArgPair(args, "-b:a", "160k") || !containsArgPair(args, "-ar", "48000") {
		t.Fatalf("unexpected args: %v", args)
	}
	if !containsArgPair(args, "-ac", "2") {
		t.Fatalf("expected -ac 2: %v", args)
	}
}

func TestAppendTranscodedStreamingAACArgs_PassthroughChannels(t *testing.T) {
	settings := db.DefaultTranscodingSettings()
	settings.AudioChannels = 0

	args := appendTranscodedStreamingAACArgs(nil, settings, -1)
	if containsArgs(args, "-ac") {
		t.Fatalf("expected no -ac for channel passthrough: %v", args)
	}
}

func TestAppendTranscodedStreamingAACArgs_PerOutputStreamIndex(t *testing.T) {
	settings := db.DefaultTranscodingSettings()
	settings.AudioBitrate = "192k"
	settings.AudioChannels = 2

	args := appendTranscodedStreamingAACArgs(nil, settings, 1)
	if !containsArgPair(args, "-c:a:1", "aac") {
		t.Fatalf("missing -c:a:1: %v", args)
	}
	if !containsArgPair(args, "-aac_coder:a:1", "fast") {
		t.Fatalf("missing -aac_coder:a:1: %v", args)
	}
	if !containsArgPair(args, "-b:a:1", "192k") || !containsArgPair(args, "-ar:a:1", "48000") {
		t.Fatalf("missing per-stream rate args: %v", args)
	}
	if !containsArgPair(args, "-ac:1", "2") {
		t.Fatalf("missing -ac:1: %v", args)
	}
}
