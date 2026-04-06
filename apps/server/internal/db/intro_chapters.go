package db

import (
	"regexp"
	"strings"
)

// Intro chapter titles: common English + a few romanizations; Jellyfin-style naming.
var introChapterTitlePattern = regexp.MustCompile(
	`(?i)(\bintro\b|\bopening\b|オープニング|オープニングテーマ|\bop\b|` +
		`\bpre[-\s]?credits\b|\bmain\s+titles?\b|\btitles?\s+sequence\b|\bshow\s+intro\b)`,
)

const maxIntroChapterDurationSec = 600

type chapterProbe struct {
	startSec float64
	endSec   float64
	title    string
}

// introChapterGapToleranceSec allows merging two adjacent intro chapters even if there is a
// tiny gap between them (e.g. rounding in chapter timestamps).
const introChapterGapToleranceSec = 0.5

// IntroChapterRangeFromProbes finds chapters whose titles match common intro naming and merges
// contiguous ones into a single range.  Returns ok=false when none match or the range is invalid.
func IntroChapterRangeFromProbes(chapters []chapterProbe) (startSec, endSec float64, ok bool) {
	var mergedStart, mergedEnd float64
	found := false
	for _, ch := range chapters {
		title := strings.TrimSpace(ch.title)
		if title == "" || !introChapterTitlePattern.MatchString(title) {
			if found {
				// Non-intro chapter breaks any contiguous run.
				break
			}
			continue
		}
		if ch.endSec <= ch.startSec || ch.endSec <= 0 {
			continue
		}
		if !found {
			mergedStart = ch.startSec
			mergedEnd = ch.endSec
			found = true
		} else if ch.startSec <= mergedEnd+introChapterGapToleranceSec {
			// Contiguous or overlapping with the current run — extend it.
			if ch.endSec > mergedEnd {
				mergedEnd = ch.endSec
			}
		} else {
			// Gap too large; stop merging.
			break
		}
	}
	if !found {
		return 0, 0, false
	}
	if mergedEnd-mergedStart > maxIntroChapterDurationSec {
		return 0, 0, false
	}
	return mergedStart, mergedEnd, true
}
