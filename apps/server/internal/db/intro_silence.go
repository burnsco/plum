package db

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ffmpegIntroSilenceCommandContext is overridden in tests.
var ffmpegIntroSilenceCommandContext = exec.CommandContext

const (
	// introSilenceScanWindowSec limits decode work: most TV openings finish within a few minutes;
	// keep this modest so library re-scans stay tractable.
	introSilenceScanWindowSec = 180
	// introSilenceFilterMinSec is passed to ffmpeg silencedetect as minimum contiguous silence.
	introSilenceFilterMinSec = 0.65
	// introSilenceNoiseDB is the RMS threshold for silencedetect (more negative = stricter).
	introSilenceNoiseDB = -48
	// introSilenceMinStartSec ignores early gaps (studio logos, padding).
	introSilenceMinStartSec = 25
	// introSilenceMinEndSec rejects trivially short “intros”.
	introSilenceMinEndSec = 20
	// Wall-clock cap: decoding introSilenceScanWindowSec of audio can be slow on big files / NAS.
	introSilenceCmdTimeout = 2 * time.Minute
)

var (
	silenceStartLineRe = regexp.MustCompile(`silence_start:\s*([0-9.]+)`)
	silenceEndLineRe   = regexp.MustCompile(`silence_end:\s*([0-9.]+)\s*\|\s*silence_duration:\s*([0-9.]+)`)
)

type silenceInterval struct {
	start, end, duration float64
}

func parseSilenceDetectOutput(output string) []silenceInterval {
	var out []silenceInterval
	var pendingStart float64
	hasPending := false
	for _, line := range strings.Split(output, "\n") {
		if m := silenceEndLineRe.FindStringSubmatch(line); m != nil {
			end, errE := strconv.ParseFloat(m[1], 64)
			dur, errD := strconv.ParseFloat(m[2], 64)
			if errE != nil || errD != nil {
				hasPending = false
				continue
			}
			if hasPending {
				out = append(out, silenceInterval{
					start:    pendingStart,
					end:      end,
					duration: dur,
				})
			}
			hasPending = false
			continue
		}
		if m := silenceStartLineRe.FindStringSubmatch(line); m != nil {
			st, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				hasPending = false
				continue
			}
			pendingStart = st
			hasPending = true
		}
	}
	return out
}

// pickIntroEndFromSilence returns the end timestamp of the first qualifying silence gap
// (opening-credits style break before main audio).
func pickIntroEndFromSilence(intervals []silenceInterval, durationSec int) (float64, bool) {
	for _, iv := range intervals {
		if iv.start < introSilenceMinStartSec {
			continue
		}
		if iv.end > float64(introSilenceScanWindowSec)+0.5 {
			continue
		}
		if iv.duration+1e-6 < introSilenceFilterMinSec {
			continue
		}
		if iv.end < introSilenceMinEndSec {
			continue
		}
		if durationSec > 0 {
			if iv.end >= float64(durationSec)-8 {
				continue
			}
		}
		return iv.end, true
	}
	return 0, false
}

// probeIntroEndViaSilenceDetect runs ffmpeg silencedetect when chapter metadata did not define an intro.
func probeIntroEndViaSilenceDetect(ctx context.Context, path string, durationSec int) (float64, bool) {
	if strings.TrimSpace(path) == "" {
		return 0, false
	}
	sctx, cancel := context.WithTimeout(ctx, introSilenceCmdTimeout)
	defer cancel()

	filter := fmt.Sprintf("silencedetect=noise=%ddB:d=%.2f", introSilenceNoiseDB, introSilenceFilterMinSec)
	args := []string{
		"-hide_banner", "-nostats", "-loglevel", "info",
		"-i", path,
		"-t", strconv.Itoa(introSilenceScanWindowSec),
		"-map", "0:a:0?",
		"-af", filter,
		"-f", "null",
		"-",
	}
	cmd := ffmpegIntroSilenceCommandContext(sctx, "ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Missing audio, unsupported file, or timeout — skip silently.
		_ = out
		return 0, false
	}
	end, ok := pickIntroEndFromSilence(parseSilenceDetectOutput(string(out)), durationSec)
	if !ok {
		return 0, false
	}
	if durationSec > 0 && end > float64(durationSec) {
		end = float64(durationSec)
	}
	if end > maxIntroChapterDurationSec {
		return 0, false
	}
	return end, true
}
