package plum.tv.core.player

import android.content.Context
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.Player
import androidx.media3.common.TrackSelectionOverride
import androidx.media3.common.Tracks
import androidx.media3.datasource.DataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import plum.tv.core.data.PlaybackRepository
import plum.tv.core.data.PlumWebSocketManager
import plum.tv.core.network.EmbeddedAudioTrackJson
import plum.tv.core.network.PlaybackSessionJson
import plum.tv.core.network.PlaybackSessionUpdateEventJson

/**
 * Core playback controller used by the TV app.
 *
 * Wraps Media3/ExoPlayer + WebSocket session attach/update/detach and implements:
 * - direct URL + HLS playback switching
 * - 10s progress heartbeat + pause/end/completion sync
 * - audio cycling for server-provided transcoded tracks
 * - subtitle cycling for embedded text tracks
 * - resume seek when we start playback with a non-zero resumeSec
 */
class PlumPlayerController(
    private val appContext: Context,
    private val dataSourceFactory: DataSource.Factory,
    private val playbackRepository: PlaybackRepository,
    private val wsManager: PlumWebSocketManager,
    private val scope: CoroutineScope,
    private val mediaId: Int,
    private val resumeSec: Float,
) {
    private var hlsSessionId: String? = null
    private var activeStreamUrl: String? = null
    private var lastDurationSec: Double = 0.0

    @Volatile
    private var embeddedAudioTracks: List<EmbeddedAudioTrackJson> = emptyList()

    @Volatile
    private var serverAudioIndex: Int = -1

    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error.asStateFlow()

    private val _status = MutableStateFlow("Starting…")
    val status: StateFlow<String> = _status.asStateFlow()

    val player: ExoPlayer =
        ExoPlayer.Builder(appContext)
            .setMediaSourceFactory(DefaultMediaSourceFactory(dataSourceFactory))
            .build()

    private var wsCollectJob: Job? = null
    private var progressJob: Job? = null

    init {
        // Player listeners should live on Main because ExoPlayer reads require it.
        scope.launch(Dispatchers.Main) {
            player.addListener(
                object : Player.Listener {
                    override fun onPlaybackStateChanged(playbackState: Int) {
                        if (playbackState == Player.STATE_ENDED) {
                            scope.launch { persistProgressAsync(completed = true) }
                        }
                    }

                    override fun onIsPlayingChanged(isPlaying: Boolean) {
                        if (!isPlaying &&
                            player.playbackState != Player.STATE_ENDED &&
                            player.mediaItemCount > 0 &&
                            player.currentPosition > 0
                        ) {
                            scope.launch { persistProgressAsync(completed = false) }
                        }
                    }
                },
            )
        }

        scope.launch {
            playbackRepository.createSession(mediaId, audioIndex = -1).fold(
                onSuccess = { applyInitialSession(it) },
                onFailure = { e ->
                    _error.value = e.message ?: "Playback failed"
                    _status.value = "Error"
                },
            )
        }

        wsCollectJob =
            scope.launch {
                wsManager.playbackSessionUpdates.collect { ev ->
                    if (ev.mediaId != mediaId) return@collect
                    if (hlsSessionId != null && ev.sessionId != hlsSessionId) return@collect
                    handleSessionUpdate(ev)
                }
            }

        progressJob =
            scope.launch {
                while (isActive) {
                    delay(10_000)
                    if (player.isPlaying) {
                        launch { persistProgressAsync(completed = false) }
                    }
                }
            }
    }

    private fun applyTrackMetadata(session: PlaybackSessionJson) {
        embeddedAudioTracks = session.embeddedAudioTracks.orEmpty()
        serverAudioIndex = session.audioIndex ?: -1
    }

    private suspend fun applyInitialSession(session: PlaybackSessionJson) {
        applyTrackMetadata(session)
        when {
            session.delivery == "direct" -> {
                hlsSessionId = null
                val url = playbackRepository.absoluteStreamUrl(session.streamUrl)
                activeStreamUrl = url
                lastDurationSec = session.durationSeconds
                loadAndPlay(url)
                _status.value = "Playing"
            }

            session.delivery == "remux" || session.delivery == "transcode" -> {
                val sid =
                    session.sessionId ?: run {
                        _error.value = "Missing session id"
                        return
                    }
                hlsSessionId = sid
                wsManager.sendAttach(sid)
                if (session.status == "ready") {
                    val url = playbackRepository.absoluteStreamUrl(session.streamUrl)
                    activeStreamUrl = url
                    lastDurationSec = session.durationSeconds
                    loadAndPlay(url)
                    _status.value = "Playing"
                } else {
                    _status.value = "Preparing stream…"
                }
            }

            else -> {
                _error.value = "Unknown delivery: ${session.delivery}"
            }
        }
    }

    private suspend fun handleSessionUpdate(ev: PlaybackSessionUpdateEventJson) {
        serverAudioIndex = ev.audioIndex
        when (ev.status) {
            "ready" -> {
                val url = playbackRepository.absoluteStreamUrl(ev.streamUrl)
                if (url != activeStreamUrl) {
                    activeStreamUrl = url
                    lastDurationSec = ev.durationSeconds
                    withContext(Dispatchers.Main) {
                        val hadMedia = player.mediaItemCount > 0
                        val wasPlaying = player.isPlaying || player.playWhenReady
                        val pos = player.currentPosition
                        player.setMediaItem(MediaItem.fromUri(url))
                        player.prepare()
                        if (hadMedia) {
                            player.seekTo(pos)
                            player.playWhenReady = wasPlaying
                        } else {
                            if (resumeSec > 0f) {
                                player.seekTo((resumeSec * 1000).toLong())
                            }
                            player.playWhenReady = true
                        }
                    }
                }
                _status.value = "Playing"
            }

            "error" -> {
                _error.value = ev.error ?: "Playback error"
                _status.value = "Error"
            }

            "closed" -> {
                _status.value = "Ended"
            }

            else -> _status.value = "Preparing…"
        }
    }

    private suspend fun loadAndPlay(url: String) {
        withContext(Dispatchers.Main) {
            player.setMediaItem(MediaItem.fromUri(url))
            player.prepare()
            if (resumeSec > 0f) {
                player.seekTo((resumeSec * 1000).toLong())
            }
            player.playWhenReady = true
        }
    }

    private suspend fun persistProgressAsync(completed: Boolean) {
        val (posMs, durMs) =
            withContext(Dispatchers.Main) { player.currentPosition to player.duration }

        val durSec =
            when {
                durMs > 0 && durMs != C.TIME_UNSET -> durMs / 1000.0
                lastDurationSec > 0 -> lastDurationSec
                else -> return
            }

        val posSec = posMs.coerceAtLeast(0) / 1000.0
        withContext(Dispatchers.IO) {
            runCatching {
                playbackRepository.updateProgress(
                    mediaId,
                    positionSec = posSec,
                    durationSec = durSec,
                    completed = completed,
                )
            }
        }
    }

    /** Cycles audio: server stream index when transcoding with multiple tracks, else ExoPlayer audio tracks. */
    fun cycleAudioTrack() {
        scope.launch(Dispatchers.Main) {
            val sid = hlsSessionId
            val tracks = embeddedAudioTracks
            if (sid != null && tracks.size >= 2) {
                val indices = tracks.map { it.streamIndex }.distinct().sorted()
                if (indices.isEmpty()) return@launch

                val cur =
                    if (serverAudioIndex >= 0 && indices.contains(serverAudioIndex)) {
                        serverAudioIndex
                    } else {
                        indices.first()
                    }
                val pos = indices.indexOf(cur).let { if (it < 0) 0 else it }
                val next = indices[(pos + 1) % indices.size]

                val result = withContext(Dispatchers.IO) { playbackRepository.updateSessionAudio(sid, next) }
                result.onFailure { e ->
                    _error.value = e.message ?: "Audio switch failed"
                }
                return@launch
            }
            cycleExoPlayerTrackType(C.TRACK_TYPE_AUDIO)
        }
    }

    /** Cycles embedded text tracks (off → track 0 → … → off). */
    fun cycleSubtitles() {
        scope.launch(Dispatchers.Main) {
            val textGroup = findFirstTrackGroup(C.TRACK_TYPE_TEXT) ?: return@launch
            val g = textGroup
            val mg = g.mediaTrackGroup

            var current = -1
            for (j in 0 until g.length) {
                if (g.isTrackSelected(j)) {
                    current = j
                    break
                }
            }

            val builder = player.trackSelectionParameters.buildUpon()
            builder.clearOverridesOfType(C.TRACK_TYPE_TEXT)
            builder.setTrackTypeDisabled(C.TRACK_TYPE_TEXT, false)

            val next =
                when {
                    current < 0 -> 0
                    current + 1 < g.length -> current + 1
                    else -> -1
                }

            if (next < 0) {
                builder.setTrackTypeDisabled(C.TRACK_TYPE_TEXT, true)
            } else {
                builder.addOverride(TrackSelectionOverride(mg, listOf(next)))
            }
            player.trackSelectionParameters = builder.build()
        }
    }

    private fun findFirstTrackGroup(trackType: Int): Tracks.Group? {
        for (i in 0 until player.currentTracks.groups.size) {
            val group = player.currentTracks.groups[i]
            if (group.type == trackType && group.length > 0) return group
        }
        return null
    }

    private fun cycleExoPlayerTrackType(trackType: Int) {
        val g = findFirstTrackGroup(trackType) ?: return
        if (g.length <= 1) return

        val mg = g.mediaTrackGroup
        var sel = -1
        for (j in 0 until g.length) {
            if (g.isTrackSelected(j)) {
                sel = j
                break
            }
        }
        val next = (sel + 1) % g.length

        val builder = player.trackSelectionParameters.buildUpon()
        builder.clearOverridesOfType(trackType)
        builder.setTrackTypeDisabled(trackType, false)
        builder.addOverride(TrackSelectionOverride(mg, listOf(next)))
        player.trackSelectionParameters = builder.build()
    }

    /**
     * Cleanup that persists progress + closes playback session, then releases ExoPlayer.
     *
     * Uses a dedicated scope so it can complete even as the ViewModel scope is cancelled.
     */
    fun close() {
        progressJob?.cancel()
        wsCollectJob?.cancel()

        hlsSessionId?.let { wsManager.sendDetach(it) }
        val sid = hlsSessionId

        CoroutineScope(SupervisorJob() + Dispatchers.Default).launch {
            val (posMs, durMs, state) =
                withContext(Dispatchers.Main) {
                    Triple(player.currentPosition, player.duration, player.playbackState)
                }

            val durSec =
                when {
                    durMs > 0 && durMs != C.TIME_UNSET -> durMs / 1000.0
                    lastDurationSec > 0 -> lastDurationSec
                    else -> 0.0
                }

            if (durSec > 0) {
                val posSec = posMs.coerceAtLeast(0) / 1000.0
                val ended = state == Player.STATE_ENDED

                withContext(Dispatchers.IO) {
                    runCatching { playbackRepository.updateProgress(mediaId, posSec, durSec, ended) }
                    runCatching { sid?.let { playbackRepository.closeSession(it) } }
                }
            }

            withContext(Dispatchers.Main) { player.release() }
        }
    }
}
