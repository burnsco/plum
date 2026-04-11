package db

import "strings"

// EmbeddedSubtitleCodecLikelyBitmap reports codecs that cannot be converted to WebVTT for our pipeline
// (no separate bitmap→VTT path yet). Matches transcoder HLS eligibility and should stay in sync.
func EmbeddedSubtitleCodecLikelyBitmap(codec string) bool {
	c := strings.ToLower(strings.TrimSpace(codec))
	switch c {
	case "hdmv_pgs_subtitle", "pgssub", "pgs", "dvd_subtitle", "dvdsub", "dvb_subtitle", "xsub", "dvb_teletext":
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

// EmbeddedSubtitlePgsBinaryDeliveryEligible is true when the stream can be demuxed to raw HDMV PGS
// (.sup) for clients that decode APPLICATION_PGS (e.g. Media3 on Android TV — same idea as Jellyfin pgssub).
func EmbeddedSubtitlePgsBinaryDeliveryEligible(e EmbeddedSubtitle) bool {
	if e.Supported != nil && !*e.Supported {
		return false
	}
	c := strings.ToLower(strings.TrimSpace(e.Codec))
	switch c {
	case "hdmv_pgs_subtitle", "pgssub", "pgs":
		return true
	default:
		return false
	}
}

// EmbeddedSubtitleAssDeliveryEligible returns true when the stream can be served as ASS for
// clients that render ASS natively (e.g. JASSUB in web browsers). Native ASS/SSA use stream copy;
// subrip/mov_text/webvtt are converted to ASS by ffmpeg on read.
func EmbeddedSubtitleAssDeliveryEligible(e EmbeddedSubtitle) bool {
	if e.Supported != nil && !*e.Supported {
		return false
	}
	if EmbeddedSubtitleCodecLikelyBitmap(e.Codec) {
		return false
	}
	c := strings.ToLower(strings.TrimSpace(e.Codec))
	switch c {
	case "ass", "ssa":
		return true
	case "subrip", "srt", "mov_text", "webvtt":
		return true
	default:
		return false
	}
}

// EmbeddedSubtitleCodecIsNativeASS reports codecs that are already ASS/SSA and can be stream-copied
// for the raw ASS delivery endpoint.
func EmbeddedSubtitleCodecIsNativeASS(codec string) bool {
	c := strings.ToLower(strings.TrimSpace(codec))
	switch c {
	case "ass", "ssa":
		return true
	default:
		return false
	}
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
