package plum.tv.feature.search

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.FlowPreview
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.debounce
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.flow.collectLatest
import plum.tv.core.data.SearchRepository
import plum.tv.core.network.SearchResultJson

data class SearchUiState(
    val query: String = "",
    val type: SearchType? = null,
    val loading: Boolean = false,
    val error: String? = null,
    val results: List<SearchResultJson> = emptyList(),
)

enum class SearchType(val queryParamValue: String) {
    Movies("movie"),
    Shows("show"),
}

@HiltViewModel
class SearchViewModel @Inject constructor(
    private val searchRepository: SearchRepository,
) : ViewModel() {

    private val _query = MutableStateFlow("")
    private val _type = MutableStateFlow<SearchType?>(null)
    private val _retryTick = MutableStateFlow(0)

    private val _state = MutableStateFlow(SearchUiState())
    val state: StateFlow<SearchUiState> = _state.asStateFlow()

    init {
        observeSearch()
    }

    fun setQuery(newQuery: String) {
        _query.value = newQuery
    }

    fun setType(newType: SearchType?) {
        _type.value = newType
    }

    fun retry() {
        _retryTick.update { it + 1 }
    }

    @OptIn(FlowPreview::class)
    private fun observeSearch() {
        viewModelScope.launch {
            combine(_query, _type, _retryTick) { q, t, r -> Triple(q, t, r) }
                .debounce(350)
                .map { (q, t, r) -> Triple(q.trim(), t, r) }
                .distinctUntilChanged()
                .collectLatest { (trimmedQuery, type, _) ->
                    _state.update {
                        it.copy(
                            query = trimmedQuery,
                            type = type,
                            loading = false,
                            error = null,
                            results = emptyList(),
                        )
                    }
                    if (trimmedQuery.length < 2) {
                        return@collectLatest
                    }
                    _state.update { it.copy(loading = true, error = null) }
                    val res =
                        searchRepository.searchLibraryMedia(
                            query = trimmedQuery,
                            type = type?.queryParamValue,
                            limit = 30,
                        )
                    res.fold(
                        onSuccess = { response ->
                            _state.update {
                                it.copy(
                                    loading = false,
                                    error = null,
                                    results = response.results,
                                )
                            }
                        },
                        onFailure = { e ->
                            _state.update {
                                it.copy(
                                    loading = false,
                                    error = e.message ?: "Search failed",
                                    results = emptyList(),
                                )
                            }
                        },
                    )
                }
        }
    }
}
