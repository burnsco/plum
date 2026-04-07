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
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch
import plum.tv.core.data.BrowseRepository
import plum.tv.core.data.LibraryCatalogRefreshCoordinator
import plum.tv.core.network.LibraryShowDetailsJson
import plum.tv.core.network.ShowSeasonEpisodesJson

sealed interface ShowDetailUiState {
    data object Loading : ShowDetailUiState
    data class Ready(
        val details: LibraryShowDetailsJson,
        val seasons: List<ShowSeasonEpisodesJson>,
        val selectedSeasonIndex: Int,
        /** Season tab chosen on first open (resume / next episode). */
        val resumeSeasonIndex: Int,
        /** Episode row to focus within [resumeSeasonIndex] on first open; when user switches season, UI uses 0. */
        val resumeEpisodeIndex: Int,
    ) : ShowDetailUiState
    data class Error(val message: String) : ShowDetailUiState
}

@HiltViewModel
class ShowDetailViewModel @Inject constructor(
    private val browseRepository: BrowseRepository,
    catalogRefreshCoordinator: LibraryCatalogRefreshCoordinator,
    private val savedStateHandle: SavedStateHandle,
) : ViewModel() {

    companion object {
        const val RETURN_FOCUS_EPISODE_MEDIA_ID = "plum_return_focus_episode_media_id"
    }

    /** Set on the nav back stack when closing the player from this show; `<= 0` means none. */
    val returnFocusEpisodeMediaId: StateFlow<Int> =
        savedStateHandle.getStateFlow(RETURN_FOCUS_EPISODE_MEDIA_ID, -1)

    private val libraryId: Int = savedStateHandle.get<Int>("libraryId")!!
    private val showKeyEncoded: String = savedStateHandle.get<String>("showKey")!!
    private val showKey: String = Uri.decode(showKeyEncoded)

    private val _state = MutableStateFlow<ShowDetailUiState>(ShowDetailUiState.Loading)
    val state: StateFlow<ShowDetailUiState> = _state.asStateFlow()
    private var loadJob: Job? = null

    init {
        load()
        viewModelScope.launch {
            catalogRefreshCoordinator.catalogRefreshEvents.collect { ev ->
                if (ev.libraryId == libraryId) load()
            }
        }
    }

    fun load() {
        loadJob?.cancel()
        loadJob =
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
                    val (resumeSeason, resumeEp) = computeResumeSeasonAndEpisode(seasons)
                    _state.value =
                        ShowDetailUiState.Ready(
                            details = det.getOrNull()!!,
                            seasons = seasons,
                            selectedSeasonIndex = resumeSeason,
                            resumeSeasonIndex = resumeSeason,
                            resumeEpisodeIndex = resumeEp,
                        )
                }
            }
    }

    fun selectSeason(index: Int) {
        val cur = _state.value
        if (cur !is ShowDetailUiState.Ready) return
        if (index in cur.seasons.indices) {
            _state.value =
                cur.copy(
                    selectedSeasonIndex = index,
                )
        }
    }

    /** Selects the season row that contains [mediaId] (e.g. after prev/next episode in the player). */
    fun ensureSeasonSelectedForMediaId(mediaId: Int) {
        val cur = _state.value
        if (cur !is ShowDetailUiState.Ready) return
        val idx = cur.seasons.indexOfFirst { se -> se.episodes.any { it.id == mediaId } }
        if (idx >= 0 && idx != cur.selectedSeasonIndex) {
            _state.value = cur.copy(selectedSeasonIndex = idx)
        }
    }

    fun clearReturnFocusEpisodeRequest() {
        savedStateHandle[RETURN_FOCUS_EPISODE_MEDIA_ID] = -1
    }
}
