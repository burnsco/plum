package plum.tv.core.player

import android.content.Context
import android.net.Uri
import android.os.SystemClock
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
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
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import plum.tv.core.data.BrowseRepository
import plum.tv.core.data.PlaybackRepository
import plum.tv.core.data.PlumWebSocketManager
import plum.tv.core.network.EmbeddedAudioTrackJson
import plum.tv.core.network.EmbeddedSubtitleJson
import plum.tv.core.network.LibraryBrowseItemJson
import plum.tv.core.network.PlaybackSessionJson
import plum.tv.core.network.PlaybackSessionUpdateEventJson
import plum.tv.core.network.SubtitleJson
import plum.tv.core.network.ShowEpisodesResponseJson
import kotlin.math.roundToLong
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock

data class PlayerQueueItem(
    val mediaId: Int,
    val title: String,
    val subtitle: String? = null,
)

private data class ProgressPersistSnapshot(
    val mediaId: Int,
    val positionSec: Long,
    val durationSec: Long,
    val completed: Boolean,
)

data class PlayerUiState(
    val title: String = "Playing",
    val subtitle: String? = null,
    val status: String = "Starting…",
    val error: String? = null,
    val positionMs: Long = 0,
    val durationMs: Long = 0,
    val isPlaying: Boolean = false,
    val isBuffering: Boolean = false,
    val canPrev: Boolean = false,
    val canNext: Boolean = false,
    val canCycleAudio: Boolean = false,
    val canCycleSubtitles: Boolean = false,
    val audioTrackLabel: String? = null,
    val subtitleTrackLabel: String? = null,
    val queueIndex: Int = -1,
    val queueSize: Int = 0,
) {
    val progressFraction: Float
        get() = if (durationMs > 0) (positionMs.coerceAtMost(durationMs).toDouble() / durationMs).toFloat() else 0f

    val remainingMs: Long
        get() = (durationMs - positionMs).coerceAtLeast(0)
}

private const val RESTART_PREVIOUS_THRESHOLD_MS = 10_000L
private const val PROGRESS_HEARTBEAT_MS = 10_000L
private const val PROGRESS_DUPLICATE_WINDOW_MS = 3_000L

/**
 * Core playback controller used by the TV app.
 *
 * Wraps Media3/ExoPlayer + WebSocket session attach/update/detach and implements:
 * - direct URL + HLS playback switching
 * - 10s progress heartbeat + pause/end/completion sync
 * - audio cycling for server-provided transcoded tracks
 * - subtitle cycling for embedded text tracks
 * - resume seek when we start playback with a non-zero resumeSec
 * - optional TV episode queue with prev/next switching
 */
class PlumPlayerController(
    private val appContext: Context,
    private val dataSourceFactory: DataSource.Factory,
    private val playbackRepository: PlaybackRepository,
    private val browseRepository: BrowseRepository,
    private val wsManager: PlumWebSocketManager,
    private val scope: CoroutineScope,
    private var mediaId: Int,
    private val resumeSec: Float,
    private val libraryId: Int? = null,
    private val showKey: String? = null,
) {
    private var hlsSessionId: String? = null
    private var activeStreamUrl: String? = null
    private var lastDurationSec: Double = 0.0

    @Volatile
    private var embeddedAudioTracks: List<EmbeddedAudioTrackJson> = emptyList()

    @Volatile
    private var embeddedSubtitleTracks: List<EmbeddedSubtitleJson> = emptyList()

    @Volatile
    private var externalSubtitles: List<SubtitleJson> = emptyList()

    @Volatile
    private var serverAudioIndex: Int = -1

    @Volatile
    private var episodeQueue: List<PlayerQueueItem> = emptyList()

    @Volatile
    private var queueIndex: Int = -1

    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error.asStateFlow()

    private val _status = MutableStateFlow("Starting…")
    val status: StateFlow<String> = _status.asStateFlow()

    private val _uiState =
        MutableStateFlow(
            PlayerUiState(
                status = "Starting…",
            ),
        )
    val uiState: StateFlow<PlayerUiState> = _uiState.asStateFlow()

    val player: ExoPlayer =
        ExoPlayer.Builder(appContext)
            .setMediaSourceFactory(DefaultMediaSourceFactory(dataSourceFactory))
            .build()

    private var wsCollectJob: Job? = null
    private var progressJob: Job? = null
    private var queueLoadJob: Job? = null
    private val progressPersistMutex = Mutex()

    @Volatile
    private var lastPersistedProgress: ProgressPersistSnapshot? = null

    @Volatile
    private var lastPersistedAtMs: Long = 0L

    init {
        // Register the listener synchronously. The constructor is always invoked on the main thread
        // (via PlayerViewModel which Hilt creates on the main thread), so this is safe without a
        // coroutine dispatch. Registering it synchronously eliminates the race where early playback
        // events (e.g. an immediate error) could fire before the listener was attached.
        player.addListener(
            object : Player.Listener {
                override fun onPlaybackStateChanged(playbackState: Int) {
                    when (playbackState) {
                        Player.STATE_ENDED -> {
                            scope.launch { persistProgressAsync(completed = true) }
                            updateStatus("Ended")
                        }
                        Player.STATE_BUFFERING -> refreshUiState()
                        Player.STATE_READY -> refreshUiState()
                    }
                }

                override fun onIsPlayingChanged(isPlaying: Boolean) {
                    refreshUiState()
                    if (!isPlaying &&
                        player.playbackState != Player.STATE_ENDED &&
                        player.mediaItemCount > 0 &&
                        player.currentPosition > 0
                    ) {
                        scope.launch { persistProgressAsync(completed = false) }
                    }
                }

                override fun onMediaItemTransition(mediaItem: MediaItem?, reason: Int) {
                    refreshUiState()
                }

                override fun onTracksChanged(tracks: Tracks) {
                    refreshUiState()
                }

                override fun onPlayerError(error: androidx.media3.common.PlaybackException) {
                    updateError(error.message ?: "Playback error")
                    updateStatus("Error")
                }
            },
        )

        scope.launch {
            openMedia(mediaId = mediaId, resumeSec = resumeSec)
        }

        if (libraryId != null && !showKey.isNullOrBlank()) {
            queueLoadJob =
                scope.launch {
                    loadEpisodeQueue(libraryId, showKey)
                }
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
                // Refresh UI every second, but persist progress less frequently
                var ticks = 0
                val heartbeatSeconds = (PROGRESS_HEARTBEAT_MS / 1000).toInt().coerceAtLeast(1)
                while (isActive) {
                    delay(1_000)
                    refreshUiState()
                    if (player.isPlaying) {
                        ticks++
                        if (ticks >= heartbeatSeconds) {
                            ticks = 0
                            // Run the persist call here (synchronously) to avoid overlapping network calls
                            try {
                                persistProgressAsync(completed = false)
                            } catch (_: Throwable) {
                            }
                        }
                    } else {
                        ticks = 0
                    }
                }
            }
    }

    private fun updateStatus(value: String) {
        _status.value = value
        refreshUiState(statusOverride = value)
    }

    private fun updateError(value: String?) {
        _error.value = value
        refreshUiState(errorOverride = value)
    }

    private fun currentQueueItem(): PlayerQueueItem? =
        episodeQueue.getOrNull(queueIndex)

    private fun currentDurationMs(): Long {
        val durationMs = player.duration
        if (durationMs > 0 && durationMs != C.TIME_UNSET) return durationMs
        return if (lastDurationSec > 0) (lastDurationSec * 1000).toLong() else 0L
    }

    private fun refreshUiState(
        statusOverride: String? = null,
        errorOverride: String? = null,
    ) {
        val item = currentQueueItem()
        val title = item?.title ?: "Playing"
        val subtitle = item?.subtitle
        val durationMs = currentDurationMs()
        val positionMs = player.currentPosition.coerceAtLeast(0)
        val status = statusOverride ?: _status.value
        val error = errorOverride ?: _error.value
        _uiState.value =
            PlayerUiState(
                title = title,
                subtitle = subtitle,
                status = status,
                error = error,
                positionMs = positionMs,
                durationMs = durationMs,
                isPlaying = player.isPlaying,
                isBuffering = player.playbackState == Player.STATE_BUFFERING,
                canPrev = queueIndex > 0 || positionMs > 0,
                canNext = queueIndex >= 0 && queueIndex < episodeQueue.lastIndex,
                canCycleAudio = findFirstTrackGroup(C.TRACK_TYPE_AUDIO)?.length?.let { it > 1 } == true || embeddedAudioTracks.size > 1,
                canCycleSubtitles = externalSubtitles.isNotEmpty() || embeddedSubtitleTracks.isNotEmpty() || findFirstTrackGroup(C.TRACK_TYPE_TEXT)?.length?.let { it > 0 } == true,
                audioTrackLabel = currentAudioTrackLabel(),
                subtitleTrackLabel = currentSubtitleTrackLabel(),
                queueIndex = queueIndex,
                queueSize = episodeQueue.size,
            )
    }

    private fun buildEpisodeQueue(response: ShowEpisodesResponseJson): List<PlayerQueueItem> {
        return response.seasons.flatMap { season ->
            season.episodes.map { episode ->
                val seasonNo = episode.season
                val episodeNo = episode.episode
                val label =
                    when {
                        seasonNo != null && episodeNo != null ->
                            "S${seasonNo.toString().padStart(2, '0')}E${episodeNo.toString().padStart(2, '0')}"
                        seasonNo != null -> "Season $seasonNo"
                        else -> null
                    }
                PlayerQueueItem(
                    mediaId = episode.id,
                    title = episode.title,
                    subtitle = label,
                )
            }
        }
    }

    private suspend fun loadEpisodeQueue(libraryId: Int, showKey: String) {
        browseRepository.showEpisodes(libraryId, showKey).fold(
            onSuccess = { response ->
                episodeQueue = buildEpisodeQueue(response)
                queueIndex = episodeQueue.indexOfFirst { it.mediaId == mediaId }
                refreshUiState()
            },
            onFailure = {
                episodeQueue = emptyList()
                queueIndex = -1
                refreshUiState()
            },
        )
    }

    private fun applyTrackMetadata(session: PlaybackSessionJson) {
        externalSubtitles = session.subtitles.orEmpty()
        embeddedAudioTracks = session.embeddedAudioTracks.orEmpty()
        embeddedSubtitleTracks = session.embeddedSubtitles.orEmpty()
        serverAudioIndex = session.audioIndex ?: -1
    }

    private fun currentAudioTrackLabel(): String? {
        val sid = hlsSessionId
        if (sid != null && embeddedAudioTracks.isNotEmpty()) {
            val current = embeddedAudioTracks.firstOrNull { it.streamIndex == serverAudioIndex } ?: embeddedAudioTracks.firstOrNull()
            return current?.displayLabel() ?: "Track ${serverAudioIndex + 1}"
        }

        val group = findFirstTrackGroup(C.TRACK_TYPE_AUDIO) ?: return null
        for (j in 0 until group.length) {
            if (group.isTrackSelected(j)) {
                return group.mediaTrackGroup.getFormat(j).displayLabel() ?: "Track ${j + 1}"
            }
        }
        return null
    }

    private fun currentSubtitleTrackLabel(): String? {
        if (externalSubtitles.isNotEmpty() || embeddedSubtitleTracks.isNotEmpty() || findFirstTrackGroup(C.TRACK_TYPE_TEXT) != null) {
            val group = findFirstTrackGroup(C.TRACK_TYPE_TEXT)
            if (group != null) {
                for (j in 0 until group.length) {
                    if (group.isTrackSelected(j)) {
                        return group.mediaTrackGroup.getFormat(j).displayLabel() ?: "Track ${j + 1}"
                    }
                }
            }
            return "Off"
        }
        return null
    }

    private fun EmbeddedAudioTrackJson.displayLabel(): String? =
        listOfNotNull(title.trim().takeIf { it.isNotEmpty() }, languageLabel(language)).firstOrNull()

    private fun EmbeddedSubtitleJson.displayLabel(): String? =
        listOfNotNull(title.trim().takeIf { it.isNotEmpty() }, languageLabel(language)).firstOrNull()

    private fun androidx.media3.common.Format.displayLabel(): String? =
        listOfNotNull(label?.trim()?.takeIf { it.isNotEmpty() }, languageLabel(language)).firstOrNull()

    private fun languageLabel(language: String?): String? {
        val trimmed = language?.trim().orEmpty()
        if (trimmed.isEmpty()) return null
        return trimmed.uppercase()
    }

    private suspend fun buildMediaItem(streamUrl: String): MediaItem {
        val subtitleConfigurations = mutableListOf<MediaItem.SubtitleConfiguration>()

        externalSubtitles.forEach { subtitle ->
            val subtitleUrl = playbackRepository.absoluteStreamUrl("/api/subtitles/${subtitle.id}")
            val builder =
                MediaItem.SubtitleConfiguration.Builder(Uri.parse(subtitleUrl))
                    .setMimeType(MimeTypes.TEXT_VTT)
            if (subtitle.language.isNotBlank()) {
                builder.setLanguage(subtitle.language)
            }
            if (subtitle.title.isNotBlank()) {
                builder.setLabel(subtitle.title)
            }
            subtitleConfigurations += builder.build()
        }

        embeddedSubtitleTracks.forEach { subtitle ->
            val subtitleUrl =
                playbackRepository.absoluteStreamUrl("/api/media/$mediaId/subtitles/embedded/${subtitle.streamIndex}")
            val builder =
                MediaItem.SubtitleConfiguration.Builder(Uri.parse(subtitleUrl))
                    .setMimeType(MimeTypes.TEXT_VTT)
            if (subtitle.language.isNotBlank()) {
                builder.setLanguage(subtitle.language)
            }
            if (subtitle.title.isNotBlank()) {
                builder.setLabel(subtitle.title)
            }
            subtitleConfigurations += builder.build()
        }

        return MediaItem.Builder()
            .setUri(streamUrl)
            .setSubtitleConfigurations(subtitleConfigurations)
            .build()
    }

    private suspend fun createAndLoadMedia(mediaId: Int, resumeSec: Float) {
        val audioIndex = serverAudioIndex.takeIf { it >= 0 }
        playbackRepository.createSession(mediaId, audioIndex = audioIndex).fold(
            onSuccess = { session ->
                applyTrackMetadata(session)
                when (session.delivery) {
                    "direct" -> {
                        hlsSessionId = null
                        val url = playbackRepository.absoluteStreamUrl(session.streamUrl)
                        activeStreamUrl = url
                        lastDurationSec = session.durationSeconds
                        loadAndPlay(url, resumeSec)
                        updateStatus("Playing")
                    }
                    "remux", "transcode" -> {
                        val sid =
                            session.sessionId ?: run {
                                updateError("Missing session id")
                                updateStatus("Error")
                                return
                            }
                        hlsSessionId = sid
                        wsManager.sendAttach(sid)
                        if (session.status == "ready") {
                            val url = playbackRepository.absoluteStreamUrl(session.streamUrl)
                            activeStreamUrl = url
                            lastDurationSec = session.durationSeconds
                            loadAndPlay(url, resumeSec)
                            updateStatus("Playing")
                        } else {
                            updateStatus("Preparing stream…")
                        }
                    }
                    else -> {
                        updateError("Unknown delivery: ${session.delivery}")
                        updateStatus("Error")
                    }
                }
            },
            onFailure = { e ->
                updateError(e.message ?: "Playback failed")
                updateStatus("Error")
            },
        )
        refreshUiState()
    }

    private suspend fun openMedia(mediaId: Int, resumeSec: Float) {
        updateError(null)
        refreshUiState(statusOverride = "Preparing stream…", errorOverride = null)
        createAndLoadMedia(mediaId, resumeSec)
    }

    private suspend fun switchToMedia(mediaId: Int, resumeSec: Float = 0f) {
        if (this.mediaId == mediaId && hlsSessionId != null) return
        val previousMediaId = this.mediaId
        val previousSessionId = hlsSessionId
        persistProgressAsync(completed = false, mediaIdOverride = previousMediaId)
        previousSessionId?.let { sid ->
            runCatching { wsManager.sendDetach(sid) }
            runCatching { playbackRepository.closeSession(sid) }
        }
        this.mediaId = mediaId
        queueIndex = episodeQueue.indexOfFirst { it.mediaId == mediaId }
        activeStreamUrl = null
        hlsSessionId = null
        openMedia(mediaId, resumeSec)
    }

    private suspend fun handleSessionUpdate(ev: PlaybackSessionUpdateEventJson) {
        serverAudioIndex = ev.audioIndex
        when (ev.status) {
            "ready" -> {
                val url = playbackRepository.absoluteStreamUrl(ev.streamUrl)
                if (url != activeStreamUrl) {
                    activeStreamUrl = url
                    lastDurationSec = ev.durationSeconds
                    val mediaItem = buildMediaItem(url)
                    withContext(Dispatchers.Main) {
                        val hadMedia = player.mediaItemCount > 0
                        val wasPlaying = player.isPlaying || player.playWhenReady
                        val pos = player.currentPosition
                        player.setMediaItem(mediaItem)
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
                updateStatus("Playing")
            }
            "error" -> {
                updateError(ev.error ?: "Playback error")
                updateStatus("Error")
            }
            "closed" -> {
                updateStatus("Ended")
            }
            else -> updateStatus("Preparing…")
        }
    }

    private suspend fun loadAndPlay(url: String, resumeSec: Float) {
        val mediaItem = buildMediaItem(url)
        withContext(Dispatchers.Main) {
            player.setMediaItem(mediaItem)
            player.prepare()
            if (resumeSec > 0f) {
                player.seekTo((resumeSec * 1000).toLong())
            }
            player.playWhenReady = true
        }
        refreshUiState()
    }

    private suspend fun persistProgressAsync(
        completed: Boolean,
        mediaIdOverride: Int? = null,
    ) {
        val targetMediaId = mediaIdOverride ?: mediaId
        val (posMs, durMs) =
            withContext(Dispatchers.Main) { player.currentPosition to player.duration }

        val durSec =
            when {
                durMs > 0 && durMs != C.TIME_UNSET -> durMs / 1000.0
                lastDurationSec > 0 -> lastDurationSec
                else -> return
            }

        val posSec = posMs.coerceAtLeast(0) / 1000.0
        val snapshot =
            ProgressPersistSnapshot(
                mediaId = targetMediaId,
                positionSec = posSec.roundToLong(),
                durationSec = durSec.roundToLong(),
                completed = completed,
            )

        progressPersistMutex.withLock {
            val now = SystemClock.elapsedRealtime()
            val lastSnapshot = lastPersistedProgress
            if (lastSnapshot == snapshot && now - lastPersistedAtMs < PROGRESS_DUPLICATE_WINDOW_MS) {
                return
            }

            val success =
                runCatching {
                    withContext(Dispatchers.IO) {
                        playbackRepository.updateProgress(
                            targetMediaId,
                            positionSec = posSec,
                            durationSec = durSec,
                            completed = completed,
                        )
                    }
                }.isSuccess

            if (success) {
                lastPersistedProgress = snapshot
                lastPersistedAtMs = now
            }
        }
    }

    fun togglePlayPause() {
        scope.launch(Dispatchers.Main) {
            if (player.isPlaying) {
                player.pause()
            } else {
                player.play()
            }
            refreshUiState()
        }
    }

    fun seekBy(deltaMs: Long) {
        scope.launch(Dispatchers.Main) {
            val duration = currentDurationMs()
            val next = (player.currentPosition + deltaMs).coerceIn(0L, duration.takeIf { it > 0 } ?: Long.MAX_VALUE)
            player.seekTo(next)
            refreshUiState()
        }
    }

    fun rewind10() {
        seekBy(-10_000)
    }

    fun forward10() {
        seekBy(10_000)
    }

    fun previousEpisode() {
        scope.launch {
            val currentPosition = withContext(Dispatchers.Main) { player.currentPosition }
            val prev = if (queueIndex > 0 && currentPosition <= RESTART_PREVIOUS_THRESHOLD_MS) {
                episodeQueue.getOrNull(queueIndex - 1)
            } else {
                null
            }
            if (prev != null) {
                switchToMedia(prev.mediaId, 0f)
            } else {
                scope.launch(Dispatchers.Main) {
                    player.seekTo(0)
                    refreshUiState()
                }
            }
        }
    }

    fun nextEpisode() {
        scope.launch {
            val next = if (queueIndex >= 0 && queueIndex < episodeQueue.lastIndex) episodeQueue.getOrNull(queueIndex + 1) else null
            if (next != null) {
                switchToMedia(next.mediaId, 0f)
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
                result.getOrNull()?.let { applyTrackMetadata(it) }
                result.onFailure { e ->
                    updateError(e.message ?: "Audio switch failed")
                }
                refreshUiState()
                return@launch
            }
            cycleExoPlayerTrackType(C.TRACK_TYPE_AUDIO)
            refreshUiState()
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
            refreshUiState()
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
        queueLoadJob?.cancel()

        hlsSessionId?.let { wsManager.sendDetach(it) }
        val sid = hlsSessionId
        val closingMediaId = mediaId

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
                val ended = state == Player.STATE_ENDED

                try {
                    persistProgressAsync(completed = ended, mediaIdOverride = closingMediaId)
                } catch (_: Throwable) {
                }
                withContext(Dispatchers.IO) {
                    runCatching { sid?.let { playbackRepository.closeSession(it) } }
                }
            }

            withContext(Dispatchers.Main) { player.release() }
        }
    }
}
