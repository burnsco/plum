package transcoder

import (
	"strconv"

	"plum/internal/db"
)

// appendTranscodedStreamingAACArgs appends FFmpeg options to encode audio to AAC for
// progressive MP4 and MPEG-TS HLS. Uses the native encoder's fast coder and 48 kHz
// sample rate for predictable browser/TV compatibility.
//
// When outAudioIndex is negative, options apply to the single output audio stream.
// When non-negative, they target output stream N (adaptive HLS with one audio per variant).
func appendTranscodedStreamingAACArgs(args []string, settings db.TranscodingSettings, outAudioIndex int) []string {
	if outAudioIndex < 0 {
		args = append(args,
			"-c:a", "aac",
			"-aac_coder", "fast",
			"-b:a", settings.AudioBitrate,
			"-ar", "48000",
		)
		if settings.AudioChannels > 0 {
			args = append(args, "-ac", strconv.Itoa(settings.AudioChannels))
		}
		return args
	}
	s := strconv.Itoa(outAudioIndex)
	args = append(args,
		"-c:a:"+s, "aac",
		"-aac_coder:a:"+s, "fast",
		"-b:a:"+s, settings.AudioBitrate,
		"-ar:a:"+s, "48000",
	)
	if settings.AudioChannels > 0 {
		args = append(args, "-ac:"+s, strconv.Itoa(settings.AudioChannels))
	}
	return args
}
