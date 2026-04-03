package plum.tv.core.data

import android.util.Log
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
    private companion object {
        const val TAG = "PlumTV"
    }

    private val socketLock = Any()
    private val playbackUpdateAdapter = moshi.adapter(PlaybackSessionUpdateEventJson::class.java)
    private val attachCommandAdapter = moshi.adapter(AttachPlaybackSessionCommandJson::class.java)
    private val detachCommandAdapter = moshi.adapter(DetachPlaybackSessionCommandJson::class.java)
    private val attachedSessionIds = linkedSetOf<String>()

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
                    Log.d(TAG, "ws connect start base=$base")
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
        val currentSocket =
            synchronized(socketLock) {
                val activeSocket = socket
                socket = null
                activeSocket
            }
        currentSocket?.close(1000, "app stop")
    }

    private suspend fun awaitSocketSession(httpBase: String, token: String) {
        val wsUrl = buildPlumWebSocketUrl(httpBase)
        Log.d(TAG, "ws connect url=$wsUrl")
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
                            synchronized(socketLock) {
                                socket = webSocket
                            }
                            Log.i(TAG, "ws open url=$wsUrl")
                            snapshotAttachedSessions().forEach { sessionId ->
                                if (shouldSendTo(webSocket, sessionId)) {
                                    webSocket.send(attachCommandJson(sessionId))
                                }
                            }
                        }

                        override fun onMessage(webSocket: WebSocket, text: String) {
                            parseUpdate(text)?.let { _updates.tryEmit(it) }
                        }

                        override fun onClosing(webSocket: WebSocket, code: Int, reason: String) {
                            Log.i(TAG, "ws closing code=$code reason=$reason")
                            clearSocket(webSocket)
                        }

                        override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                            Log.i(TAG, "ws closed code=$code reason=$reason")
                            clearSocket(webSocket)
                            if (cont.isActive) {
                                cont.resume(Unit)
                            }
                        }

                        override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                            Log.w(TAG, "ws failure error=${t.message}", t)
                            clearSocket(webSocket)
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
        return runCatching { playbackUpdateAdapter.fromJson(text) }.getOrNull()?.takeIf {
            it.type == "playback_session_update"
        }
    }

    fun sendAttach(sessionId: String) {
        Log.d(TAG, "ws attach session=$sessionId")
        val sendNow =
            synchronized(socketLock) {
                attachedSessionIds += sessionId
                socket?.let { activeSocket -> activeSocket to attachCommandJson(sessionId) }
            }
        sendNow?.let { (activeSocket, json) -> activeSocket.send(json) }
    }

    fun sendDetach(sessionId: String) {
        Log.d(TAG, "ws detach session=$sessionId")
        val sendNow =
            synchronized(socketLock) {
                attachedSessionIds -= sessionId
                socket?.let { activeSocket -> activeSocket to detachCommandJson(sessionId) }
            }
        sendNow?.let { (activeSocket, json) -> activeSocket.send(json) }
    }

    private fun clearSocket(webSocket: WebSocket) {
        synchronized(socketLock) {
            if (socket === webSocket) {
                socket = null
            }
        }
    }

    private fun snapshotAttachedSessions(): List<String> =
        synchronized(socketLock) {
            attachedSessionIds.toList()
        }

    private fun shouldSendTo(webSocket: WebSocket, sessionId: String): Boolean =
        synchronized(socketLock) {
            socket === webSocket && sessionId in attachedSessionIds
        }

    private fun attachCommandJson(sessionId: String): String =
        attachCommandAdapter.toJson(AttachPlaybackSessionCommandJson(sessionId = sessionId))

    private fun detachCommandJson(sessionId: String): String =
        detachCommandAdapter.toJson(DetachPlaybackSessionCommandJson(sessionId = sessionId))
}
