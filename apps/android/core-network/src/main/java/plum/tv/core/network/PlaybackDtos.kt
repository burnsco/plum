package plum.tv.core.network

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class ClientPlaybackCapabilitiesJson(
    @param:Json(name = "supportsNativeHls") val supportsNativeHls: Boolean,
    @param:Json(name = "supportsMseHls") val supportsMseHls: Boolean,
    @param:Json(name = "videoCodecs") val videoCodecs: List<String>,
    @param:Json(name = "audioCodecs") val audioCodecs: List<String>,
    @param:Json(name = "containers") val containers: List<String>,
)

@JsonClass(generateAdapter = true)
data class CreatePlaybackSessionPayloadJson(
    @param:Json(name = "audioIndex") val audioIndex: Int? = null,
    @param:Json(name = "clientCapabilities") val clientCapabilities: ClientPlaybackCapabilitiesJson? = null,
    @param:Json(name = "burnEmbeddedSubtitleStreamIndex") val burnEmbeddedSubtitleStreamIndex: Int? = null,
)

@JsonClass(generateAdapter = true)
data class UpdatePlaybackSessionAudioPayloadJson(
    @param:Json(name = "audioIndex") val audioIndex: Int,
)

@JsonClass(generateAdapter = true)
data class UpdateMediaProgressPayloadJson(
    @param:Json(name = "position_seconds") val positionSeconds: Double,
    @param:Json(name = "duration_seconds") val durationSeconds: Double,
    @param:Json(name = "completed") val completed: Boolean? = null,
)

/** Server returns either direct or HLS session in one shape; optional fields differ by delivery. */
@JsonClass(generateAdapter = true)
data class PlaybackSessionJson(
    @param:Json(name = "delivery") val delivery: String,
    @param:Json(name = "mediaId") val mediaId: Int,
    @param:Json(name = "sessionId") val sessionId: String? = null,
    @param:Json(name = "revision") val revision: Int? = null,
    @param:Json(name = "audioIndex") val audioIndex: Int? = null,
    @param:Json(name = "status") val status: String,
    @param:Json(name = "streamUrl") val streamUrl: String,
    @param:Json(name = "durationSeconds") val durationSeconds: Double,
    @param:Json(name = "error") val error: String? = null,
    @param:Json(name = "subtitles") val subtitles: List<SubtitleJson>? = null,
    @param:Json(name = "embeddedAudioTracks") val embeddedAudioTracks: List<EmbeddedAudioTrackJson>? = null,
    @param:Json(name = "embeddedSubtitles") val embeddedSubtitles: List<EmbeddedSubtitleJson>? = null,
    @param:Json(name = "intro_start_seconds") val introStartSeconds: Double? = null,
    @param:Json(name = "intro_end_seconds") val introEndSeconds: Double? = null,
    @param:Json(name = "credits_start_seconds") val creditsStartSeconds: Double? = null,
    @param:Json(name = "credits_end_seconds") val creditsEndSeconds: Double? = null,
)

/** Must match `PlaybackSessionUpdateEvent` / `PlaybackSessionUpdateEventSchema` in @plum/contracts. */
@JsonClass(generateAdapter = true)
data class PlaybackSessionUpdateEventJson(
    @param:Json(name = "type") val type: String,
    @param:Json(name = "sessionId") val sessionId: String,
    @param:Json(name = "delivery") val delivery: String,
    @param:Json(name = "mediaId") val mediaId: Int,
    @param:Json(name = "revision") val revision: Int? = null,
    @param:Json(name = "audioIndex") val audioIndex: Int,
    @param:Json(name = "status") val status: String,
    @param:Json(name = "streamUrl") val streamUrl: String,
    @param:Json(name = "durationSeconds") val durationSeconds: Double,
    @param:Json(name = "error") val error: String? = null,
    @param:Json(name = "burnEmbeddedSubtitleStreamIndex") val burnEmbeddedSubtitleStreamIndex: Int? = null,
    @param:Json(name = "intro_start_seconds") val introStartSeconds: Double? = null,
    @param:Json(name = "intro_end_seconds") val introEndSeconds: Double? = null,
    @param:Json(name = "credits_start_seconds") val creditsStartSeconds: Double? = null,
    @param:Json(name = "credits_end_seconds") val creditsEndSeconds: Double? = null,
)

@JsonClass(generateAdapter = true)
data class AttachPlaybackSessionCommandJson(
    @param:Json(name = "action") val action: String = "attach_playback_session",
    @param:Json(name = "sessionId") val sessionId: String,
)

@JsonClass(generateAdapter = true)
data class DetachPlaybackSessionCommandJson(
    @param:Json(name = "action") val action: String = "detach_playback_session",
    @param:Json(name = "sessionId") val sessionId: String,
)
