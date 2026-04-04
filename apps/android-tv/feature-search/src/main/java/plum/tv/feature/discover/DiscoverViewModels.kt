package plum.tv.feature.discover

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import plum.tv.core.data.DiscoverRepository
import plum.tv.core.network.DiscoverBrowseResponseJson
import plum.tv.core.network.DiscoverGenresResponseJson
import plum.tv.core.network.DiscoverResponseJson
import plum.tv.core.network.DiscoverTitleDetailsJson
import plum.tv.core.network.DownloadItemJson

sealed interface DiscoverUiState {
    data object Loading : DiscoverUiState
    data class Ready(
        val discover: DiscoverResponseJson,
        val genres: DiscoverGenresResponseJson,
    ) : DiscoverUiState
    data class Error(val message: String) : DiscoverUiState
}

sealed interface DiscoverBrowseUiState {
    data object Loading : DiscoverBrowseUiState
    data class Ready(
        val title: String,
        val category: String?,
        val mediaType: String?,
        val genre: plum.tv.core.network.DiscoverGenreJson?,
        val genres: List<plum.tv.core.network.DiscoverGenreJson>,
        val items: List<plum.tv.core.network.DiscoverItemJson>,
        val totalResults: Int,
        val currentPage: Int = 1,
        val totalPages: Int = 1,
        val hasMore: Boolean = false,
        /** True while a filter change is loading new results (grid stays visible). */
        val refreshing: Boolean = false,
        /** True while appending the next page (bottom loading indicator). */
        val loadingMore: Boolean = false,
    ) : DiscoverBrowseUiState
    data class Error(val message: String) : DiscoverBrowseUiState
}

sealed interface DiscoverDetailUiState {
    data object Loading : DiscoverDetailUiState
    data class Ready(val details: DiscoverTitleDetailsJson) : DiscoverDetailUiState
    data class Error(val message: String) : DiscoverDetailUiState
}

sealed interface DownloadsUiState {
    data object Loading : DownloadsUiState
    data class Ready(
        val configured: Boolean,
        val items: List<DownloadItemJson>,
    ) : DownloadsUiState
    data class Error(val message: String) : DownloadsUiState
}

@HiltViewModel
class DiscoverViewModel @Inject constructor(
    private val repository: DiscoverRepository,
) : ViewModel() {
    private val _state = MutableStateFlow<DiscoverUiState>(DiscoverUiState.Loading)
    val state: StateFlow<DiscoverUiState> = _state.asStateFlow()

    init {
        refresh()
    }

    fun refresh() {
        viewModelScope.launch {
            _state.value = DiscoverUiState.Loading
            val discover = repository.discover().getOrElse {
                _state.value = DiscoverUiState.Error(it.message ?: "Failed to load discover")
                return@launch
            }
            val genres = repository.discoverGenres().getOrElse {
                _state.value = DiscoverUiState.Error(it.message ?: "Failed to load discover genres")
                return@launch
            }
            _state.value = DiscoverUiState.Ready(discover = discover, genres = genres)
        }
    }
}

@HiltViewModel
class DiscoverBrowseViewModel @Inject constructor(
    private val repository: DiscoverRepository,
) : ViewModel() {
    private val _state = MutableStateFlow<DiscoverBrowseUiState>(DiscoverBrowseUiState.Loading)
    val state: StateFlow<DiscoverBrowseUiState> = _state.asStateFlow()

    private var cachedGenres: plum.tv.core.network.DiscoverGenresResponseJson? = null
    private var currentCategory: String? = null
    private var currentMediaType: String? = null
    private var currentGenreId: Int? = null
    private var isLoadingMore = false

    fun refresh(category: String? = null, mediaType: String? = null, genreId: Int? = null) {
        currentCategory = category
        currentMediaType = mediaType
        currentGenreId = genreId
        isLoadingMore = false

        viewModelScope.launch {
            val current = _state.value
            if (current is DiscoverBrowseUiState.Ready) {
                _state.value = current.copy(refreshing = true, loadingMore = false)
            }

            val genres = cachedGenres ?: repository.discoverGenres().getOrElse {
                if (current is DiscoverBrowseUiState.Ready) {
                    _state.value = current.copy(refreshing = false)
                } else {
                    _state.value = DiscoverBrowseUiState.Error(it.message ?: "Failed to load discover genres")
                }
                return@launch
            }
            cachedGenres = genres

            val browse = repository.browseDiscover(category, mediaType, genreId, page = 1).getOrElse {
                if (current is DiscoverBrowseUiState.Ready) {
                    _state.value = current.copy(refreshing = false)
                } else {
                    _state.value = DiscoverBrowseUiState.Error(it.message ?: "Failed to load discover browse")
                }
                return@launch
            }
            _state.value = DiscoverBrowseUiState.Ready(
                title = browse.title(),
                category = browse.category,
                mediaType = browse.mediaType,
                genre = browse.genre,
                genres = when (mediaType) {
                    "movie" -> genres.movieGenres
                    "tv" -> genres.tvGenres
                    else -> genres.movieGenres + genres.tvGenres
                },
                items = browse.items,
                totalResults = browse.totalResults,
                currentPage = browse.page,
                totalPages = browse.totalPages,
                hasMore = browse.page < browse.totalPages,
            )
        }
    }

    fun loadNextPage() {
        val current = _state.value
        if (current !is DiscoverBrowseUiState.Ready) return
        if (!current.hasMore || isLoadingMore) return

        isLoadingMore = true
        val nextPage = current.currentPage + 1

        viewModelScope.launch {
            _state.value = current.copy(loadingMore = true)

            val browse = repository.browseDiscover(
                currentCategory,
                currentMediaType,
                currentGenreId,
                page = nextPage,
            ).getOrElse {
                isLoadingMore = false
                _state.value = current.copy(loadingMore = false)
                return@launch
            }

            val existingKeys = current.items.mapTo(HashSet()) { "${it.mediaType}-${it.tmdbId}" }
            val newItems = browse.items.filter { "${it.mediaType}-${it.tmdbId}" !in existingKeys }
            val allItems = current.items + newItems

            isLoadingMore = false
            _state.value = current.copy(
                items = allItems,
                currentPage = browse.page,
                totalPages = browse.totalPages,
                totalResults = browse.totalResults,
                hasMore = browse.page < browse.totalPages,
                loadingMore = false,
            )
        }
    }

    private fun DiscoverBrowseResponseJson.title(): String {
        val genreName = genre?.name
        val categoryName = category
        return when {
            genreName != null && mediaType == "movie" -> "$genreName Movies"
            genreName != null && mediaType == "tv" -> "$genreName TV"
            genreName != null -> genreName
            !categoryName.isNullOrBlank() -> categoryName.replace('-', ' ')
                .split(' ').joinToString(" ") { it.replaceFirstChar { c -> c.uppercase() } }
            mediaType == "movie" -> "Movies"
            mediaType == "tv" -> "TV Shows"
            else -> "Browse"
        }
    }
}

@HiltViewModel
class DiscoverDetailViewModel @Inject constructor(
    private val repository: DiscoverRepository,
) : ViewModel() {
    private val _state = MutableStateFlow<DiscoverDetailUiState>(DiscoverDetailUiState.Loading)
    val state: StateFlow<DiscoverDetailUiState> = _state.asStateFlow()

    fun refresh(mediaType: String, tmdbId: Int) {
        viewModelScope.launch {
            _state.value = DiscoverDetailUiState.Loading
            val details = repository.discoverTitleDetails(mediaType, tmdbId).getOrElse {
                _state.value = DiscoverDetailUiState.Error(it.message ?: "Failed to load discover title")
                return@launch
            }
            if (details == null) {
                _state.value = DiscoverDetailUiState.Error("Title not found")
                return@launch
            }
            _state.value = DiscoverDetailUiState.Ready(details)
        }
    }

    fun addTitle(mediaType: String, tmdbId: Int) {
        viewModelScope.launch {
            repository.addDiscoverTitle(mediaType, tmdbId)
            refresh(mediaType, tmdbId)
        }
    }
}

private const val DOWNLOADS_POLL_INTERVAL_MS = 5_000L

@HiltViewModel
class DownloadsViewModel @Inject constructor(
    private val repository: DiscoverRepository,
) : ViewModel() {
    private val _state = MutableStateFlow<DownloadsUiState>(DownloadsUiState.Loading)
    val state: StateFlow<DownloadsUiState> = _state.asStateFlow()

    init {
        startPolling()
    }

    fun refresh() {
        viewModelScope.launch {
            _state.value = DownloadsUiState.Loading
            fetchOnce()
        }
    }

    private fun startPolling() {
        viewModelScope.launch {
            while (true) {
                fetchOnce()
                delay(DOWNLOADS_POLL_INTERVAL_MS)
            }
        }
    }

    private suspend fun fetchOnce() {
        val downloads = repository.downloads().getOrElse {
            _state.value = DownloadsUiState.Error(it.message ?: "Failed to load downloads")
            return
        }
        _state.value = DownloadsUiState.Ready(
            configured = downloads.configured,
            items = downloads.items,
        )
    }
}
