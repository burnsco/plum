package plum.tv.feature.details

import android.net.Uri
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import plum.tv.core.data.BrowseRepository
import plum.tv.core.network.LibraryShowDetailsJson
import plum.tv.core.network.ShowSeasonEpisodesJson

sealed interface ShowDetailUiState {
    data object Loading : ShowDetailUiState
    data class Ready(
        val details: LibraryShowDetailsJson,
        val seasons: List<ShowSeasonEpisodesJson>,
        val selectedSeasonIndex: Int,
    ) : ShowDetailUiState
    data class Error(val message: String) : ShowDetailUiState
}

@HiltViewModel
class ShowDetailViewModel @Inject constructor(
    private val browseRepository: BrowseRepository,
    savedStateHandle: SavedStateHandle,
) : ViewModel() {

    private val libraryId: Int = savedStateHandle.get<Int>("libraryId")!!
    private val showKeyEncoded: String = savedStateHandle.get<String>("showKey")!!
    private val showKey: String = Uri.decode(showKeyEncoded)

    private val _state = MutableStateFlow<ShowDetailUiState>(ShowDetailUiState.Loading)
    val state: StateFlow<ShowDetailUiState> = _state.asStateFlow()

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            _state.value = ShowDetailUiState.Loading
            coroutineScope {
                val d = async { browseRepository.showDetails(libraryId, showKey) }
                val e = async { browseRepository.showEpisodes(libraryId, showKey) }
                val det = d.await()
                val eps = e.await()
                if (det.isFailure) {
                    _state.value = ShowDetailUiState.Error(det.exceptionOrNull()?.message ?: "Failed to load show")
                    return@coroutineScope
                }
                if (eps.isFailure) {
                    _state.value = ShowDetailUiState.Error(eps.exceptionOrNull()?.message ?: "Failed to load episodes")
                    return@coroutineScope
                }
                val seasons = eps.getOrNull()?.seasons.orEmpty()
                _state.value = ShowDetailUiState.Ready(
                    details = det.getOrNull()!!,
                    seasons = seasons,
                    selectedSeasonIndex = 0,
                )
            }
        }
    }

    fun selectSeason(index: Int) {
        val cur = _state.value
        if (cur !is ShowDetailUiState.Ready) return
        if (index in cur.seasons.indices) {
            _state.value = cur.copy(selectedSeasonIndex = index)
        }
    }
}
