package plum.tv.core.network

import com.squareup.moshi.Json

data class ClientPlaybackCapabilitiesJson(
    @Json(name = "supportsNativeHls") val supportsNativeHls: Boolean,
    @Json(name = "supportsMseHls") val supportsMseHls: Boolean,
    @Json(name = "videoCodecs") val videoCodecs: List<String>,
    @Json(name = "audioCodecs") val audioCodecs: List<String>,
    @Json(name = "containers") val containers: List<String>,
)

data class CreatePlaybackSessionPayloadJson(
    @Json(name = "audioIndex") val audioIndex: Int? = null,
    @Json(name = "clientCapabilities") val clientCapabilities: ClientPlaybackCapabilitiesJson? = null,
)

data class UpdatePlaybackSessionAudioPayloadJson(
    @Json(name = "audioIndex") val audioIndex: Int,
)

data class UpdateMediaProgressPayloadJson(
    @Json(name = "position_seconds") val positionSeconds: Double,
    @Json(name = "duration_seconds") val durationSeconds: Double,
    @Json(name = "completed") val completed: Boolean? = null,
)

/** Server returns either direct or HLS session in one shape; optional fields differ by delivery. */
data class PlaybackSessionJson(
    @Json(name = "delivery") val delivery: String,
    @Json(name = "mediaId") val mediaId: Int,
    @Json(name = "sessionId") val sessionId: String? = null,
    @Json(name = "revision") val revision: Int? = null,
    @Json(name = "audioIndex") val audioIndex: Int? = null,
    @Json(name = "status") val status: String,
    @Json(name = "streamUrl") val streamUrl: String,
    @Json(name = "durationSeconds") val durationSeconds: Double,
    @Json(name = "error") val error: String? = null,
    @Json(name = "embeddedAudioTracks") val embeddedAudioTracks: List<EmbeddedAudioTrackJson>? = null,
    @Json(name = "embeddedSubtitles") val embeddedSubtitles: List<EmbeddedSubtitleJson>? = null,
)

data class PlaybackSessionUpdateEventJson(
    @Json(name = "type") val type: String,
    @Json(name = "sessionId") val sessionId: String,
    @Json(name = "delivery") val delivery: String,
    @Json(name = "mediaId") val mediaId: Int,
    @Json(name = "revision") val revision: Int,
    @Json(name = "audioIndex") val audioIndex: Int,
    @Json(name = "status") val status: String,
    @Json(name = "streamUrl") val streamUrl: String,
    @Json(name = "durationSeconds") val durationSeconds: Double,
    @Json(name = "error") val error: String? = null,
)

data class AttachPlaybackSessionCommandJson(
    @Json(name = "action") val action: String = "attach_playback_session",
    @Json(name = "sessionId") val sessionId: String,
)

data class DetachPlaybackSessionCommandJson(
    @Json(name = "action") val action: String = "detach_playback_session",
    @Json(name = "sessionId") val sessionId: String,
)
