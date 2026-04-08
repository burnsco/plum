package transcoder

import (
	"fmt"

	"plum/internal/db"
)

type PlaybackEmbeddedSubtitleDeliveryMode string

const (
	PlaybackEmbeddedSubtitleDeliveryModeHlsVTT    PlaybackEmbeddedSubtitleDeliveryMode = "hls_vtt"
	PlaybackEmbeddedSubtitleDeliveryModeDirectVTT PlaybackEmbeddedSubtitleDeliveryMode = "direct_vtt"
	PlaybackEmbeddedSubtitleDeliveryModeASS       PlaybackEmbeddedSubtitleDeliveryMode = "ass"
	PlaybackEmbeddedSubtitleDeliveryModePgsBinary PlaybackEmbeddedSubtitleDeliveryMode = "pgs_binary"
	PlaybackEmbeddedSubtitleDeliveryModeBurnIn    PlaybackEmbeddedSubtitleDeliveryMode = "burn_in"
)

type PlaybackEmbeddedSubtitleDeliveryModeJSON struct {
	Mode           PlaybackEmbeddedSubtitleDeliveryMode `json:"mode"`
	RequiresReload bool                                 `json:"requiresReload"`
}

type playbackEmbeddedSubtitleClassification struct {
	LogicalID                    string
	VttEligible                  bool
	PgsBinaryEligible            bool
	AssEligible                  bool
	DeliveryModes                []PlaybackEmbeddedSubtitleDeliveryModeJSON
	PreferredWebDeliveryMode     *PlaybackEmbeddedSubtitleDeliveryMode
	PreferredAndroidDeliveryMode *PlaybackEmbeddedSubtitleDeliveryMode
}

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
	StreamIndex                  int                                        `json:"streamIndex"`
	Language                     string                                     `json:"language"`
	Title                        string                                     `json:"title"`
	Codec                        string                                     `json:"codec,omitempty"`
	LogicalID                    string                                     `json:"logicalId,omitempty"`
	Supported                    *bool                                      `json:"supported,omitempty"`
	Forced                       bool                                       `json:"forced,omitempty"`
	Default                      bool                                       `json:"default,omitempty"`
	HearingImpaired              bool                                       `json:"hearingImpaired,omitempty"`
	VttEligible                  bool                                       `json:"vttEligible"`
	PgsBinaryEligible            bool                                       `json:"pgsBinaryEligible"`
	AssEligible                  bool                                       `json:"assEligible"`
	DeliveryModes                []PlaybackEmbeddedSubtitleDeliveryModeJSON `json:"deliveryModes,omitempty"`
	PreferredWebDeliveryMode     *PlaybackEmbeddedSubtitleDeliveryMode      `json:"preferredWebDeliveryMode,omitempty"`
	PreferredAndroidDeliveryMode *PlaybackEmbeddedSubtitleDeliveryMode      `json:"preferredAndroidDeliveryMode,omitempty"`
}

func addDeliveryMode(
	dst []PlaybackEmbeddedSubtitleDeliveryModeJSON,
	seen map[PlaybackEmbeddedSubtitleDeliveryMode]struct{},
	mode PlaybackEmbeddedSubtitleDeliveryMode,
	requiresReload bool,
) []PlaybackEmbeddedSubtitleDeliveryModeJSON {
	if _, ok := seen[mode]; ok {
		return dst
	}
	seen[mode] = struct{}{}
	return append(dst, PlaybackEmbeddedSubtitleDeliveryModeJSON{
		Mode:           mode,
		RequiresReload: requiresReload,
	})
}

func preferredDeliveryMode(mode PlaybackEmbeddedSubtitleDeliveryMode) *PlaybackEmbeddedSubtitleDeliveryMode {
	v := mode
	return &v
}

func classifyPlaybackEmbeddedSubtitle(
	embedded db.EmbeddedSubtitle,
	playbackDelivery string,
) playbackEmbeddedSubtitleClassification {
	classification := playbackEmbeddedSubtitleClassification{
		LogicalID: db.EmbeddedSubtitleLogicalID(embedded.StreamIndex),
	}
	if embedded.Supported != nil && !*embedded.Supported {
		return classification
	}

	seenModes := make(map[PlaybackEmbeddedSubtitleDeliveryMode]struct{}, 4)
	isBitmap := db.EmbeddedSubtitleCodecLikelyBitmap(embedded.Codec)
	if !isBitmap {
		classification.VttEligible = true
		switch playbackDelivery {
		case "direct":
			classification.DeliveryModes = addDeliveryMode(
				classification.DeliveryModes,
				seenModes,
				PlaybackEmbeddedSubtitleDeliveryModeDirectVTT,
				false,
			)
		default:
			classification.DeliveryModes = addDeliveryMode(
				classification.DeliveryModes,
				seenModes,
				PlaybackEmbeddedSubtitleDeliveryModeHlsVTT,
				false,
			)
		}
	}
	if db.EmbeddedSubtitleAssDeliveryEligible(embedded) {
		classification.AssEligible = true
		classification.DeliveryModes = addDeliveryMode(
			classification.DeliveryModes,
			seenModes,
			PlaybackEmbeddedSubtitleDeliveryModeASS,
			false,
		)
	}
	if db.EmbeddedSubtitlePgsBinaryDeliveryEligible(embedded) {
		classification.PgsBinaryEligible = true
		classification.DeliveryModes = addDeliveryMode(
			classification.DeliveryModes,
			seenModes,
			PlaybackEmbeddedSubtitleDeliveryModePgsBinary,
			false,
		)
		// Browsers still need a burn-in path even when Android can decode PGS natively.
		classification.DeliveryModes = addDeliveryMode(
			classification.DeliveryModes,
			seenModes,
			PlaybackEmbeddedSubtitleDeliveryModeBurnIn,
			true,
		)
	}
	if isBitmap && !classification.PgsBinaryEligible {
		classification.DeliveryModes = addDeliveryMode(
			classification.DeliveryModes,
			seenModes,
			PlaybackEmbeddedSubtitleDeliveryModeBurnIn,
			true,
		)
	}

	switch {
	case classification.AssEligible:
		classification.PreferredWebDeliveryMode = preferredDeliveryMode(PlaybackEmbeddedSubtitleDeliveryModeASS)
	case classification.VttEligible && playbackDelivery == "direct":
		classification.PreferredWebDeliveryMode = preferredDeliveryMode(PlaybackEmbeddedSubtitleDeliveryModeDirectVTT)
	case classification.VttEligible:
		classification.PreferredWebDeliveryMode = preferredDeliveryMode(PlaybackEmbeddedSubtitleDeliveryModeHlsVTT)
	case isBitmap:
		classification.PreferredWebDeliveryMode = preferredDeliveryMode(PlaybackEmbeddedSubtitleDeliveryModeBurnIn)
	case len(classification.DeliveryModes) > 0:
		classification.PreferredWebDeliveryMode = preferredDeliveryMode(classification.DeliveryModes[0].Mode)
	}

	switch {
	case classification.PgsBinaryEligible:
		classification.PreferredAndroidDeliveryMode = preferredDeliveryMode(PlaybackEmbeddedSubtitleDeliveryModePgsBinary)
	case classification.VttEligible && playbackDelivery == "direct":
		classification.PreferredAndroidDeliveryMode = preferredDeliveryMode(PlaybackEmbeddedSubtitleDeliveryModeDirectVTT)
	case classification.VttEligible:
		classification.PreferredAndroidDeliveryMode = preferredDeliveryMode(PlaybackEmbeddedSubtitleDeliveryModeHlsVTT)
	case len(classification.DeliveryModes) > 0:
		classification.PreferredAndroidDeliveryMode = preferredDeliveryMode(classification.DeliveryModes[0].Mode)
	}

	return classification
}

func embeddedSubtitlesForPlaybackJSON(media db.MediaItem, playbackDelivery string) []PlaybackEmbeddedSubtitleJSON {
	if len(media.EmbeddedSubtitles) == 0 {
		return nil
	}
	out := make([]PlaybackEmbeddedSubtitleJSON, 0, len(media.EmbeddedSubtitles))
	for _, e := range media.EmbeddedSubtitles {
		classification := classifyPlaybackEmbeddedSubtitle(e, playbackDelivery)
		out = append(out, PlaybackEmbeddedSubtitleJSON{
			StreamIndex:                  e.StreamIndex,
			Language:                     e.Language,
			Title:                        e.Title,
			Codec:                        e.Codec,
			LogicalID:                    classification.LogicalID,
			Supported:                    e.Supported,
			Forced:                       e.Forced,
			Default:                      e.Default,
			HearingImpaired:              e.HearingImpaired,
			VttEligible:                  classification.VttEligible,
			PgsBinaryEligible:            classification.PgsBinaryEligible,
			AssEligible:                  classification.AssEligible,
			DeliveryModes:                classification.DeliveryModes,
			PreferredWebDeliveryMode:     classification.PreferredWebDeliveryMode,
			PreferredAndroidDeliveryMode: classification.PreferredAndroidDeliveryMode,
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
