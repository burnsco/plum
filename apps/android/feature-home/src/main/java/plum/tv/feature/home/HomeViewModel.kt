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
import plum.tv.core.data.HomeDashboardDiskCache
import plum.tv.core.data.LibraryCatalogRefreshCoordinator
import plum.tv.core.data.SessionPreferences
import plum.tv.core.network.ContinueWatchingEntryJson
import plum.tv.core.network.HomeDashboardJson
import plum.tv.core.network.MediaItemJson
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
    private val homeDashboardDiskCache: HomeDashboardDiskCache,
    private val sessionPreferences: SessionPreferences,
    catalogRefreshCoordinator: LibraryCatalogRefreshCoordinator,
) : ViewModel() {

    private val _state = MutableStateFlow<HomeUiState>(HomeUiState.Loading)
    val state: StateFlow<HomeUiState> = _state.asStateFlow()

    init {
        viewModelScope.launch {
            catalogRefreshCoordinator.catalogRefreshEvents.collect {
                refreshInternal(showLoading = false)
            }
        }
    }

    /**
     * Called when the Home destination is composed (including after returning from Library/Details).
     * The ViewModel is scoped to the Home back stack entry, so without this the dashboard JSON and
     * Coil URLs stay frozen at the first fetch while browse screens load fresher metadata.
     */
    fun onAppear() {
        viewModelScope.launch {
            applyCachedHomeIfAvailable()
            refreshInternal(showLoading = _state.value !is HomeUiState.Ready)
        }
    }

    fun refresh(showLoading: Boolean = true) {
        viewModelScope.launch { refreshInternal(showLoading) }
    }

    private suspend fun applyCachedHomeIfAvailable() {
        if (_state.value is HomeUiState.Ready) return
        val url = sessionPreferences.serverUrl.value?.trim()?.trimEnd('/') ?: return
        homeDashboardDiskCache.read(url)?.let { dash ->
            _state.value = dash.toHomeUiReady()
        }
    }

    private suspend fun refreshInternal(showLoading: Boolean) {
        val hadReadyDashboard = _state.value is HomeUiState.Ready
        if (showLoading || !hadReadyDashboard) {
            _state.value = HomeUiState.Loading
        }
        browseRepository.homeDashboard().fold(
            onSuccess = { dash: HomeDashboardJson ->
                val normalizedUrl = sessionPreferences.serverUrl.value?.trim()?.trimEnd('/')
                if (!normalizedUrl.isNullOrBlank()) {
                    homeDashboardDiskCache.write(normalizedUrl, dash)
                }
                _state.value = dash.toHomeUiReady()
            },
            onFailure = { e ->
                if (hadReadyDashboard && !showLoading) {
                    return@fold
                }
                _state.value = HomeUiState.Error(e.message ?: "Failed to load home")
            },
        )
    }

    private fun HomeDashboardJson.toHomeUiReady(): HomeUiState.Ready {
        val mergedRecentlyAdded =
            buildList {
                addAll(recentlyAddedTvEpisodes)
                addAll(recentlyAddedTvShows)
                addAll(recentlyAddedMovies)
                addAll(recentlyAddedAnimeEpisodes)
                addAll(recentlyAddedAnimeShows)
            }
        return HomeUiState.Ready(
            continueWatching = continueWatching.filterNot { it.media.isMissingFromLibrary() },
            recentlyAdded = mergedRecentlyAdded,
        )
    }
}

/** Matches server/dashboard rules: soft-removed files must not appear in Continue watching. */
private fun MediaItemJson.isMissingFromLibrary(): Boolean =
    missing == true || !missingSince.isNullOrBlank()
