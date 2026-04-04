package plum.tv.feature.home

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import plum.tv.core.data.BrowseRepository
import plum.tv.core.data.LibraryCatalogRefreshCoordinator
import plum.tv.core.network.ContinueWatchingEntryJson
import plum.tv.core.network.HomeDashboardJson
import plum.tv.core.network.RecentlyAddedEntryJson

sealed interface HomeUiState {
    data object Loading : HomeUiState
    data class Ready(
        val continueWatching: List<ContinueWatchingEntryJson>,
        val recentlyAdded: List<RecentlyAddedEntryJson>,
    ) : HomeUiState
    data class Error(val message: String) : HomeUiState
}

@HiltViewModel
class HomeViewModel @Inject constructor(
    private val browseRepository: BrowseRepository,
    catalogRefreshCoordinator: LibraryCatalogRefreshCoordinator,
) : ViewModel() {

    private val _state = MutableStateFlow<HomeUiState>(HomeUiState.Loading)
    val state: StateFlow<HomeUiState> = _state.asStateFlow()

    init {
        viewModelScope.launch {
            catalogRefreshCoordinator.catalogRefreshEvents.collect {
                refresh(showLoading = false)
            }
        }
    }

    /**
     * Called when the Home destination is composed (including after returning from Library/Details).
     * The ViewModel is scoped to the Home back stack entry, so without this the dashboard JSON and
     * Coil URLs stay frozen at the first fetch while browse screens load fresher metadata.
     */
    fun onAppear() {
        refresh(showLoading = _state.value !is HomeUiState.Ready)
    }

    fun refresh(showLoading: Boolean = true) {
        viewModelScope.launch {
            val hadReadyDashboard = _state.value is HomeUiState.Ready
            if (showLoading || !hadReadyDashboard) {
                _state.value = HomeUiState.Loading
            }
            browseRepository.homeDashboard().fold(
                onSuccess = { dash: HomeDashboardJson ->
                    val mergedRecentlyAdded =
                        buildList {
                            addAll(dash.recentlyAddedTvEpisodes)
                            addAll(dash.recentlyAddedTvShows)
                            addAll(dash.recentlyAddedMovies)
                            addAll(dash.recentlyAddedAnimeEpisodes)
                            addAll(dash.recentlyAddedAnimeShows)
                        }
                    _state.value = HomeUiState.Ready(
                        continueWatching = dash.continueWatching,
                        recentlyAdded = mergedRecentlyAdded,
                    )
                },
                onFailure = { e ->
                    if (hadReadyDashboard && !showLoading) {
                        return@fold
                    }
                    _state.value = HomeUiState.Error(e.message ?: "Failed to load home")
                },
            )
        }
    }
}
