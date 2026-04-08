package db

import (
	"regexp"

	"plum/internal/metadata"
)

// SkipFFprobeInScan is set by tests to skip ffprobe during scan (avoids blocking on fake files).
var SkipFFprobeInScan bool

var (
	showKeyNonAlnumRegexp   = regexp.MustCompile(`[^a-z0-9]+`)
	showNameFromTitleRegexp = regexp.MustCompile(`^(.+?)\s*-\s*S\d+`)
)

var (
	videoExtensions = map[string]struct{}{
		".mp4": {}, ".mkv": {}, ".mov": {}, ".avi": {}, ".webm": {}, ".ts": {}, ".m4v": {},
	}
	audioExtensions = map[string]struct{}{
		".mp3": {}, ".flac": {}, ".m4a": {}, ".aac": {}, ".ogg": {}, ".opus": {}, ".wav": {}, ".alac": {},
	}
	readAudioMetadata = metadata.ReadAudioMetadata
	readVideoMetadata = probeVideoMetadata
	computeMediaHash  = computeFileHash
)

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullFloat64(v float64) interface{} {
	if v == 0 {
		return nil
	}
	return v
}
