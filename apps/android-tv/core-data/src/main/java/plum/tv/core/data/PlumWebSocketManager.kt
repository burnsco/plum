package plum.tv.core.data

import com.squareup.moshi.Moshi
import javax.inject.Inject
import javax.inject.Singleton
import kotlin.coroutines.resume
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.BufferOverflow
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.suspendCancellableCoroutine
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONObject
import plum.tv.core.network.AttachPlaybackSessionCommandJson
import plum.tv.core.network.DetachPlaybackSessionCommandJson
import plum.tv.core.network.PlaybackSessionUpdateEventJson
import plum.tv.core.network.buildPlumWebSocketUrl

@Singleton
class PlumWebSocketManager @Inject constructor(
    private val okHttpClient: OkHttpClient,
    private val prefs: SessionPreferences,
    private val tokenBridge: AuthTokenBridge,
    private val moshi: Moshi,
) {
    private var socket: WebSocket? = null
    private var loopJob: Job? = null

    private val _updates = MutableSharedFlow<PlaybackSessionUpdateEventJson>(
        extraBufferCapacity = 16,
        onBufferOverflow = BufferOverflow.DROP_OLDEST,
    )
    val playbackSessionUpdates: SharedFlow<PlaybackSessionUpdateEventJson> = _updates.asSharedFlow()

    fun start(scope: CoroutineScope) {
        loopJob?.cancel()
        loopJob =
            scope.launch(Dispatchers.IO) {
                while (isActive) {
                    val base = prefs.serverUrl.first()?.trim()?.trimEnd('/') ?: break
                    val token = tokenBridge.bearerToken() ?: break
                    try {
                        awaitSocketSession(base, token)
                    } catch (cancelled: CancellationException) {
                        throw cancelled
                    } catch (_: Exception) {
                    }
                    delay(3_000)
                }
            }
    }

    fun stop() {
        loopJob?.cancel()
        loopJob = null
        socket?.close(1000, "app stop")
        socket = null
    }

    private suspend fun awaitSocketSession(httpBase: String, token: String) {
        val wsUrl = buildPlumWebSocketUrl(httpBase)
        val req =
            Request.Builder()
                .url(wsUrl)
                .header("Authorization", "Bearer $token")
                .build()
        suspendCancellableCoroutine { cont ->
            val ws =
                okHttpClient.newWebSocket(
                    req,
                    object : WebSocketListener() {
                        override fun onOpen(webSocket: WebSocket, response: Response) {
                            socket = webSocket
                        }

                        override fun onMessage(webSocket: WebSocket, text: String) {
                            parseUpdate(text)?.let { _updates.tryEmit(it) }
                        }

                        override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
                            socket = null
                        }

                        override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                            socket = null
                            if (cont.isActive) {
                                cont.resume(Unit)
                            }
                        }

                        override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                            socket = null
                            if (cont.isActive) {
                                cont.resume(Unit)
                            }
                        }
                    },
                )
            cont.invokeOnCancellation { ws.cancel() }
        }
    }

    private fun parseUpdate(text: String): PlaybackSessionUpdateEventJson? {
        return try {
            val o = JSONObject(text)
            if (o.optString("type") != "playback_session_update") {
                null
            } else {
                PlaybackSessionUpdateEventJson(
                    type = o.getString("type"),
                    sessionId = o.getString("sessionId"),
                    delivery = o.getString("delivery"),
                    mediaId = o.getInt("mediaId"),
                    revision = o.getInt("revision"),
                    audioIndex = o.getInt("audioIndex"),
                    status = o.getString("status"),
                    streamUrl = o.getString("streamUrl"),
                    durationSeconds = o.getDouble("durationSeconds"),
                    error = if (o.has("error") && !o.isNull("error")) o.getString("error") else null,
                )
            }
        } catch (_: Exception) {
            null
        }
    }

    fun sendAttach(sessionId: String) {
        val json = moshi.adapter(AttachPlaybackSessionCommandJson::class.java).toJson(
            AttachPlaybackSessionCommandJson(sessionId = sessionId),
        )
        socket?.send(json)
    }

    fun sendDetach(sessionId: String) {
        val json = moshi.adapter(DetachPlaybackSessionCommandJson::class.java).toJson(
            DetachPlaybackSessionCommandJson(sessionId = sessionId),
        )
        socket?.send(json)
    }
}
