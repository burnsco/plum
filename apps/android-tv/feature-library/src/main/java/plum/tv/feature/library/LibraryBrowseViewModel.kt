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
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import plum.tv.core.data.BrowseRepository
import plum.tv.core.data.LibraryCatalogRefreshCoordinator
import plum.tv.core.network.LibraryBrowseItemJson
import plum.tv.core.network.groupLibraryBrowseItemsByShow
import plum.tv.core.network.isShowOnlyBrowseLibrary

sealed interface LibraryBrowseUiState {
    data object Loading : LibraryBrowseUiState
    data class Ready(
        val rows: List<LibraryBrowseGridRow>,
        val hasMore: Boolean,
        val loadingMore: Boolean,
    ) : LibraryBrowseUiState
    data class Error(val message: String) : LibraryBrowseUiState
}

@HiltViewModel
class LibraryBrowseViewModel @Inject constructor(
    private val browseRepository: BrowseRepository,
    catalogRefreshCoordinator: LibraryCatalogRefreshCoordinator,
    savedStateHandle: SavedStateHandle,
) : ViewModel() {

    private val libraryId: Int = savedStateHandle.get<Int>("libraryId")
        ?: error("libraryId required")

    private val _state = MutableStateFlow<LibraryBrowseUiState>(LibraryBrowseUiState.Loading)
    val state: StateFlow<LibraryBrowseUiState> = _state.asStateFlow()

    private var nextOffset: Int? = null
    private val pageSize = 60
    private val accumulatedItems = mutableListOf<LibraryBrowseItemJson>()
    private val listingMutex = Mutex()

    // Cached after first page load; null means not yet determined.
    private var isShowLibrary: Boolean? = null

    init {
        // Synchronous peek avoids a one-frame "Loading" flash when prefetch or a prior visit warmed cache.
        browseRepository.peekLibraryMediaPage(libraryId, offset = 0, limit = pageSize)?.let { page ->
            nextOffset = page.nextOffset
            accumulatedItems.addAll(page.items)
            _state.value =
                LibraryBrowseUiState.Ready(
                    rows = rebuildRows(),
                    hasMore = page.hasMore,
                    loadingMore = false,
                )
        }
        viewModelScope.launch {
            listingMutex.withLock {
                refreshInitialFromNetwork(forceRefresh = false)
            }
        }
        viewModelScope.launch {
            catalogRefreshCoordinator.catalogRefreshEvents.collect { ev ->
                if (ev.libraryId != libraryId) return@collect
                listingMutex.withLock {
                    _state.value = LibraryBrowseUiState.Loading
                    accumulatedItems.clear()
                    refreshInitialFromNetwork(forceRefresh = true)
                }
            }
        }
    }

    private fun rebuildRows(): List<LibraryBrowseGridRow> {
        val items = accumulatedItems.toList()
        if (items.isEmpty()) return emptyList()
        val showLib = isShowOnlyBrowseLibrary(items)
        isShowLibrary = showLib
        return if (showLib) {
            groupLibraryBrowseItemsByShow(items).map { LibraryBrowseGridRow.Show(it) }
        } else {
            items.map { LibraryBrowseGridRow.Movie(it) }
        }
    }

    /**
     * For movie libraries, avoids re-wrapping all accumulated items by appending only the new rows.
     * Show libraries still require a full rebuild because new episodes can merge into existing show rows.
     */
    private fun appendRows(
        newItems: List<LibraryBrowseItemJson>,
        existingRows: List<LibraryBrowseGridRow>,
    ): List<LibraryBrowseGridRow> = when (isShowLibrary) {
        false -> existingRows + newItems.map { LibraryBrowseGridRow.Movie(it) }
        else -> rebuildRows()
    }

    fun loadInitial(forceNetwork: Boolean = false) {
        viewModelScope.launch {
            listingMutex.withLock {
                if (forceNetwork) {
                    _state.value = LibraryBrowseUiState.Loading
                    accumulatedItems.clear()
                }
                refreshInitialFromNetwork(forceRefresh = forceNetwork)
            }
        }
    }

    private suspend fun refreshInitialFromNetwork(forceRefresh: Boolean) {
        browseRepository.libraryMedia(libraryId, offset = 0, limit = pageSize, forceRefresh = forceRefresh).fold(
            onSuccess = { page ->
                // loadMore() may finish before this initial refresh; do not wipe extra pages.
                if (accumulatedItems.size > page.items.size) {
                    return@fold
                }
                nextOffset = page.nextOffset
                accumulatedItems.clear()
                accumulatedItems.addAll(page.items)
                _state.value =
                    LibraryBrowseUiState.Ready(
                        rows = rebuildRows(),
                        hasMore = page.hasMore,
                        loadingMore = false,
                    )
            },
            onFailure = { e ->
                if (_state.value !is LibraryBrowseUiState.Ready) {
                    _state.value = LibraryBrowseUiState.Error(e.message ?: "Failed to load library")
                }
            },
        )
    }

    fun loadMore() {
        val cur = _state.value
        if (cur !is LibraryBrowseUiState.Ready || !cur.hasMore || cur.loadingMore) return
        val offset = nextOffset ?: return
        viewModelScope.launch {
            listingMutex.withLock {
                val latest = _state.value
                if (latest !is LibraryBrowseUiState.Ready || !latest.hasMore || latest.loadingMore) return@withLock
                _state.value = latest.copy(loadingMore = true)
                browseRepository.libraryMedia(libraryId, offset = offset, limit = pageSize, forceRefresh = false).fold(
                    onSuccess = { page ->
                        nextOffset = page.nextOffset
                        accumulatedItems.addAll(page.items)
                        _state.value =
                            LibraryBrowseUiState.Ready(
                                rows = appendRows(page.items, latest.rows),
                                hasMore = page.hasMore,
                                loadingMore = false,
                            )
                    },
                    onFailure = {
                        _state.value = latest.copy(loadingMore = false)
                    },
                )
            }
        }
    }
}
