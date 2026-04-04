// Package ffopts holds small FFmpeg/ffprobe CLI fragments shared across the server.
package ffopts

// inputProbeMatroskaRemux must appear immediately before each "-i" path (or before the input path
// in ffprobe) when opening Matroska/Blu-ray remuxes. Default libav probesize (~5 MiB) is often too
// small for hdmv_pgs_subtitle streams after the first few, which then log "Could not find codec
// parameters ... unspecified size" and may break demuxing.
//
// ffmpeg embedded subtitle extraction uses the same probe: a smaller probesize caused ffmpeg to
// stall or fail stream init on remuxes that ffprobe (with this probe) had already catalogued,
// which surfaced in the UI as subtitle loads that never completed before the client timed out.
var inputProbeMatroskaRemux = []string{
	"-analyzeduration", "100000000", // µs — cap on how much of the timeline libav analyzes
	"-probesize", "134217728", // 128 MiB from file start
}

// InputProbeBeforeI is appended immediately before "-i" / the input path for ffprobe and
// transcoding probe paths.
var InputProbeBeforeI = inputProbeMatroskaRemux

// InputProbeEmbeddedExtract is appended immediately before "-i" when ffmpeg decodes embedded
// text subtitles straight to WebVTT from the container (slow path). Needs the heavy probe for
// difficult Matroska remuxes.
var InputProbeEmbeddedExtract = inputProbeMatroskaRemux

// InputProbeSubtitleDemux is used for stream-copy subtitle extraction to a temp file. A 128 MiB
// probe on every request often delayed the first WebVTT byte by tens of seconds on slow disks
// and made every client look “stuck on loading”. Stream copy usually initializes with far less.
var InputProbeSubtitleDemux = []string{
	"-analyzeduration", "20000000", // 20s
	"-probesize", "33554432", // 32 MiB
}
