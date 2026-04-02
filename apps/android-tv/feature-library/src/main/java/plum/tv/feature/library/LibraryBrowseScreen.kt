package plum.tv.feature.library

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.itemsIndexed
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.tv.material3.Text
import kotlinx.coroutines.flow.distinctUntilChanged
import plum.tv.core.network.LibraryBrowseItemJson
import plum.tv.core.network.LibraryShowBrowseRow
import plum.tv.core.network.showKeyForBrowseItem
import plum.tv.core.ui.LocalServerBaseUrl
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumImageSizes
import plum.tv.core.ui.PlumPosterCard
import plum.tv.core.ui.PlumScreenPadding
import plum.tv.core.ui.PlumScreenTitle
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.resolveArtworkUrl
import plum.tv.core.ui.resolveImageUrl

@Composable
fun LibraryBrowseRoute(
    onOpenMovie: (libraryId: Int, mediaId: Int) -> Unit,
    onOpenShow: (libraryId: Int, showKey: String) -> Unit,
    viewModel: LibraryBrowseViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val gridState = rememberLazyGridState()
    val metrics = PlumTheme.metrics
    val minCell = metrics.posterWidth + metrics.cardGap + 8.dp

    LaunchedEffect(gridState, state) {
        snapshotFlow { gridState.layoutInfo.visibleItemsInfo.lastOrNull()?.index }
            .distinctUntilChanged()
            .collect { last ->
                if (last != null && state is LibraryBrowseUiState.Ready) {
                    val ready = state as LibraryBrowseUiState.Ready
                    if (last >= ready.rows.size - 12) {
                        viewModel.loadMore()
                    }
                }
            }
    }

    when (val s = state) {
        is LibraryBrowseUiState.Loading -> Text("Loading...", modifier = Modifier.padding(48.dp))
        is LibraryBrowseUiState.Error -> {
            LazyVerticalGrid(
                columns = GridCells.Fixed(1),
                modifier = Modifier.fillMaxSize(),
                contentPadding = PlumScreenPadding(),
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                item {
                    PlumScreenTitle("Library", "We could not load this library right now.")
                }
                item {
                    Text(s.message, color = PlumTheme.palette.muted)
                }
                item {
                    PlumActionButton(
                        label = "Retry",
                        onClick = { viewModel.loadInitial() },
                        variant = PlumButtonVariant.Primary,
                        leadingBadge = "R",
                    )
                }
            }
        }
        is LibraryBrowseUiState.Ready -> LazyVerticalGrid(
            columns = GridCells.Adaptive(minSize = minCell),
            state = gridState,
            modifier = Modifier.fillMaxSize(),
            contentPadding = PlumScreenPadding(),
            horizontalArrangement = Arrangement.spacedBy(PlumTheme.metrics.cardGap),
            verticalArrangement = Arrangement.spacedBy(PlumTheme.metrics.sectionGap),
        ) {
            item(span = { GridItemSpan(maxLineSpan) }) {
                PlumScreenTitle(
                    title = "Library",
                    subtitle = "Browse your collection in the same refined Plum style as the web app.",
                    modifier = Modifier.padding(bottom = 8.dp),
                )
            }
            itemsIndexed(
                s.rows,
                key = { _, row ->
                    when (row) {
                        is LibraryBrowseGridRow.Movie -> "m-${row.item.id}"
                        is LibraryBrowseGridRow.Show -> "s-${row.row.showKey}"
                    }
                },
            ) { _, row ->
                when (row) {
                    is LibraryBrowseGridRow.Movie ->
                        BrowseMoviePosterCard(row.item) {
                            val lib = row.item.libraryId ?: return@BrowseMoviePosterCard
                            when (row.item.type) {
                                "movie" -> onOpenMovie(lib, row.item.id)
                                "tv", "anime" -> onOpenShow(lib, showKeyForBrowseItem(row.item))
                                else -> onOpenMovie(lib, row.item.id)
                            }
                        }
                    is LibraryBrowseGridRow.Show ->
                        BrowseShowPosterCard(row.row) {
                            val lib = row.row.posterItem.libraryId ?: return@BrowseShowPosterCard
                            onOpenShow(lib, row.row.showKey)
                        }
                }
            }
        }
    }
}

@Composable
private fun BrowseMoviePosterCard(
    item: LibraryBrowseItemJson,
    onClick: () -> Unit,
) {
    val serverBase = LocalServerBaseUrl.current
    val sz = PlumImageSizes.POSTER_GRID
    PlumPosterCard(
        title = item.title,
        subtitle = item.releaseDate?.take(4) ?: item.type,
        imageUrl =
            resolveArtworkUrl(serverBase, item.posterUrl, item.posterPath, sz)
                ?: resolveArtworkUrl(serverBase, item.showPosterUrl, item.showPosterPath, sz)
                ?: item.thumbnailUrl?.takeIf { it.isNotBlank() }?.let { resolveImageUrl(serverBase, it) }
                ?: item.thumbnailPath?.takeIf { it.isNotBlank() }?.let { resolveImageUrl(serverBase, it) },
        onClick = onClick,
    )
}

@Composable
private fun BrowseShowPosterCard(
    row: LibraryShowBrowseRow,
    onClick: () -> Unit,
) {
    val serverBase = LocalServerBaseUrl.current
    val ep = row.posterItem
    val sz = PlumImageSizes.POSTER_GRID
    PlumPosterCard(
        title = row.displayTitle,
        subtitle = "${row.episodes.size} episodes",
        imageUrl =
            resolveArtworkUrl(serverBase, ep.showPosterUrl, ep.showPosterPath, sz)
                ?: resolveArtworkUrl(serverBase, ep.posterUrl, ep.posterPath, sz)
                ?: ep.thumbnailUrl?.takeIf { it.isNotBlank() }?.let { resolveImageUrl(serverBase, it) }
                ?: ep.thumbnailPath?.takeIf { it.isNotBlank() }?.let { resolveImageUrl(serverBase, it) },
        onClick = onClick,
    )
}
