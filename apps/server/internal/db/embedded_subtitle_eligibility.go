package db

import "strings"

// EmbeddedSubtitleCodecLikelyBitmap reports codecs that cannot be converted to WebVTT for our pipeline
// (no separate bitmap→VTT path yet). Matches transcoder HLS eligibility and should stay in sync.
func EmbeddedSubtitleCodecLikelyBitmap(codec string) bool {
	c := strings.ToLower(strings.TrimSpace(codec))
	switch c {
	case "hdmv_pgs_subtitle", "pgssub", "dvd_subtitle", "dvdsub", "dvb_subtitle", "xsub", "dvb_teletext":
		return true
	default:
		return false
	}
}

// EmbeddedSubtitleWebVTTDeliveryEligible is true when this stream may be exposed as WebVTT
// (HLS subtitle group, /api/.../embedded/N, Android sideload, warm-cache).
func EmbeddedSubtitleWebVTTDeliveryEligible(e EmbeddedSubtitle) bool {
	if e.Supported != nil && !*e.Supported {
		return false
	}
	return !EmbeddedSubtitleCodecLikelyBitmap(e.Codec)
}

// PlaybackEmbeddedSubtitles returns embedded entries safe for WebVTT APIs and playback JSON.
func PlaybackEmbeddedSubtitles(in []EmbeddedSubtitle) []EmbeddedSubtitle {
	if len(in) == 0 {
		return nil
	}
	out := make([]EmbeddedSubtitle, 0, len(in))
	for i := range in {
		if EmbeddedSubtitleWebVTTDeliveryEligible(in[i]) {
			out = append(out, in[i])
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
