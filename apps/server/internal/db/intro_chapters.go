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

// IntroChapterRangeFromProbes picks the first timeline-ordered chapter whose title matches
// common intro naming. Returns ok=false when none match or the range is invalid.
func IntroChapterRangeFromProbes(chapters []chapterProbe) (startSec, endSec float64, ok bool) {
	for _, ch := range chapters {
		title := strings.TrimSpace(ch.title)
		if title == "" || !introChapterTitlePattern.MatchString(title) {
			continue
		}
		if ch.endSec <= ch.startSec || ch.endSec <= 0 {
			continue
		}
		if ch.endSec-ch.startSec > maxIntroChapterDurationSec {
			continue
		}
		return ch.startSec, ch.endSec, true
	}
	return 0, 0, false
}
