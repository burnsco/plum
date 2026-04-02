package plum.tv.feature.library

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
import plum.tv.core.network.LibraryBrowseItemJson

sealed interface LibraryBrowseUiState {
    data object Loading : LibraryBrowseUiState
    data class Ready(
        val items: List<LibraryBrowseItemJson>,
        val hasMore: Boolean,
        val loadingMore: Boolean,
    ) : LibraryBrowseUiState
    data class Error(val message: String) : LibraryBrowseUiState
}

@HiltViewModel
class LibraryBrowseViewModel @Inject constructor(
    private val browseRepository: BrowseRepository,
    savedStateHandle: SavedStateHandle,
) : ViewModel() {

    private val libraryId: Int = savedStateHandle.get<Int>("libraryId")
        ?: error("libraryId required")

    private val _state = MutableStateFlow<LibraryBrowseUiState>(LibraryBrowseUiState.Loading)
    val state: StateFlow<LibraryBrowseUiState> = _state.asStateFlow()

    private var nextOffset: Int? = null
    private val pageSize = 60

    init {
        loadInitial()
    }

    fun loadInitial() {
        viewModelScope.launch {
            _state.value = LibraryBrowseUiState.Loading
            browseRepository.libraryMedia(libraryId, offset = 0, limit = pageSize).fold(
                onSuccess = { page ->
                    nextOffset = page.nextOffset
                    _state.value = LibraryBrowseUiState.Ready(
                        items = page.items,
                        hasMore = page.hasMore,
                        loadingMore = false,
                    )
                },
                onFailure = { e ->
                    _state.value = LibraryBrowseUiState.Error(e.message ?: "Failed to load library")
                },
            )
        }
    }

    fun loadMore() {
        val cur = _state.value
        if (cur !is LibraryBrowseUiState.Ready || !cur.hasMore || cur.loadingMore) return
        val offset = nextOffset ?: return
        viewModelScope.launch {
            _state.value = cur.copy(loadingMore = true)
            browseRepository.libraryMedia(libraryId, offset = offset, limit = pageSize).fold(
                onSuccess = { page ->
                    nextOffset = page.nextOffset
                    _state.value = LibraryBrowseUiState.Ready(
                        items = cur.items + page.items,
                        hasMore = page.hasMore,
                        loadingMore = false,
                    )
                },
                onFailure = { e ->
                    _state.value = cur.copy(loadingMore = false)
                },
            )
        }
    }
}
