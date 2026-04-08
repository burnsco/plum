package plum.tv.feature.search

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.unit.dp
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel
import plum.tv.core.network.SearchResultJson
import plum.tv.core.ui.LaunchedTvFocusTo
import plum.tv.core.ui.LocalServerBaseUrl
import plum.tv.core.ui.PlumImageSizes
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumPosterCard
import plum.tv.core.ui.PlumScreenPadding
import plum.tv.core.ui.PlumScreenTitle
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.plumOutlinedFieldColors
import plum.tv.core.ui.resolveArtworkUrl

@Composable
fun SearchRoute(
    onOpenMovie: (libraryId: Int, mediaId: Int) -> Unit,
    onOpenShow: (libraryId: Int, showKey: String) -> Unit,
    viewModel: SearchViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val queryFieldFocus = remember { FocusRequester() }
    LaunchedTvFocusTo(focusRequester = queryFieldFocus)
    val trimmedQuery = state.query.trim()
    val metrics = PlumTheme.metrics
    val minCell = metrics.posterWidth + metrics.cardGap + 8.dp

    LazyVerticalGrid(
        columns = GridCells.Adaptive(minSize = minCell),
        modifier = Modifier.fillMaxSize(),
        contentPadding = PlumScreenPadding(),
        horizontalArrangement = Arrangement.spacedBy(PlumTheme.metrics.cardGap),
        verticalArrangement = Arrangement.spacedBy(PlumTheme.metrics.sectionGap),
    ) {
        item(span = { androidx.compose.foundation.lazy.grid.GridItemSpan(maxLineSpan) }) {
            PlumScreenTitle("Search", "Find movies and shows across your libraries.")
        }
        item(span = { androidx.compose.foundation.lazy.grid.GridItemSpan(maxLineSpan) }) {
            OutlinedTextField(
                value = state.query,
                onValueChange = viewModel::setQuery,
                singleLine = true,
                label = { Text("Query") },
                modifier = Modifier.fillMaxWidth().focusRequester(queryFieldFocus),
                colors = plumOutlinedFieldColors(),
            )
        }
        item(span = { androidx.compose.foundation.lazy.grid.GridItemSpan(maxLineSpan) }) {
            Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                TypeButton(selected = state.type == null, label = "All", onClick = { viewModel.setType(null) })
                TypeButton(
                    selected = state.type == SearchType.Movies,
                    label = "Movies",
                    onClick = { viewModel.setType(SearchType.Movies) },
                )
                TypeButton(
                    selected = state.type == SearchType.Shows,
                    label = "Shows",
                    onClick = { viewModel.setType(SearchType.Shows) },
                )
            }
        }

        when {
            state.loading -> item(span = { androidx.compose.foundation.lazy.grid.GridItemSpan(maxLineSpan) }) {
                Text("Searching...", color = PlumTheme.palette.muted)
            }
            trimmedQuery.length < 2 -> item(span = { androidx.compose.foundation.lazy.grid.GridItemSpan(maxLineSpan) }) {
                Text("Type at least 2 characters to search.", color = PlumTheme.palette.muted)
            }
            state.error != null -> {
                item(span = { androidx.compose.foundation.lazy.grid.GridItemSpan(maxLineSpan) }) {
                    Text(state.error ?: "Search failed", color = PlumTheme.palette.muted)
                }
                item(span = { androidx.compose.foundation.lazy.grid.GridItemSpan(maxLineSpan) }) {
                    PlumActionButton("Retry", onClick = { viewModel.retry() }, leadingBadge = "R")
                }
            }
            state.results.isEmpty() -> item(span = { androidx.compose.foundation.lazy.grid.GridItemSpan(maxLineSpan) }) {
                Text("No results for \"$trimmedQuery\".", color = PlumTheme.palette.muted)
            }
            else -> {
                items(state.results, key = { it.href }) { result ->
                    SearchResultCard(
                        result = result,
                        onClick = {
                            handleOpenHref(
                                result = result,
                                onOpenMovie = onOpenMovie,
                                onOpenShow = onOpenShow,
                            )
                        },
                    )
                }
            }
        }
    }
}

@Composable
private fun TypeButton(selected: Boolean, label: String, onClick: () -> Unit) {
    PlumActionButton(
        label = label,
        onClick = onClick,
        variant = if (selected) PlumButtonVariant.Primary else PlumButtonVariant.Secondary,
        leadingBadge = if (selected) "ON" else null,
    )
}

@Composable
private fun SearchResultCard(result: SearchResultJson, onClick: () -> Unit) {
    val serverBase = LocalServerBaseUrl.current
    PlumPosterCard(
        title = result.title,
        subtitle = result.subtitle,
        imageUrl = resolveArtworkUrl(serverBase, result.posterUrl, result.posterPath, PlumImageSizes.POSTER_GRID),
        onClick = onClick,
        focusedScale = 1f,
    )
}

private fun handleOpenHref(
    result: SearchResultJson,
    onOpenMovie: (libraryId: Int, mediaId: Int) -> Unit,
    onOpenShow: (libraryId: Int, showKey: String) -> Unit,
) {
    val parts = result.href.trim().trim('/').split('/')
    if (parts.size == 4 && parts[0] == "library") {
        val libraryId = parts[1].toIntOrNull() ?: result.libraryId
        when (parts[2]) {
            "movie" -> {
                val mediaId = parts[3].toIntOrNull() ?: return
                onOpenMovie(libraryId, mediaId)
            }
            "show" -> onOpenShow(libraryId, parts[3])
        }
        return
    }

    when (result.kind) {
        "movie" -> {
            val mediaId = parts.lastOrNull()?.toIntOrNull() ?: return
            onOpenMovie(result.libraryId, mediaId)
        }
        "show" -> {
            val showKey = parts.lastOrNull() ?: return
            onOpenShow(result.libraryId, showKey)
        }
    }
}
