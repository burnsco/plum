package plum.tv.core.network

/**
 * Capability report for ExoPlayer on Android TV (native HLS).
 *
 * Do **not** claim AC-3 / E-AC-3 here unless the app ships an FFmpeg (or similar) audio extension:
 * many TV devices have no AC-3 [MediaCodec], so the server would remux/copy Dolby into HLS and
 * playback fails during audio decode (often surfaced as a generic decoder error). Omitting those
 * codecs makes Plum transcode audio to AAC while still copying video when possible.
 */
fun androidTvClientCapabilities(): ClientPlaybackCapabilitiesJson =
    ClientPlaybackCapabilitiesJson(
        supportsNativeHls = true,
        supportsMseHls = false,
        videoCodecs = listOf("h264", "hevc", "vp9", "av1"),
        audioCodecs = listOf("aac", "mp3", "opus", "vorbis"),
        containers = listOf("mp4", "mkv", "webm", "m4v"),
    )
