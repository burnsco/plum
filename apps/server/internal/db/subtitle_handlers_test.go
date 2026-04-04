package db

import (
	"strings"
	"testing"
)

func TestTrimFFmpegStderrProgress_DropsSizeLines(t *testing.T) {
	raw := `[matroska @ 0x1] error at demux
size=       0KiB time=00:00:07.25 bitrate=   0.0kbits/s speed=3.63x elapsed=0:00:02.00    00.50
Conversion failed!`
	got := trimFFmpegStderrProgress(raw)
	if got == raw {
		t.Fatalf("expected progress line removed, got %q", got)
	}
	if !strings.Contains(got, "[matroska") || !strings.Contains(got, "Conversion failed!") {
		t.Fatalf("expected error lines kept, got %q", got)
	}
	if strings.Contains(got, "speed=3.63x") {
		t.Fatalf("expected progress fragment dropped, got %q", got)
	}
}

func TestTrimFFmpegStderrProgress_DropsFrameLines(t *testing.T) {
	raw := "header\nframe=  100 fps= 25 q=28.0 size=    1024kB time=00:00:04.00 bitrate=2097.2kbits/s\nfooter"
	got := trimFFmpegStderrProgress(raw)
	if strings.Contains(got, "frame=") {
		t.Fatalf("expected frame progress dropped, got %q", got)
	}
	if !strings.Contains(got, "header") || !strings.Contains(got, "footer") {
		t.Fatalf("expected non-progress kept, got %q", got)
	}
}

func TestTrimFFmpegStderrProgress_FallbackWhenAllProgress(t *testing.T) {
	raw := "size=0KiB time=00:00:01.00 bitrate=1.0kbits/s speed=1x"
	got := trimFFmpegStderrProgress(raw)
	if strings.TrimSpace(got) != strings.TrimSpace(raw) {
		t.Fatalf("expected fallback to raw when filter removes everything, got %q", got)
	}
}
