package plum.tv.feature.library

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import plum.tv.core.data.BrowseRepository
import plum.tv.core.network.LibraryJson

sealed interface LibraryListUiState {
    data object Loading : LibraryListUiState
    data class Ready(val libraries: List<LibraryJson>) : LibraryListUiState
    data class Error(val message: String) : LibraryListUiState
}

@HiltViewModel
class LibraryListViewModel @Inject constructor(
    private val browseRepository: BrowseRepository,
) : ViewModel() {

    private val _state = MutableStateFlow<LibraryListUiState>(LibraryListUiState.Loading)
    val state: StateFlow<LibraryListUiState> = _state.asStateFlow()

    init {
        loadInitial()
    }

    /** Uses cached libraries when available (e.g. after rail shortcut). */
    private fun loadInitial() {
        viewModelScope.launch {
            _state.value = LibraryListUiState.Loading
            browseRepository.libraries(forceRefresh = false).fold(
                onSuccess = { libs -> _state.value = LibraryListUiState.Ready(libs) },
                onFailure = { e -> _state.value = LibraryListUiState.Error(e.message ?: "Failed to load libraries") },
            )
        }
    }

    fun refresh() {
        viewModelScope.launch {
            _state.value = LibraryListUiState.Loading
            browseRepository.libraries(forceRefresh = true).fold(
                onSuccess = { libs -> _state.value = LibraryListUiState.Ready(libs) },
                onFailure = { e -> _state.value = LibraryListUiState.Error(e.message ?: "Failed to load libraries") },
            )
        }
    }
}
