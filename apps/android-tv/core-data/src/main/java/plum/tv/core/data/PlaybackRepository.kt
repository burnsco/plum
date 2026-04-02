package plum.tv.core.data

import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.first
import plum.tv.core.network.CreatePlaybackSessionPayloadJson
import plum.tv.core.network.PlaybackSessionJson
import plum.tv.core.network.UpdateMediaProgressPayloadJson
import plum.tv.core.network.UpdatePlaybackSessionAudioPayloadJson
import plum.tv.core.network.androidTvClientCapabilities
import plum.tv.core.network.resolveBackendUrl

@Singleton
class PlaybackRepository @Inject constructor(
    private val sessionRepository: SessionRepository,
) {
    suspend fun createSession(mediaId: Int, audioIndex: Int? = null): Result<PlaybackSessionJson> = runCatching {
        val api = sessionRepository.getPlumApi()
        val payload = CreatePlaybackSessionPayloadJson(
            audioIndex = audioIndex,
            clientCapabilities = androidTvClientCapabilities(),
        )
        val res = api.createPlaybackSession(mediaId, payload)
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "createPlaybackSession: HTTP ${res.code()}")
        }
        res.body() ?: error("Empty playback session")
    }

    suspend fun updateProgress(mediaId: Int, positionSec: Double, durationSec: Double, completed: Boolean? = null) {
        val api = sessionRepository.getPlumApi()
        val res = api.updateMediaProgress(
            mediaId,
            UpdateMediaProgressPayloadJson(
                positionSeconds = positionSec,
                durationSeconds = durationSec,
                completed = completed,
            ),
        )
        if (!res.isSuccessful) {
            throw IllegalStateException(res.errorBody()?.string() ?: "progress: ${res.code()}")
        }
    }

    suspend fun updateSessionAudio(sessionId: String, audioIndex: Int): Result<PlaybackSessionJson> = runCatching {
        val api = sessionRepository.getPlumApi()
        val res = api.updatePlaybackSessionAudio(sessionId, UpdatePlaybackSessionAudioPayloadJson(audioIndex))
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "update audio: ${res.code()}")
        }
        res.body() ?: error("Empty session")
    }

    suspend fun closeSession(sessionId: String) {
        runCatching {
            val api = sessionRepository.getPlumApi()
            api.closePlaybackSession(sessionId)
        }
    }

    suspend fun absoluteStreamUrl(streamUrl: String): String {
        val base = sessionRepository.serverUrl.first()?.trim()?.trimEnd('/')
            ?: error("Server URL not set")
        return resolveBackendUrl(base, streamUrl)
    }
}
