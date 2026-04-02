package plum.tv.core.network

/** Conservative capability report for ExoPlayer on Android TV (native HLS, common codecs). */
fun androidTvClientCapabilities(): ClientPlaybackCapabilitiesJson =
    ClientPlaybackCapabilitiesJson(
        supportsNativeHls = true,
        supportsMseHls = false,
        videoCodecs = listOf("h264", "hevc", "vp9", "av1"),
        audioCodecs = listOf("aac", "mp3", "opus", "vorbis", "ac3", "eac3"),
        containers = listOf("mp4", "mkv", "webm", "m4v"),
    )
