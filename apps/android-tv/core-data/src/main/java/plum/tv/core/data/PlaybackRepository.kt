package plum.tv.core.data

import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import plum.tv.core.network.CreatePlaybackSessionPayloadJson
import plum.tv.core.network.PlaybackSessionJson
import plum.tv.core.network.UpdateMediaProgressPayloadJson
import plum.tv.core.network.UpdatePlaybackSessionAudioPayloadJson
import plum.tv.core.network.androidTvClientCapabilities
import plum.tv.core.network.resolveBackendUrl

@Singleton
class PlaybackRepository @Inject constructor(
    private val sessionRepository: SessionRepository,
    private val okHttpClient: OkHttpClient,
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

    /**
     * Overlaps with the server’s own warm on create-session; harmless and reduces time-to-first-subtitle
     * when the user opens subtitles before background extraction finishes.
     */
    suspend fun warmEmbeddedSubtitleCaches(mediaId: Int) {
        val base = sessionRepository.serverUrl.first()?.trim()?.trimEnd('/') ?: return
        val url = "$base/api/media/$mediaId/embedded-subtitles/warm-cache"
        withContext(Dispatchers.IO) {
            val client =
                okHttpClient.newBuilder()
                    .cache(null)
                    .callTimeout(15, TimeUnit.SECONDS)
                    .readTimeout(15, TimeUnit.SECONDS)
                    .build()
            val req =
                Request.Builder()
                    .url(url)
                    .post(byteArrayOf().toRequestBody(null))
                    .build()
            runCatching { client.newCall(req).execute().close() }
        }
    }

    /** True when the master playlist exists and looks parseable (avoids swapping the player to an empty m3u8). */
    suspend fun hlsMasterPlaylistLooksReady(absoluteUrl: String): Boolean =
        withContext(Dispatchers.IO) {
            val client =
                okHttpClient.newBuilder()
                    .cache(null)
                    .callTimeout(5, TimeUnit.SECONDS)
                    .readTimeout(5, TimeUnit.SECONDS)
                    .build()
            val req =
                Request.Builder()
                    .url(absoluteUrl)
                    .header("Cache-Control", "no-cache")
                    .get()
                    .build()
            runCatching {
                client.newCall(req).execute().use { resp ->
                    if (!resp.isSuccessful) return@use false
                    val body = resp.body?.string() ?: return@use false
                    body.startsWith("#EXTM3U") && body.length >= 32
                }
            }.getOrDefault(false)
        }
}
