package plum.tv.core.player

import android.app.ActivityManager
import android.content.Context
import androidx.annotation.OptIn
import android.net.Uri
import android.os.Looper
import android.os.SystemClock
import androidx.media3.common.C
import androidx.media3.common.Format
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.Player
import androidx.media3.common.TrackSelectionOverride
import androidx.media3.common.Tracks
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.DataSource
import androidx.media3.datasource.HttpDataSource
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.trackselection.DefaultTrackSelector
import androidx.media3.exoplayer.Renderer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.exoplayer.text.TextOutput
import androidx.media3.exoplayer.text.TextRenderer
import androidx.media3.extractor.DefaultExtractorsFactory
import androidx.media3.extractor.ts.TsExtractor
import java.util.ArrayList
import java.util.Locale
import java.util.concurrent.atomic.AtomicBoolean
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeoutOrNull
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
    val backdropPath: String? = null,
    val backdropUrl: String? = null,
    val showPosterPath: String? = null,
    val showPosterUrl: String? = null,
)

private data class ProgressPersistSnapshot(
    val mediaId: Int,
    val positionSec: Long,
    val durationSec: Long,
    val completed: Boolean,
)

/** Preserves subtitle choice across [setMediaItem] when audio switch replaces the HLS timeline. */
private data class SubtitleRestoreState(
    val disabled: Boolean,
    val language: String?,
    val label: String?,
    /** [MediaItem.SubtitleConfiguration.Builder.setId] / [Format.id] when sideloading; HLS uses manifest ids. */
    val configurationId: String?,
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
    val showSkipIntro: Boolean = false,
) {
    val progressFraction: Float
        get() = if (durationMs > 0) (positionMs.coerceAtMost(durationMs).toDouble() / durationMs).toFloat() else 0f

    val remainingMs: Long
        get() = (durationMs - positionMs).coerceAtLeast(0)
}

/** Plex-style “up next” overlay when an episode ends and another is queued. */
data class UpNextOverlayState(
    val secondsRemaining: Int,
    val title: String,
    val subtitle: String?,
    val backdropPath: String?,
    val backdropUrl: String?,
    val showPosterPath: String?,
    val showPosterUrl: String?,
)

private const val RESTART_PREVIOUS_THRESHOLD_MS = 10_000L
private const val UPNEXT_COUNTDOWN_SECONDS = 10
private const val PROGRESS_HEARTBEAT_MS = 10_000L
private const val PROGRESS_DUPLICATE_WINDOW_MS = 3_000L
private const val INTRO_SKIP_LEADING_SLACK_SEC = 0.35
private const val INTRO_SKIP_TRAILING_SLACK_SEC = 0.35

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
@OptIn(UnstableApi::class)
class PlumPlayerController(
    private val appContext: Context,
    private val dataSourceFactory: DataSource.Factory,
    private val playbackRepository: PlaybackRepository,
    private val browseRepository: BrowseRepository,
    private val wsManager: PlumWebSocketManager,
    private val scope: CoroutineScope,
    private val applicationScope: CoroutineScope,
    private var mediaId: Int,
    private val resumeSec: Float,
    private val libraryId: Int? = null,
    private val showKey: String? = null,
    private val navDisplayTitle: String? = null,
    private val navDisplaySubtitle: String? = null,
) {
    private var hlsSessionId: String? = null
    private var activeStreamUrl: String? = null
    private var lastDurationSec: Double = 0.0

    /** Last HLS revision we applied; audio switches bump revision even when URL normalization matches. */
    @Volatile
    private var lastAppliedStreamRevision: Int = -1

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

    @Volatile
    private var introStartSec: Double? = null

    @Volatile
    private var introEndSec: Double? = null

    @Volatile
    private var sessionIntroSkipMode: String = "manual"

    @Volatile
    private var introAutoSkipConsumed: Boolean = false

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

    private val _trackPicker = MutableStateFlow<TrackPicker?>(null)
    val trackPicker: StateFlow<TrackPicker?> = _trackPicker.asStateFlow()

    private val _upNext = MutableStateFlow<UpNextOverlayState?>(null)
    val upNext: StateFlow<UpNextOverlayState?> = _upNext.asStateFlow()

    /** Wall-clock millis for the controls overlay clock; in its own flow so uiState deduplication works during pause. */
    private val _wallClock = MutableStateFlow(System.currentTimeMillis())
    val wallClock: StateFlow<Long> = _wallClock.asStateFlow()

    /**
     * Sideloaded WebVTT (sidecar + embedded-extract URLs) still flows through the text renderer as
     * legacy `text/vtt` samples in common Media3/HLS merge setups; without this, tracks appear in the
     * picker but no cues reach [androidx.media3.ui.PlayerView]'s subtitle overlay.
     * See [androidx/media#1606](https://github.com/androidx/media/issues/1606).
     */
    @Suppress("DEPRECATION")
    private val renderersFactory =
        object : DefaultRenderersFactory(appContext) {
            init {
                // TVs often advertise HEVC (e.g. Main 10 in MKV) as supported but the primary
                // hardware decoder then fails at configure/decode; allow ExoPlayer to fall back.
                setEnableDecoderFallback(true)
            }

            override fun buildTextRenderers(
                context: Context,
                output: TextOutput,
                outputLooper: android.os.Looper,
                extensionRendererMode: Int,
                out: ArrayList<Renderer>,
            ) {
                val textRenderer = TextRenderer(output, outputLooper)
                textRenderer.experimentalSetLegacyDecodingEnabled(true)
                out.add(textRenderer)
            }
        }

    /**
     * Low-RAM TV devices: smaller TS probe window (Jellyfin-style) so timestamp discovery is cheaper.
     * CBR-friendly seeking for .ts sidecars and some broadcast captures.
     */
    private val mediaExtractorsFactory =
        DefaultExtractorsFactory().apply {
            val lowRam = appContext.getSystemService(Context.ACTIVITY_SERVICE) as? ActivityManager
            setTsExtractorTimestampSearchBytes(
                if (lowRam?.isLowRamDevice == true) {
                    TsExtractor.TS_PACKET_SIZE * 1800
                } else {
                    TsExtractor.DEFAULT_TIMESTAMP_SEARCH_BYTES
                },
            )
            setConstantBitrateSeekingEnabled(true)
            setConstantBitrateSeekingAlwaysEnabled(true)
        }

    private val trackSelector =
        DefaultTrackSelector(appContext).apply {
            parameters = buildUponParameters().setTunnelingEnabled(false).build()
        }

    val player: ExoPlayer =
        ExoPlayer.Builder(appContext, renderersFactory)
            .setTrackSelector(trackSelector)
            .setMediaSourceFactory(DefaultMediaSourceFactory(dataSourceFactory, mediaExtractorsFactory))
            .build()

    private var wsCollectJob: Job? = null
    private var progressJob: Job? = null
    private var queueLoadJob: Job? = null
    private var upNextJob: Job? = null
    /** Polls until a new revision’s HLS master is readable; cancelled when WebSocket applies the swap. */
    private var revisionReadyPollJob: Job? = null
    private val progressPersistMutex = Mutex()

    /** Prevents double [close] and use-after-[androidx.media3.exoplayer.ExoPlayer.release]. */
    private val controllerClosed = AtomicBoolean(false)

    @Volatile
    private var lastPersistedProgress: ProgressPersistSnapshot? = null

    @Volatile
    private var lastPersistedAtMs: Long = 0L

    @Volatile
    private var pendingSubtitleRestore: SubtitleRestoreState? = null

    /**
     * After each new [MediaItem], Exo may auto-select in-band CEA-608/708 from the video stream
     * over Plum WebVTT sideloads or HLS subtitle renditions. Prefer the first non-CEA text track once.
     */
    @Volatile
    private var preferNonCea608TextAfterLoad: Boolean = false

    /** Filled when movie playback has no episode queue but we can resolve title from the API. */
    @Volatile
    private var fetchedStandaloneTitle: String? = null

    @Volatile
    private var fetchedStandaloneSubtitle: String? = null

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
                            scheduleUpNextCountdown()
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
                    restorePendingSubtitlesIfNeeded()
                    preferNonCea608TextTrackOnceIfNeeded()
                    refreshUiState()
                }

                override fun onPlayerError(error: androidx.media3.common.PlaybackException) {
                    val detail = describePlaybackError(error)
                    android.util.Log.w("PlumTV", "player error: $detail", error)
                    updateError(detail)
                    updateStatus("Error")
                }
            },
        )

        scope.launch {
            openMedia(mediaId = mediaId, resumeSec = resumeSec)
        }

        if (showKey.isNullOrBlank() &&
            (libraryId ?: 0) > 0 &&
            navDisplayTitle.isNullOrBlank()
        ) {
            val lid = libraryId!!
            scope.launch(Dispatchers.IO) {
                browseRepository.movieDetails(lid, mediaId).onSuccess { d ->
                    fetchedStandaloneTitle = d.title
                    fetchedStandaloneSubtitle = d.releaseDate?.take(4)?.takeIf { it.length == 4 }
                    withContext(Dispatchers.Main) { refreshUiState() }
                }
            }
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
                    _wallClock.value = System.currentTimeMillis()
                    maybeAutoSkipIntro()
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

    private fun describePlaybackError(error: androidx.media3.common.PlaybackException): String {
        val codeName = runCatching { error.errorCodeName }.getOrNull()?.takeIf { it.isNotBlank() }
        val chain = generateSequence(error as Throwable?) { it.cause }.toList()
        val httpInvalid =
            chain.firstOrNull { it is HttpDataSource.InvalidResponseCodeException }
                as? HttpDataSource.InvalidResponseCodeException
        val baseMessage =
            when {
                httpInvalid != null ->
                    buildString {
                        append("HTTP ")
                        append(httpInvalid.responseCode)
                        httpInvalid.responseMessage?.trim()?.takeIf { it.isNotBlank() }?.let {
                            append(" ")
                            append(it)
                        }
                    }
                else -> {
                    // IO_UNSPECIFIED often wraps an IOException with a useful message on an inner cause.
                    val innerFirst =
                        chain.asReversed().firstOrNull { t ->
                            t.message?.trim()?.isNotBlank() == true &&
                                t !is androidx.media3.common.PlaybackException
                        }
                    val detail =
                        innerFirst?.message?.trim()
                            ?: chain.asReversed().firstOrNull { t -> t.message?.trim()?.isNotBlank() == true }
                                ?.message?.trim()
                    detail ?: error.message?.trim()?.takeUnless { it.isBlank() } ?: "Playback error"
                }
            }
        return if (codeName.isNullOrBlank()) baseMessage else "$baseMessage ($codeName)"
    }

    private fun currentQueueItem(): PlayerQueueItem? =
        episodeQueue.getOrNull(queueIndex)

    /**
     * ExoPlayer sometimes reports only the first HLS playlist window as [Player.getDuration] while
     * transcoded/remuxed Plum sessions are full-length VOD. Prefer the longer of player vs server
     * when both are known so the scrubber and seek bounds match the real runtime.
     */
    private fun resolvedDurationMs(): Long {
        val exoMs = player.duration
        val serverMs = (lastDurationSec * 1000.0).toLong().coerceAtLeast(0L)
        val exoValid = exoMs > 0 && exoMs != C.TIME_UNSET
        return when {
            !exoValid && serverMs > 0L -> serverMs
            exoValid && serverMs <= 0L -> exoMs
            exoValid && serverMs > 0L -> maxOf(exoMs, serverMs)
            else -> 0L
        }
    }

    private fun currentDurationMs(): Long = resolvedDurationMs()

    private fun normalizeIntroSkipMode(raw: String?): String =
        when (raw?.trim()?.lowercase()) {
            "off", "manual", "auto" -> raw.trim().lowercase()
            else -> "manual"
        }

    private fun effectiveIntroSkipMode(): String = normalizeIntroSkipMode(sessionIntroSkipMode)

    private fun positionInsideIntroWindow(positionSec: Double): Boolean {
        val end = introEndSec ?: return false
        val start = introStartSec ?: 0.0
        return positionSec >= start - INTRO_SKIP_LEADING_SLACK_SEC &&
            positionSec < end - INTRO_SKIP_TRAILING_SLACK_SEC
    }

    /**
     * Seeks to the end of the detected intro window when the server provided bounds and the
     * library is not in "off" mode.
     */
    fun skipIntro() {
        if (effectiveIntroSkipMode() == "off") return
        val end = introEndSec ?: return
        scope.launch(Dispatchers.Main) {
            val targetMs = (end * 1000.0).toLong().coerceIn(0L, resolvedDurationMs())
            if (targetMs > player.currentPosition) {
                player.seekTo(targetMs)
            }
            introAutoSkipConsumed = true
        }
    }

    private fun maybeAutoSkipIntro() {
        if (!player.isPlaying) return
        if (effectiveIntroSkipMode() != "auto") return
        if (introAutoSkipConsumed) return
        val end = introEndSec ?: return
        val start = introStartSec ?: 0.0
        val posSec = player.currentPosition.coerceAtLeast(0) / 1000.0
        if (posSec < start - INTRO_SKIP_LEADING_SLACK_SEC) return
        if (posSec >= end - INTRO_SKIP_TRAILING_SLACK_SEC) return
        introAutoSkipConsumed = true
        scope.launch(Dispatchers.Main) {
            val targetMs = (end * 1000.0).toLong().coerceIn(0L, resolvedDurationMs())
            if (targetMs > player.currentPosition) {
                player.seekTo(targetMs)
            }
        }
    }

    private fun mergeIntroFromSession(
        skipMode: String?,
        start: Double?,
        end: Double?,
    ) {
        skipMode?.trim()?.takeIf { it.isNotEmpty() }?.let { sessionIntroSkipMode = it }
        start?.let { introStartSec = it }
        end?.let { introEndSec = it }
    }

    private fun refreshUiState(
        statusOverride: String? = null,
        errorOverride: String? = null,
    ) {
        val item = currentQueueItem()
        val title =
            item?.title?.takeIf { it.isNotBlank() }
                ?: navDisplayTitle?.trim()?.takeIf { it.isNotEmpty() }
                ?: fetchedStandaloneTitle?.trim()?.takeIf { it.isNotEmpty() }
                ?: "Playing"
        val subtitle =
            item?.subtitle?.takeIf { it.isNotBlank() }
                ?: navDisplaySubtitle?.trim()?.takeIf { it.isNotEmpty() }
                ?: fetchedStandaloneSubtitle?.trim()?.takeIf { it.isNotEmpty() }
        val durationMs = currentDurationMs()
        val positionMs = player.currentPosition.coerceAtLeast(0)
        val positionSec = positionMs / 1000.0
        val status = statusOverride ?: _status.value
        val error = errorOverride ?: _error.value
        val mode = effectiveIntroSkipMode()
        val showSkipIntro =
            mode == "manual" &&
                introEndSec != null &&
                positionInsideIntroWindow(positionSec) &&
                player.mediaItemCount > 0 &&
                error == null
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
                canCycleAudio = serverEmbeddedAudioChoiceCount() > 1 || exoAudioTrackCount() > 1,
                // Subtitle picker lists Exo text tracks once manifests are loaded (sidecars + embedded).
                canCycleSubtitles = trackGroups(C.TRACK_TYPE_TEXT).isNotEmpty(),
                audioTrackLabel = currentAudioTrackLabel(),
                subtitleTrackLabel = currentSubtitleTrackLabel(),
                queueIndex = queueIndex,
                queueSize = episodeQueue.size,
                showSkipIntro = showSkipIntro,
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
                    backdropPath = episode.backdropPath,
                    backdropUrl = episode.backdropUrl,
                    showPosterPath = episode.showPosterPath,
                    showPosterUrl = episode.showPosterUrl,
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

    /** Full replace when starting a new playback session (avoids carrying lists across [switchToMedia]). */
    private fun replaceTrackMetadataFromSession(session: PlaybackSessionJson) {
        externalSubtitles = session.subtitles.orEmpty()
        embeddedAudioTracks = session.embeddedAudioTracks.orEmpty()
        embeddedSubtitleTracks = session.embeddedSubtitles.orEmpty()
        serverAudioIndex = session.audioIndex ?: -1
        introStartSec = session.introStartSeconds
        introEndSec = session.introEndSeconds
        sessionIntroSkipMode = session.introSkipMode?.trim()?.takeIf { it.isNotEmpty() } ?: "manual"
        introAutoSkipConsumed = false
    }

    /**
     * Partial merge for PATCH responses: Go uses `omitempty` on slices, so a missing key must not
     * clear embedded track lists we already have for this session.
     */
    private fun mergeTrackMetadataFromSession(session: PlaybackSessionJson) {
        session.subtitles?.let { externalSubtitles = it }
        session.embeddedAudioTracks?.let { embeddedAudioTracks = it }
        session.embeddedSubtitles?.let { embeddedSubtitleTracks = it }
        session.audioIndex?.let { serverAudioIndex = it }
        mergeIntroFromSession(
            session.introSkipMode,
            session.introStartSeconds,
            session.introEndSeconds,
        )
    }

    private fun trackGroups(trackType: Int): List<Tracks.Group> =
        player.currentTracks.groups.filter { it.type == trackType && it.length > 0 }

    private fun exoAudioTrackCount(): Int =
        trackGroups(C.TRACK_TYPE_AUDIO).sumOf { it.length }

    /** Distinct server-side audio stream indices (transcode/remux session). */
    private fun serverEmbeddedAudioChoiceCount(): Int =
        embeddedAudioTracks.map { it.streamIndex }.distinct().size

    private fun selectedAudioFormat(): Format? {
        for (g in trackGroups(C.TRACK_TYPE_AUDIO)) {
            for (j in 0 until g.length) {
                if (g.isTrackSelected(j)) return g.mediaTrackGroup.getFormat(j)
            }
        }
        return null
    }

    private fun selectedSubtitleFormat(): Format? {
        for (g in trackGroups(C.TRACK_TYPE_TEXT)) {
            for (j in 0 until g.length) {
                if (g.isTrackSelected(j)) return g.mediaTrackGroup.getFormat(j)
            }
        }
        return null
    }

    private fun currentAudioTrackLabel(): String? {
        val sid = hlsSessionId
        if (sid != null && embeddedAudioTracks.isNotEmpty()) {
            val current = embeddedAudioTracks.firstOrNull { it.streamIndex == serverAudioIndex } ?: embeddedAudioTracks.firstOrNull()
            return current?.displayLabel() ?: "Track ${serverAudioIndex + 1}"
        }

        return selectedAudioFormat()?.displayLabel()
    }

    private fun currentSubtitleTrackLabel(): String? {
        if (trackGroups(C.TRACK_TYPE_TEXT).isEmpty()) return null
        return selectedSubtitleFormat()?.displayLabel() ?: "Off"
    }

    private fun EmbeddedAudioTrackJson.displayLabel(): String? =
        listOfNotNull(title.trim().takeIf { it.isNotEmpty() }, languageLabel(language)).firstOrNull()

    private fun EmbeddedSubtitleJson.displayLabel(): String? =
        listOfNotNull(title.trim().takeIf { it.isNotEmpty() }, languageLabel(language)).firstOrNull()

    private fun Format.displayLabel(): String? =
        listOfNotNull(label?.trim()?.takeIf { it.isNotEmpty() }, languageLabel(language)).firstOrNull()

    private fun languageLabel(language: String?): String? {
        val trimmed = language?.trim().orEmpty()
        if (trimmed.isEmpty()) return null
        return trimmed.uppercase()
    }

    private fun Format.primaryAudioPickerLabel(fallbackIndex: Int): String {
        val lang = languageLabel(language)
        val lab = label?.trim()?.takeIf { it.isNotEmpty() }
        return when {
            lab != null && lang != null && !lab.contains(lang, ignoreCase = true) -> "$lab · $lang"
            lab != null -> lab
            lang != null -> lang
            else -> "Audio $fallbackIndex"
        }
    }

    private fun Format.primarySubtitlePickerLabel(): String {
        val lang = languageLabel(language)
        val lab = label?.trim()?.takeIf { it.isNotEmpty() }
        return when {
            lab != null && lang != null && !lab.contains(lang, ignoreCase = true) -> "$lab · $lang"
            lab != null -> lab
            lang != null -> lang
            else -> "Subtitle"
        }
    }

    private fun Format.audioPickerDetail(): String? {
        val parts = mutableListOf<String>()
        if (channelCount > 0) {
            parts +=
                when (channelCount) {
                    1 -> "Mono"
                    2 -> "Stereo"
                    6 -> "5.1 surround"
                    8 -> "7.1 surround"
                    else -> "${channelCount} channels"
                }
        }
        if (sampleRate > 0) {
            parts += "${sampleRate / 1000} kHz"
        }
        codecs?.trim()?.takeIf { it.isNotEmpty() }?.uppercase(Locale.US)?.let { parts += it }
        averageBitrate.takeIf { it > 0 }?.let { parts += "${it / 1000} kb/s" }
        return parts.takeIf { it.isNotEmpty() }?.joinToString(" · ")
    }

    private fun Format.subtitlePickerDetail(): String? {
        val parts = mutableListOf<String>()
        sampleMimeType?.trim()?.takeIf { it.isNotEmpty() }?.let { mime ->
            val short = mime.substringAfterLast('/').uppercase(Locale.US)
            if (short.isNotBlank()) parts += short
        }
        val lab = label?.trim().orEmpty()
        if (lab.isEmpty()) {
            languageLabel(language)?.let { parts += it }
        }
        return parts.takeIf { it.isNotEmpty() }?.joinToString(" · ")
    }

    private fun buildSubtitlePickerOptions(): List<TrackPickerOption> {
        val options = mutableListOf<TrackPickerOption>()
        val textDisabled =
            player.trackSelectionParameters.disabledTrackTypes.contains(C.TRACK_TYPE_TEXT)
        options +=
            TrackPickerOption(
                id = "off",
                label = "Off",
                selected = textDisabled,
                detail = "Hide subtitles",
            )
        val groups = player.currentTracks.groups
        for (gi in groups.indices) {
            val g = groups[gi]
            if (g.type != C.TRACK_TYPE_TEXT || g.length == 0) continue
            for (j in 0 until g.length) {
                val fmt = g.mediaTrackGroup.getFormat(j)
                val label = fmt.primarySubtitlePickerLabel()
                val id = "t:$gi:$j"
                val selected = !textDisabled && g.isTrackSelected(j)
                options +=
                    TrackPickerOption(
                        id = id,
                        label = label,
                        selected = selected,
                        detail = fmt.subtitlePickerDetail(),
                    )
            }
        }
        return options
    }

    /** Server-indexed tracks when in an HLS session with metadata; otherwise flattened Exo audio tracks. */
    private fun buildAudioPickerOptions(): List<TrackPickerOption>? {
        val sid = hlsSessionId
        val embeddedDistinct = embeddedAudioTracks.distinctBy { it.streamIndex }
        if (sid != null && embeddedDistinct.size >= 2) {
            val sorted = embeddedDistinct.sortedBy { it.streamIndex }
            val cur = serverAudioIndex
            return sorted.map { t ->
                val id = "s:${t.streamIndex}"
                val titlePart = t.title.trim()
                val langPart = languageLabel(t.language)
                val label =
                    when {
                        titlePart.isNotEmpty() && langPart != null -> "$titlePart · $langPart"
                        titlePart.isNotEmpty() -> titlePart
                        langPart != null -> langPart
                        else -> "Track ${t.streamIndex + 1}"
                    }
                val detail =
                    if (sorted.size > 1) {
                        "Embedded stream ${t.streamIndex + 1}"
                    } else {
                        null
                    }
                val selected =
                    t.streamIndex == cur ||
                        (cur < 0 && t.streamIndex == sorted.first().streamIndex)
                TrackPickerOption(id = id, label = label, selected = selected, detail = detail)
            }
        }
        val all = mutableListOf<TrackPickerOption>()
        val groups = player.currentTracks.groups
        for (gi in groups.indices) {
            val g = groups[gi]
            if (g.type != C.TRACK_TYPE_AUDIO || g.length == 0) continue
            for (j in 0 until g.length) {
                val fmt = g.mediaTrackGroup.getFormat(j)
                val idx = all.size + 1
                val label = fmt.primaryAudioPickerLabel(idx)
                val id = "a:$gi:$j"
                val selected = g.isTrackSelected(j)
                all +=
                    TrackPickerOption(
                        id = id,
                        label = label,
                        selected = selected,
                        detail = fmt.audioPickerDetail(),
                    )
            }
        }
        return all.takeIf { it.size >= 2 }
    }

    private fun disableTextTracks() {
        val b = player.trackSelectionParameters.buildUpon()
        b.clearOverridesOfType(C.TRACK_TYPE_TEXT)
        b.setTrackTypeDisabled(C.TRACK_TYPE_TEXT, true)
        player.trackSelectionParameters = b.build()
    }

    private fun applyTextTrackOverride(groupIndex: Int, trackIndex: Int) {
        val groups = player.currentTracks.groups
        if (groupIndex !in groups.indices) return
        val g = groups[groupIndex]
        if (g.type != C.TRACK_TYPE_TEXT || trackIndex !in 0 until g.length) return
        val mg = g.mediaTrackGroup
        val b = player.trackSelectionParameters.buildUpon()
        b.clearOverridesOfType(C.TRACK_TYPE_TEXT)
        b.setTrackTypeDisabled(C.TRACK_TYPE_TEXT, false)
        b.addOverride(TrackSelectionOverride(mg, listOf(trackIndex)))
        player.trackSelectionParameters = b.build()
    }

    private fun Format.isCea608ClosedCaptionTrack(): Boolean {
        val m = sampleMimeType ?: return false
        return m == MimeTypes.APPLICATION_CEA608 ||
            m == MimeTypes.APPLICATION_CEA708 ||
            m == MimeTypes.APPLICATION_MP4CEA608
    }

    private fun isCea608TextTrackSelected(groups: List<Tracks.Group>): Boolean {
        for (g in groups) {
            if (g.type != C.TRACK_TYPE_TEXT || g.length == 0) continue
            for (j in 0 until g.length) {
                if (g.isTrackSelected(j) && g.mediaTrackGroup.getFormat(j).isCea608ClosedCaptionTrack()) {
                    return true
                }
            }
        }
        return false
    }

    private fun findFirstNonCea608TextTrackIndex(groups: List<Tracks.Group>): Pair<Int, Int>? {
        for (gi in groups.indices) {
            val g = groups[gi]
            if (g.type != C.TRACK_TYPE_TEXT || g.length == 0) continue
            for (j in 0 until g.length) {
                val fmt = g.mediaTrackGroup.getFormat(j)
                if (!fmt.isCea608ClosedCaptionTrack()) {
                    return gi to j
                }
            }
        }
        return null
    }

    private fun preferNonCea608TextTrackOnceIfNeeded() {
        if (!preferNonCea608TextAfterLoad) return
        if (player.trackSelectionParameters.disabledTrackTypes.contains(C.TRACK_TYPE_TEXT)) {
            preferNonCea608TextAfterLoad = false
            return
        }
        val groups = player.currentTracks.groups
        val textTrackPresent = groups.any { it.type == C.TRACK_TYPE_TEXT && it.length > 0 }
        if (!textTrackPresent) {
            preferNonCea608TextAfterLoad = false
            return
        }
        if (!isCea608TextTrackSelected(groups)) {
            preferNonCea608TextAfterLoad = false
            return
        }
        val idx = findFirstNonCea608TextTrackIndex(groups) ?: run {
            preferNonCea608TextAfterLoad = false
            return
        }
        applyTextTrackOverride(idx.first, idx.second)
        preferNonCea608TextAfterLoad = false
    }

    private fun captureSubtitleRestoreState(): SubtitleRestoreState? {
        if (trackGroups(C.TRACK_TYPE_TEXT).isEmpty()) return null
        val params = player.trackSelectionParameters
        if (params.disabledTrackTypes.contains(C.TRACK_TYPE_TEXT)) {
            return SubtitleRestoreState(disabled = true, language = null, label = null, configurationId = null)
        }
        val fmt = selectedSubtitleFormat() ?: return null
        return SubtitleRestoreState(
            disabled = false,
            language = fmt.language,
            label = fmt.label,
            configurationId = fmt.id?.trim()?.takeIf { it.isNotEmpty() },
        )
    }

    private fun restorePendingSubtitlesIfNeeded() {
        val pending = pendingSubtitleRestore ?: return
        if (trackGroups(C.TRACK_TYPE_TEXT).isEmpty()) return
        pendingSubtitleRestore = null
        if (pending.disabled) {
            disableTextTracks()
            return
        }
        val wantId = pending.configurationId?.trim()?.takeIf { it.isNotEmpty() }
        val wantLang = pending.language?.trim()?.lowercase()?.takeIf { it.isNotEmpty() }
        val wantLabel = pending.label?.trim()?.lowercase()?.takeIf { it.isNotEmpty() }
        val groups = player.currentTracks.groups
        for (gi in groups.indices) {
            val g = groups[gi]
            if (g.type != C.TRACK_TYPE_TEXT || g.length == 0) continue
            for (j in 0 until g.length) {
                val fmt = g.mediaTrackGroup.getFormat(j)
                val sid = fmt.id?.trim()?.takeIf { it.isNotEmpty() }
                if (wantId != null && sid == wantId) {
                    applyTextTrackOverride(gi, j)
                    return
                }
            }
        }
        for (gi in groups.indices) {
            val g = groups[gi]
            if (g.type != C.TRACK_TYPE_TEXT || g.length == 0) continue
            for (j in 0 until g.length) {
                val fmt = g.mediaTrackGroup.getFormat(j)
                val lang = fmt.language?.trim()?.lowercase()?.takeIf { it.isNotEmpty() }
                val label = fmt.label?.trim()?.lowercase()?.takeIf { it.isNotEmpty() }
                val match =
                    when {
                        wantLang != null && lang == wantLang -> true
                        wantLang == null && wantLabel != null && label == wantLabel -> true
                        else -> false
                    }
                if (match) {
                    applyTextTrackOverride(gi, j)
                    return
                }
            }
        }
    }

    private fun applySubtitlePickerSelection(id: String) {
        if (id == "off") {
            disableTextTracks()
            return
        }
        if (!id.startsWith("t:")) return
        val parts = id.removePrefix("t:").split(":")
        if (parts.size != 2) return
        val gi = parts[0].toIntOrNull() ?: return
        val j = parts[1].toIntOrNull() ?: return
        applyTextTrackOverride(gi, j)
    }

    private fun applyExoAudioSelection(groupIndex: Int, trackIndex: Int) {
        val groups = player.currentTracks.groups
        if (groupIndex !in groups.indices) return
        val g = groups[groupIndex]
        if (g.type != C.TRACK_TYPE_AUDIO || trackIndex !in 0 until g.length) return
        val mg = g.mediaTrackGroup
        val b = player.trackSelectionParameters.buildUpon()
        b.clearOverridesOfType(C.TRACK_TYPE_AUDIO)
        b.setTrackTypeDisabled(C.TRACK_TYPE_AUDIO, false)
        b.addOverride(TrackSelectionOverride(mg, listOf(trackIndex)))
        player.trackSelectionParameters = b.build()
    }

    private fun cancelRevisionReadyPoll() {
        revisionReadyPollJob?.cancel()
        revisionReadyPollJob = null
    }

    private suspend fun applyServerAudioStream(streamIndex: Int) {
        val sid = hlsSessionId ?: return
        cancelRevisionReadyPoll()
        val result =
            withContext(Dispatchers.IO) {
                playbackRepository.updateSessionAudio(sid, streamIndex)
            }
        result.getOrNull()?.let { session ->
            mergeTrackMetadataFromSession(session)
            when (session.status) {
                "ready" -> {
                    swapToReadyStream(
                        streamUrl = session.streamUrl,
                        revision = session.revision,
                        durationSeconds = session.durationSeconds,
                    )
                    updateStatus("Playing")
                }
                "error" -> {
                    updateError(session.error ?: "Audio switch failed")
                    updateStatus("Error")
                }
                else -> {
                    // PATCH typically returns "starting": do not swap immediately — an empty/partial m3u8
                    // leaves ExoPlayer buffering for a long time. Keep the current revision playing
                    // until the playlist is readable or WebSocket reports ready.
                    val rev = session.revision
                    if (session.streamUrl.isBlank() || rev == null) {
                        updateStatus("Preparing…")
                        return@let
                    }
                    updateStatus("Switching audio…")
                    val relUrl = session.streamUrl
                    val targetRev = rev
                    val durationSec = session.durationSeconds
                    revisionReadyPollJob =
                        scope.launch {
                            val deadline = SystemClock.elapsedRealtime() + 3 * 60_000L
                            try {
                                val absUrl = playbackRepository.absoluteStreamUrl(relUrl)
                                while (isActive && SystemClock.elapsedRealtime() < deadline) {
                                    if (lastAppliedStreamRevision == targetRev) return@launch
                                    if (playbackRepository.hlsMasterPlaylistLooksReady(absUrl)) {
                                        swapToReadyStream(relUrl, targetRev, durationSec)
                                        updateStatus("Playing")
                                        return@launch
                                    }
                                    delay(250L)
                                }
                                if (isActive && lastAppliedStreamRevision != targetRev) {
                                    updateError("Audio track took too long to prepare")
                                    updateStatus("Playing")
                                }
                            } finally {
                                revisionReadyPollJob = null
                            }
                        }
                }
            }
        }
        result.onFailure { e ->
            updateError(e.message ?: "Audio switch failed")
        }
    }

    fun dismissTrackPicker() {
        _trackPicker.value = null
    }

    fun openSubtitlePicker() {
        scope.launch(Dispatchers.Main) {
            if (trackGroups(C.TRACK_TYPE_TEXT).isEmpty()) return@launch
            val options = buildSubtitlePickerOptions()
            if (options.size < 2) return@launch
            _trackPicker.value = TrackPicker.Subtitles(options = options)
        }
    }

    fun openAudioPicker() {
        scope.launch(Dispatchers.Main) {
            val options = buildAudioPickerOptions() ?: return@launch
            if (options.size < 2) return@launch
            _trackPicker.value = TrackPicker.Audio(options = options)
        }
    }

    fun selectTrackPickerOption(id: String) {
        val picker = _trackPicker.value ?: return
        _trackPicker.value = null
        when (picker) {
            is TrackPicker.Subtitles ->
                scope.launch(Dispatchers.Main) {
                    applySubtitlePickerSelection(id)
                    refreshUiState()
                }
            is TrackPicker.Audio ->
                scope.launch(Dispatchers.Main) {
                    when {
                        id.startsWith("s:") -> {
                            val stream = id.removePrefix("s:").toIntOrNull() ?: return@launch
                            applyServerAudioStream(stream)
                        }
                        id.startsWith("a:") -> {
                            val rest = id.removePrefix("a:").split(":")
                            if (rest.size != 2) return@launch
                            val gi = rest[0].toIntOrNull() ?: return@launch
                            val j = rest[1].toIntOrNull() ?: return@launch
                            applyExoAudioSelection(gi, j)
                        }
                    }
                    refreshUiState()
                }
        }
    }

    private suspend fun buildMediaItem(streamUrl: String): MediaItem {
        val subtitleConfigurations = mutableListOf<MediaItem.SubtitleConfiguration>()

        // HLS master already includes #EXT-X-MEDIA subtitle renditions (same WebVTT URLs). Sideloading
        // the same tracks again duplicates text groups and confuses the picker (Jellyfin-style: one path).
        if (hlsSessionId == null) {
            externalSubtitles.forEach { subtitle ->
                val subtitleUrl = playbackRepository.absoluteStreamUrl("/api/subtitles/${subtitle.id}")
                val builder =
                    MediaItem.SubtitleConfiguration.Builder(Uri.parse(subtitleUrl))
                        .setId("ext:${subtitle.id}")
                        // Server converts sidecars to WebVTT on the fly (see HandleStreamSubtitle).
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
                // Session payload omits bitmap/unsupported streams server-side; keep client guard for older servers.
                if (subtitle.supported == false) {
                    android.util.Log.d(
                        "PlumTV",
                        "Skipping embedded subtitle stream=${subtitle.streamIndex} (unsupported) codec=${subtitle.codec.orEmpty()}",
                    )
                    return@forEach
                }
                if (!subtitle.vttEligible) {
                    return@forEach
                }
                val subtitleUrl =
                    playbackRepository.absoluteStreamUrl("/api/media/$mediaId/subtitles/embedded/${subtitle.streamIndex}")
                val builder =
                    MediaItem.SubtitleConfiguration.Builder(Uri.parse(subtitleUrl))
                        .setId("emb:${subtitle.streamIndex}")
                        // Embedded extract endpoint always serves text/vtt (ffmpeg).
                        .setMimeType(MimeTypes.TEXT_VTT)
                if (subtitle.language.isNotBlank()) {
                    builder.setLanguage(subtitle.language)
                }
                if (subtitle.title.isNotBlank()) {
                    builder.setLabel(subtitle.title)
                }
                subtitleConfigurations += builder.build()
            }
        }

        // Blu-ray style: many language tracks are HDMV PGS (not WebVTT). HLS manifests only carry our
        // WebVTT renditions, so sideload raw PGS for every eligible stream (direct + transcode).
        embeddedSubtitleTracks.forEach { subtitle ->
            if (subtitle.supported == false) {
                return@forEach
            }
            if (!subtitle.pgsBinaryEligible) {
                return@forEach
            }
            val subtitleUrl =
                playbackRepository.absoluteStreamUrl("/api/media/$mediaId/subtitles/embedded/${subtitle.streamIndex}/sup")
            val builder =
                MediaItem.SubtitleConfiguration.Builder(Uri.parse(subtitleUrl))
                    .setId("emps:${subtitle.streamIndex}")
                    .setMimeType(MimeTypes.APPLICATION_PGS)
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
        applicationScope.launch(Dispatchers.IO) {
            runCatching { playbackRepository.warmEmbeddedSubtitleCaches(mediaId) }
        }
        val audioIndex = serverAudioIndex.takeIf { it >= 0 }
        playbackRepository.createSession(mediaId, audioIndex = audioIndex).fold(
            onSuccess = { session ->
                replaceTrackMetadataFromSession(session)
                lastAppliedStreamRevision = session.revision ?: -1
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
        pendingSubtitleRestore = null
        updateError(null)
        refreshUiState(statusOverride = "Preparing stream…", errorOverride = null)
        createAndLoadMedia(mediaId, resumeSec)
    }

    private fun clearUpNextCountdown() {
        upNextJob?.cancel()
        upNextJob = null
        _upNext.value = null
    }

    /** User dismissed the interstitial (e.g. Back): stay on the ended episode. */
    fun dismissUpNext() {
        clearUpNextCountdown()
    }

    /** User chose to skip the countdown (OK / center). */
    fun playUpNextNow() {
        clearUpNextCountdown()
        nextEpisode()
    }

    private fun scheduleUpNextCountdown() {
        if (queueIndex < 0 || queueIndex >= episodeQueue.lastIndex) return
        val next = episodeQueue.getOrNull(queueIndex + 1) ?: return
        clearUpNextCountdown()
        upNextJob =
            scope.launch {
                try {
                    repeat(UPNEXT_COUNTDOWN_SECONDS) { tick ->
                        val sec = UPNEXT_COUNTDOWN_SECONDS - tick
                        _upNext.value =
                            UpNextOverlayState(
                                secondsRemaining = sec,
                                title = next.title,
                                subtitle = next.subtitle,
                                backdropPath = next.backdropPath,
                                backdropUrl = next.backdropUrl,
                                showPosterPath = next.showPosterPath,
                                showPosterUrl = next.showPosterUrl,
                            )
                        delay(1_000)
                    }
                    // Use the episode captured at countdown start so we match the overlay. queueIndex /
                    // episodeQueue can change before this runs (e.g. [loadEpisodeQueue] completing).
                    // Do not cancel this coroutine from inside [switchToMedia]; countdown is done.
                    switchToMedia(next.mediaId, 0f, suppressUpNextJobCancel = true)
                } finally {
                    _upNext.value = null
                    upNextJob = null
                }
            }
    }

    private suspend fun switchToMedia(
        mediaId: Int,
        resumeSec: Float = 0f,
        suppressUpNextJobCancel: Boolean = false,
    ) {
        if (suppressUpNextJobCancel) {
            _upNext.value = null
        } else {
            clearUpNextCountdown()
        }
        if (this.mediaId == mediaId && hlsSessionId != null) return
        cancelRevisionReadyPoll()
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
        lastAppliedStreamRevision = -1
        introStartSec = null
        introEndSec = null
        introAutoSkipConsumed = false
        openMedia(mediaId, resumeSec)
    }

    /**
     * Swaps the player to a new ready stream when the URL changes **or** the session revision changes.
     * Relying on URL equality alone missed some audio-track switches (e.g. normalization quirks).
     */
    private suspend fun swapToReadyStream(
        streamUrl: String,
        revision: Int?,
        durationSeconds: Double,
    ) {
        val url = playbackRepository.absoluteStreamUrl(streamUrl)
        // When `revision` is absent (older servers), only the URL can disambiguate stream swaps.
        if (url == activeStreamUrl && (revision == null || revision == lastAppliedStreamRevision)) return
        if (revision != null) {
            lastAppliedStreamRevision = revision
        }
        activeStreamUrl = url
        lastDurationSec = durationSeconds
        val mediaItem = buildMediaItem(url)
        withContext(Dispatchers.Main) {
            pendingSubtitleRestore = captureSubtitleRestoreState()
            val hadMedia = player.mediaItemCount > 0
            val wasPlaying = player.isPlaying || player.playWhenReady
            val pos = player.currentPosition
            preferNonCea608TextAfterLoad = true
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

    private suspend fun handleSessionUpdate(ev: PlaybackSessionUpdateEventJson) {
        serverAudioIndex = ev.audioIndex
        mergeIntroFromSession(
            ev.introSkipMode,
            ev.introStartSeconds,
            ev.introEndSeconds,
        )
        when (ev.status) {
            "ready" -> {
                cancelRevisionReadyPoll()
                swapToReadyStream(
                    streamUrl = ev.streamUrl,
                    revision = ev.revision,
                    durationSeconds = ev.durationSeconds,
                )
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
            preferNonCea608TextAfterLoad = true
            player.setMediaItem(mediaItem)
            player.prepare()
            if (resumeSec > 0f) {
                player.seekTo((resumeSec * 1000).toLong())
            }
            player.playWhenReady = true
        }
        refreshUiState()
    }

    private suspend fun persistProgressFromValues(
        targetMediaId: Int,
        posMs: Long,
        durMs: Long,
        completed: Boolean,
    ) {
        val durSec = durMs / 1000.0
        if (durSec <= 0.0) return

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

    private suspend fun persistProgressAsync(
        completed: Boolean,
        mediaIdOverride: Int? = null,
    ) {
        val targetMediaId = mediaIdOverride ?: mediaId
        val (posMs, durMs) =
            withContext(Dispatchers.Main) { player.currentPosition to resolvedDurationMs() }
        persistProgressFromValues(targetMediaId, posMs, durMs, completed)
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

    /**
     * Called when the activity is no longer visible (Home, recents, another app). Stops audio/video
     * and tears down transient UI so returning to the app does not surprise the user.
     *
     * Releases the video surface so hardware decoders are not left reserved for Plum when the user
     * switches to live TV or another full-screen video app on the same device.
     */
    fun pauseWhenBackgrounded() {
        scope.launch(Dispatchers.Main) {
            if (controllerClosed.get()) return@launch
            clearUpNextCountdown()
            dismissTrackPicker()
            player.pause()
            player.playWhenReady = false
            runCatching { player.clearVideoSurface() }
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
        clearUpNextCountdown()
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
        clearUpNextCountdown()
        scope.launch {
            val next = if (queueIndex >= 0 && queueIndex < episodeQueue.lastIndex) episodeQueue.getOrNull(queueIndex + 1) else null
            if (next != null) {
                switchToMedia(next.mediaId, 0f)
            }
        }
    }

    /**
     * Reads position/duration/state then [androidx.media3.exoplayer.ExoPlayer.release]. Call only
     * on the main thread unless using the [runBlocking] fallback below.
     */
    private fun snapshotForCloseAndRelease(): Triple<Long, Long, Int> {
        val posMs = player.currentPosition
        val durMs = resolvedDurationMs()
        val state = player.playbackState
        player.release()
        return Triple(posMs, durMs, state)
    }

    /**
     * Cleanup that snapshots playback, releases ExoPlayer immediately (frees decoders/audio for
     * other apps), then persists progress + closes the server session in the background.
     */
    fun close() {
        if (!controllerClosed.compareAndSet(false, true)) return

        progressJob?.cancel()
        wsCollectJob?.cancel()
        queueLoadJob?.cancel()
        revisionReadyPollJob?.cancel()
        clearUpNextCountdown()
        dismissTrackPicker()

        hlsSessionId?.let { wsManager.sendDetach(it) }
        val sid = hlsSessionId
        val closingMediaId = mediaId

        val (posMs, durMs, state) =
            if (Looper.myLooper() == Looper.getMainLooper()) {
                snapshotForCloseAndRelease()
            } else {
                runBlocking(Dispatchers.Main) { snapshotForCloseAndRelease() }
            }

        applicationScope.launch(Dispatchers.Default) {
            try {
                withTimeoutOrNull(5_000) {
                    val durSec = durMs / 1000.0
                    if (durSec > 0) {
                        val ended = state == Player.STATE_ENDED
                        try {
                            persistProgressFromValues(
                                closingMediaId,
                                posMs,
                                durMs,
                                completed = ended,
                            )
                        } catch (_: Throwable) {
                        }
                        withContext(Dispatchers.IO) {
                            runCatching { sid?.let { playbackRepository.closeSession(it) } }
                        }
                    }
                }
            } catch (_: Throwable) {
            }
        }
    }
}
