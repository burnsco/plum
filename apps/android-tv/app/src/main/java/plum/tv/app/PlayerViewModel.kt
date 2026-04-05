package plum.tv.app

import android.content.Context
import android.net.Uri
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.media3.datasource.DataSource
import androidx.media3.exoplayer.ExoPlayer
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import plum.tv.core.data.BrowseRepository
import plum.tv.core.data.PlayerSubtitlePreferences
import plum.tv.core.data.SubtitleAppearance
import plum.tv.core.data.PlaybackRepository
import plum.tv.core.data.PlumWebSocketManager
import plum.tv.core.data.di.ApplicationScope
import plum.tv.core.player.PlayerUiState
import plum.tv.core.player.PlumPlayerController
import plum.tv.core.player.TrackPicker
import plum.tv.core.player.UpNextOverlayState

@HiltViewModel
class PlayerViewModel @Inject constructor(
    @param:ApplicationContext private val appContext: Context,
    @param:ApplicationScope private val applicationScope: CoroutineScope,
    private val dataSourceFactory: DataSource.Factory,
    private val browseRepository: BrowseRepository,
    private val playbackRepository: PlaybackRepository,
    private val wsManager: PlumWebSocketManager,
    private val playerSubtitlePreferences: PlayerSubtitlePreferences,
    savedStateHandle: SavedStateHandle,
) : ViewModel() {

    private val mediaId: Int = savedStateHandle.get<Int>("mediaId")!!
    private val resumeSec: Float = savedStateHandle.get<Float>("resume") ?: 0f
    private val libraryId: Int? = savedStateHandle.get<Int>("libraryId")?.takeIf { it > 0 }
    private val showKey: String? =
        savedStateHandle.get<String>("showKey")
            ?.takeIf { it.isNotBlank() }
            ?.let(Uri::decode)

    private val displayTitle: String? =
        savedStateHandle.get<String>("displayTitle")
            ?.trim()
            ?.takeIf { it.isNotEmpty() }
            ?.let(Uri::decode)

    private val displaySubtitle: String? =
        savedStateHandle.get<String>("displaySubtitle")
            ?.trim()
            ?.takeIf { it.isNotEmpty() }
            ?.let(Uri::decode)

    private val controller =
        PlumPlayerController(
            appContext = appContext,
            dataSourceFactory = dataSourceFactory,
            browseRepository = browseRepository,
            playbackRepository = playbackRepository,
            wsManager = wsManager,
            scope = viewModelScope,
            applicationScope = applicationScope,
            mediaId = mediaId,
            resumeSec = resumeSec,
            libraryId = libraryId,
            showKey = showKey,
            navDisplayTitle = displayTitle,
            navDisplaySubtitle = displaySubtitle,
        )

    val player: ExoPlayer = controller.player
    val uiState: StateFlow<PlayerUiState> = controller.uiState
    val wallClock: StateFlow<Long> = controller.wallClock
    val trackPicker: StateFlow<TrackPicker?> = controller.trackPicker
    val upNext: StateFlow<UpNextOverlayState?> = controller.upNext
    val error: StateFlow<String?> = controller.error
    val status: StateFlow<String> = controller.status

    val subtitleAppearance: StateFlow<SubtitleAppearance> = playerSubtitlePreferences.appearance

    private val _subtitleStyleOverlayVisible = MutableStateFlow(false)
    val subtitleStyleOverlayVisible: StateFlow<Boolean> = _subtitleStyleOverlayVisible.asStateFlow()

    fun togglePlayPause() {
        controller.togglePlayPause()
    }

    fun pauseWhenBackgrounded() {
        controller.pauseWhenBackgrounded()
    }

    fun rewind10() {
        controller.rewind10()
    }

    fun forward10() {
        controller.forward10()
    }

    fun skipIntro() {
        controller.skipIntro()
    }

    fun seekTimelineBySteps(steps: Int) {
        val durationMs = uiState.value.durationMs
        val step =
            if (durationMs <= 0) {
                10_000L
            } else {
                (durationMs / 100).coerceIn(5_000L, 60_000L)
            }
        controller.seekBy(step * steps)
    }

    fun previousEpisode() {
        controller.previousEpisode()
    }

    fun nextEpisode() {
        controller.nextEpisode()
    }

    fun dismissUpNext() {
        controller.dismissUpNext()
    }

    fun playUpNextNow() {
        controller.playUpNextNow()
    }

    fun openAudioPicker() {
        controller.openAudioPicker()
    }

    fun openSubtitlePicker() {
        controller.openSubtitlePicker()
    }

    fun dismissTrackPicker() {
        controller.dismissTrackPicker()
    }

    fun selectTrackPickerOption(id: String) {
        controller.selectTrackPickerOption(id)
    }

    fun openSubtitleStyleSettings() {
        _subtitleStyleOverlayVisible.value = true
    }

    fun dismissSubtitleStyleSettings() {
        _subtitleStyleOverlayVisible.value = false
    }

    fun setSubtitleAppearance(value: SubtitleAppearance) {
        viewModelScope.launch {
            playerSubtitlePreferences.setAppearance(value)
        }
    }

    override fun onCleared() {
        controller.close()
        super.onCleared()
    }
}
