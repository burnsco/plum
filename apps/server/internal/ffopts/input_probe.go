// Package ffopts holds small FFmpeg/ffprobe CLI fragments shared across the server.
package ffopts

// InputProbeBeforeI must appear immediately before each "-i" path (or before the input path in
// ffprobe) when opening Matroska/Blu-ray remuxes. Default libav probesize (~5 MiB) is often too
// small for hdmv_pgs_subtitle streams after the first few, which then log "Could not find codec
// parameters ... unspecified size" and may break demuxing.
var InputProbeBeforeI = []string{
	"-analyzeduration", "100000000", // µs — cap on how much of the timeline libav analyzes
	"-probesize", "134217728", // 128 MiB from file start
}
