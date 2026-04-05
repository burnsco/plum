package transcoder

import (
	"fmt"

	"plum/internal/db"
)

// BurnSubtitleError is returned from Create when burnEmbeddedSubtitleStreamIndex is invalid.
type BurnSubtitleError struct {
	Message string
}

func (e BurnSubtitleError) Error() string { return e.Message }

func burnIndexJSON(idx int) *int {
	if idx < 0 {
		return nil
	}
	v := idx
	return &v
}

// burnStreamJSON copies an active burn pointer for JSON (nil = no burn).
func burnStreamJSON(p *int) *int {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// PlaybackEmbeddedSubtitleJSON is the playback-session shape for embedded subtitle tracks
// (full list, including bitmap-only streams clients may burn in via transcode).
type PlaybackEmbeddedSubtitleJSON struct {
	StreamIndex       int    `json:"streamIndex"`
	Language          string `json:"language"`
	Title             string `json:"title"`
	Codec             string `json:"codec,omitempty"`
	Supported         *bool  `json:"supported,omitempty"`
	VttEligible       bool   `json:"vttEligible"`
	PgsBinaryEligible bool   `json:"pgsBinaryEligible"`
}

func embeddedSubtitlesForPlaybackJSON(media db.MediaItem) []PlaybackEmbeddedSubtitleJSON {
	if len(media.EmbeddedSubtitles) == 0 {
		return nil
	}
	out := make([]PlaybackEmbeddedSubtitleJSON, 0, len(media.EmbeddedSubtitles))
	for _, e := range media.EmbeddedSubtitles {
		out = append(out, PlaybackEmbeddedSubtitleJSON{
			StreamIndex:       e.StreamIndex,
			Language:          e.Language,
			Title:             e.Title,
			Codec:             e.Codec,
			Supported:         e.Supported,
			VttEligible:       db.EmbeddedSubtitleWebVTTDeliveryEligible(e),
			PgsBinaryEligible: db.EmbeddedSubtitlePgsBinaryDeliveryEligible(e),
		})
	}
	return out
}

// ValidateBurnEmbeddedSubtitle checks stream index exists in metadata and probe, and is a bitmap codec.
func ValidateBurnEmbeddedSubtitle(probe playbackSourceProbe, media db.MediaItem, streamIndex int) error {
	if streamIndex < 0 {
		return BurnSubtitleError{Message: "invalid burn subtitle stream index"}
	}
	var meta *db.EmbeddedSubtitle
	for i := range media.EmbeddedSubtitles {
		if media.EmbeddedSubtitles[i].StreamIndex == streamIndex {
			meta = &media.EmbeddedSubtitles[i]
			break
		}
	}
	if meta == nil {
		return BurnSubtitleError{Message: fmt.Sprintf("embedded subtitle stream %d not found", streamIndex)}
	}
	if meta.Supported != nil && !*meta.Supported {
		return BurnSubtitleError{Message: fmt.Sprintf("embedded subtitle stream %d is marked unsupported", streamIndex)}
	}
	if !db.EmbeddedSubtitleCodecLikelyBitmap(meta.Codec) {
		return BurnSubtitleError{
			Message: fmt.Sprintf(
				"burn-in is only supported for image-based subtitles (e.g. PGS); stream %d is %q",
				streamIndex,
				meta.Codec,
			),
		}
	}
	found := false
	for _, s := range probe.Streams {
		if s.Index == streamIndex && s.CodecType == "subtitle" {
			found = true
			break
		}
	}
	if !found {
		return BurnSubtitleError{Message: fmt.Sprintf("stream %d is not a subtitle stream in the source file", streamIndex)}
	}
	return nil
}
