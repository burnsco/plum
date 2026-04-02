package plum.tv.feature.library

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.itemsIndexed
import androidx.compose.foundation.lazy.grid.rememberLazyGridState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Modifier
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.tv.material3.Button
import androidx.tv.material3.Card
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.ExperimentalTvMaterial3Api
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import kotlinx.coroutines.flow.distinctUntilChanged
import plum.tv.core.network.LibraryBrowseItemJson
import plum.tv.core.network.showKeyForBrowseItem

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
fun LibraryBrowseRoute(
    onOpenMovie: (libraryId: Int, mediaId: Int) -> Unit,
    onOpenShow: (libraryId: Int, showKey: String) -> Unit,
    viewModel: LibraryBrowseViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val gridState = rememberLazyGridState()

    LaunchedEffect(gridState, state) {
        snapshotFlow { gridState.layoutInfo.visibleItemsInfo.lastOrNull()?.index }
            .distinctUntilChanged()
            .collect { last ->
                if (last != null && state is LibraryBrowseUiState.Ready) {
                    val ready = state as LibraryBrowseUiState.Ready
                    if (last >= ready.items.size - 12) {
                        viewModel.loadMore()
                    }
                }
            }
    }

    when (val s = state) {
        is LibraryBrowseUiState.Loading -> Text("Loading…", modifier = Modifier.padding(48.dp))
        is LibraryBrowseUiState.Error -> Column(Modifier.padding(48.dp)) {
            Text(s.message)
            Button(onClick = { viewModel.loadInitial() }) { Text("Retry") }
        }
        is LibraryBrowseUiState.Ready -> LazyVerticalGrid(
            columns = GridCells.Fixed(6),
            state = gridState,
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(48.dp),
            horizontalArrangement = Arrangement.spacedBy(16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            itemsIndexed(s.items, key = { _, it -> it.id }) { _, item ->
                BrowsePosterCard(item) {
                    val lib = item.libraryId ?: return@BrowsePosterCard
                    when (item.type) {
                        "movie" -> onOpenMovie(lib, item.id)
                        "tv", "anime" -> {
                            val key = showKeyForBrowseItem(item)
                            onOpenShow(lib, key)
                        }
                        else -> onOpenMovie(lib, item.id)
                    }
                }
            }
        }
    }
}

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
private fun BrowsePosterCard(
    item: LibraryBrowseItemJson,
    onClick: () -> Unit,
) {
    val url = item.posterUrl ?: item.showPosterUrl ?: item.thumbnailUrl
    Card(
        onClick = onClick,
        modifier = Modifier
            .width(180.dp)
            .height(270.dp),
        scale = CardDefaults.scale(focusedScale = 1.08f),
    ) {
        Column {
            if (url != null) {
                AsyncImage(
                    model = url,
                    contentDescription = item.title,
                    modifier = Modifier
                        .width(180.dp)
                        .height(210.dp),
                    contentScale = ContentScale.Crop,
                )
            } else {
                Text(
                    text = item.title,
                    modifier = Modifier
                        .width(180.dp)
                        .height(210.dp)
                        .padding(8.dp),
                    maxLines = 4,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Text(
                text = item.title,
                modifier = Modifier.padding(8.dp),
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
        }
    }
}
