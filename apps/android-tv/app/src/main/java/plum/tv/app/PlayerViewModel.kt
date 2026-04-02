package plum.tv.app

import android.content.Context
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.media3.datasource.DataSource
import androidx.media3.exoplayer.ExoPlayer
import dagger.hilt.android.lifecycle.HiltViewModel
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import kotlinx.coroutines.flow.StateFlow
import plum.tv.core.data.PlaybackRepository
import plum.tv.core.data.PlumWebSocketManager
import plum.tv.core.player.PlumPlayerController

@HiltViewModel
class PlayerViewModel @Inject constructor(
    @ApplicationContext private val appContext: Context,
    private val dataSourceFactory: DataSource.Factory,
    private val playbackRepository: PlaybackRepository,
    private val wsManager: PlumWebSocketManager,
    savedStateHandle: SavedStateHandle,
) : ViewModel() {

    private val mediaId: Int = savedStateHandle.get<Int>("mediaId")!!
    private val resumeSec: Float = savedStateHandle.get<Float>("resume") ?: 0f

    private val controller =
        PlumPlayerController(
            appContext = appContext,
            dataSourceFactory = dataSourceFactory,
            playbackRepository = playbackRepository,
            wsManager = wsManager,
            scope = viewModelScope,
            mediaId = mediaId,
            resumeSec = resumeSec,
        )

    val player: ExoPlayer = controller.player
    val error: StateFlow<String?> = controller.error
    val status: StateFlow<String> = controller.status

    fun cycleAudioTrack() {
        controller.cycleAudioTrack()
    }

    fun cycleSubtitles() {
        controller.cycleSubtitles()
    }

    override fun onCleared() {
        controller.close()
        super.onCleared()
    }
}
