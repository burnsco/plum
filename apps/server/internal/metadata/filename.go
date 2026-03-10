package metadata

import (
	"regexp"
	"strconv"
	"strings"
)

// MediaInfo holds parsed hints from a media filename (title, season/episode, year later).
type MediaInfo struct {
	Title   string
	Season  int
	Episode int
	IsTV    bool
}

var (
	tvRegex1 = regexp.MustCompile(`(?i)(.*?)s(\d+)e(\d+)`)
	tvRegex2 = regexp.MustCompile(`(?i)(.*?)(\d+)x(\d+)`)
)

// ParseFilename extracts title and optional season/episode from a filename.
// Non-matching filenames are treated as movie (IsTV false, title from filename).
func ParseFilename(filename string) MediaInfo {
	filename = strings.ReplaceAll(filename, ".", " ")
	filename = strings.ReplaceAll(filename, "_", " ")

	if m := tvRegex1.FindStringSubmatch(filename); len(m) == 4 {
		s, _ := strconv.Atoi(m[2])
		e, _ := strconv.Atoi(m[3])
		return MediaInfo{
			Title:   strings.TrimSpace(m[1]),
			Season:  s,
			Episode: e,
			IsTV:    true,
		}
	}

	if m := tvRegex2.FindStringSubmatch(filename); len(m) == 4 {
		s, _ := strconv.Atoi(m[2])
		e, _ := strconv.Atoi(m[3])
		return MediaInfo{
			Title:   strings.TrimSpace(m[1]),
			Season:  s,
			Episode: e,
			IsTV:    true,
		}
	}

	return MediaInfo{
		Title: strings.TrimSpace(filename),
		IsTV:  false,
	}
}
