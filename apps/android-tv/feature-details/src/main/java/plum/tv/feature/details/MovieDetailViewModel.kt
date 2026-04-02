package plum.tv.feature.details

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import plum.tv.core.data.BrowseRepository
import plum.tv.core.network.LibraryMovieDetailsJson

sealed interface MovieDetailUiState {
    data object Loading : MovieDetailUiState
    data class Ready(val details: LibraryMovieDetailsJson) : MovieDetailUiState
    data class Error(val message: String) : MovieDetailUiState
}

@HiltViewModel
class MovieDetailViewModel @Inject constructor(
    private val browseRepository: BrowseRepository,
    savedStateHandle: SavedStateHandle,
) : ViewModel() {

    private val libraryId: Int = savedStateHandle.get<Int>("libraryId")!!
    private val mediaId: Int = savedStateHandle.get<Int>("mediaId")!!

    private val _state = MutableStateFlow<MovieDetailUiState>(MovieDetailUiState.Loading)
    val state: StateFlow<MovieDetailUiState> = _state.asStateFlow()

    init {
        load()
    }

    fun load() {
        viewModelScope.launch {
            _state.value = MovieDetailUiState.Loading
            browseRepository.movieDetails(libraryId, mediaId).fold(
                onSuccess = { _state.value = MovieDetailUiState.Ready(it) },
                onFailure = { e -> _state.value = MovieDetailUiState.Error(e.message ?: "Failed to load") },
            )
        }
    }
}
