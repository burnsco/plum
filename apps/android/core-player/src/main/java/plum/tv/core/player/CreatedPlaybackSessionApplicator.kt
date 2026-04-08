package plum.tv.core.player

import plum.tv.core.network.PlaybackSessionJson

/**
 * Applies a [PlaybackSessionJson] returned from [plum.tv.core.data.PlaybackRepository.createSession]:
 * integrate metadata, then branch on [PlaybackSessionJson.delivery]. The host owns all player and
 * transport side effects so this class stays a small stateless router.
 */
interface CreatedPlaybackSessionApplyHost {
    /**
     * Refresh embedded/sidecar track lists from the session and record [PlaybackSessionJson.revision].
     * When [validateBurnAfterMetadata] is true (burn-in reload path), re-validate the active burn
     * stream against the new payload.
     */
    fun integrateSessionTrackMetadata(
        session: PlaybackSessionJson,
        validateBurnAfterMetadata: Boolean,
    )

    suspend fun transitionToDirectPlayback(
        streamUrl: String,
        resumeSec: Float,
        durationSeconds: Double,
    )

    suspend fun transitionToHlsPlayback(session: PlaybackSessionJson, resumeSec: Float)

    fun reportMissingHlsSessionId()

    fun reportUnknownDelivery(delivery: String)
}

class CreatedPlaybackSessionApplicator(
    private val host: CreatedPlaybackSessionApplyHost,
) {
    suspend fun apply(
        session: PlaybackSessionJson,
        resumeSec: Float,
        validateBurnAfterMetadata: Boolean,
    ) {
        host.integrateSessionTrackMetadata(session, validateBurnAfterMetadata)
        when (session.delivery) {
            "direct" ->
                host.transitionToDirectPlayback(
                    session.streamUrl,
                    resumeSec,
                    session.durationSeconds,
                )
            "remux", "transcode" ->
                host.transitionToHlsPlayback(session, resumeSec)
            else ->
                host.reportUnknownDelivery(session.delivery)
        }
    }
}
