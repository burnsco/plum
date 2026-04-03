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
) : ViewModel() {

    private val _state = MutableStateFlow<HomeUiState>(HomeUiState.Loading)
    val state: StateFlow<HomeUiState> = _state.asStateFlow()

    init {
        refresh()
    }

    fun refresh() {
        viewModelScope.launch {
            _state.value = HomeUiState.Loading
            browseRepository.homeDashboard().fold(
                onSuccess = { dash: HomeDashboardJson ->
                    _state.value = HomeUiState.Ready(
                        continueWatching = dash.continueWatching,
                        recentlyAdded = dash.recentlyAdded,
                    )
                    viewModelScope.launch {
                        browseRepository.prefetchFirstLibraryMediaPages()
                    }
                },
                onFailure = { e ->
                    _state.value = HomeUiState.Error(e.message ?: "Failed to load home")
                },
            )
        }
    }
}
