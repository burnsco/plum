package plum.tv.feature.library

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.itemsIndexed
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.layout.ContentScale
import androidx.hilt.navigation.compose.hiltViewModel
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
import plum.tv.core.ui.PlumTheme
import plum.tv.core.ui.LaunchedTvFocusTo
import plum.tv.core.ui.PlumStatePanel
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
    val firstPosterFocus = remember { FocusRequester() }
    val metrics = PlumTheme.metrics
    val minCell = metrics.posterCompactWidth + metrics.cardGap

    // Observe scroll and UI state in one snapshotFlow so row counts / flags stay in sync
    // with the visible index (LaunchedEffect(gridState) alone would freeze `state` in the collector).
    LaunchedEffect(gridState) {
        snapshotFlow {
            val lastVisible = gridState.layoutInfo.visibleItemsInfo.lastOrNull()?.index
            when (val s = state) {
                is LibraryBrowseUiState.Ready ->
                    Triple(
                        lastVisible,
                        s.rows.size,
                        s.hasMore && !s.loadingMore,
                    )
                else -> Triple(lastVisible, 0, false)
            }
        }
            .distinctUntilChanged()
            .collect { (lastVisible, rowCount, canLoadMore) ->
                if (lastVisible != null && canLoadMore && lastVisible >= rowCount - 24) {
                    viewModel.loadMore()
                }
            }
    }

    when (val s = state) {
        is LibraryBrowseUiState.Loading -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Loading library",
                message = "Scanning the shelf and grouping titles.",
            )
        }
        is LibraryBrowseUiState.Error -> Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            PlumStatePanel(
                title = "Could not load this library",
                message = s.message,
                actions = {
                    PlumActionButton(
                        label = "Retry",
                        onClick = { viewModel.loadInitial(forceNetwork = true) },
                        variant = PlumButtonVariant.Primary,
                        leadingBadge = "R",
                    )
                },
            )
        }
        is LibraryBrowseUiState.Ready -> {
            LaunchedTvFocusTo(
                s.rows.firstOrNull().let { row ->
                    when (row) {
                        is LibraryBrowseGridRow.Movie -> "m-${row.item.id}"
                        is LibraryBrowseGridRow.Show -> "s-${row.row.showKey}"
                        null -> "empty"
                    }
                },
                focusRequester = firstPosterFocus,
            )
            LazyVerticalGrid(
            columns = GridCells.Adaptive(minSize = minCell),
            state = gridState,
            modifier = Modifier.fillMaxSize(),
            contentPadding = PlumScreenPadding(),
            horizontalArrangement = Arrangement.spacedBy(metrics.cardGap),
            verticalArrangement = Arrangement.spacedBy(metrics.cardGap),
        ) {
            if (s.rows.isEmpty()) {
                item(span = { GridItemSpan(maxLineSpan) }) {
                    PlumStatePanel(
                        modifier = Modifier.fillMaxWidth(),
                        title = "No titles in this library",
                        message = "This shelf is empty or still syncing from the server.",
                    )
                }
            }
            itemsIndexed(
                s.rows,
                key = { _, row ->
                    when (row) {
                        is LibraryBrowseGridRow.Movie -> "m-${row.item.id}"
                        is LibraryBrowseGridRow.Show -> "s-${row.row.showKey}"
                    }
                },
            ) { index, row ->
                val posterModifier =
                    if (index == 0 && s.rows.isNotEmpty()) {
                        Modifier.focusRequester(firstPosterFocus)
                    } else {
                        Modifier
                    }
                when (row) {
                    is LibraryBrowseGridRow.Movie ->
                        BrowseMoviePosterCard(row.item, posterModifier) {
                            val lib = row.item.libraryId ?: return@BrowseMoviePosterCard
                            when (row.item.type) {
                                "movie" -> onOpenMovie(lib, row.item.id)
                                "tv", "anime" -> onOpenShow(lib, showKeyForBrowseItem(row.item))
                                else -> onOpenMovie(lib, row.item.id)
                            }
                        }
                    is LibraryBrowseGridRow.Show ->
                        BrowseShowPosterCard(row.row, posterModifier) {
                            val lib = row.row.posterItem.libraryId ?: return@BrowseShowPosterCard
                            onOpenShow(lib, row.row.showKey)
                        }
                }
            }
        }
        }
    }
}

@Composable
private fun BrowseMoviePosterCard(
    item: LibraryBrowseItemJson,
    modifier: Modifier = Modifier,
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
        modifier = modifier,
        compact = true,
        progressPercent = item.progressPercent,
        watched = item.completed == true,
        focusedScale = 1f,
        imageContentScale = ContentScale.Crop,
    )
}

@Composable
private fun BrowseShowPosterCard(
    row: LibraryShowBrowseRow,
    modifier: Modifier = Modifier,
    onClick: () -> Unit,
) {
    val serverBase = LocalServerBaseUrl.current
    val ep = row.posterItem
    val sz = PlumImageSizes.POSTER_GRID
    val unwatched = row.episodes.count { it.completed != true }
    val showSubtitle =
        when {
            unwatched == 0 -> "${row.episodes.size} episodes · Watched"
            unwatched == row.episodes.size -> "${row.episodes.size} episodes"
            else -> "${row.episodes.size} episodes · $unwatched left"
        }
    PlumPosterCard(
        title = row.displayTitle,
        subtitle = showSubtitle,
        imageUrl =
            resolveArtworkUrl(serverBase, ep.showPosterUrl, ep.showPosterPath, sz)
                ?: resolveArtworkUrl(serverBase, ep.posterUrl, ep.posterPath, sz)
                ?: ep.thumbnailUrl?.takeIf { it.isNotBlank() }?.let { resolveImageUrl(serverBase, it) }
                ?: ep.thumbnailPath?.takeIf { it.isNotBlank() }?.let { resolveImageUrl(serverBase, it) },
        onClick = onClick,
        modifier = modifier,
        compact = true,
        watched = unwatched == 0 && row.episodes.isNotEmpty(),
        focusedScale = 1f,
        imageContentScale = ContentScale.Crop,
    )
}
