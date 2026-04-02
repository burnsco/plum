package plum.tv.feature.home

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
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
import plum.tv.core.network.ContinueWatchingEntryJson
import plum.tv.core.network.MediaItemJson
import plum.tv.core.network.RecentlyAddedEntryJson

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
fun HomeRoute(
    onPlayMovie: (mediaId: Int, resumeSec: Float) -> Unit,
    onOpenShow: (libraryId: Int, showKey: String) -> Unit,
    viewModel: HomeViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    when (val s = state) {
        is HomeUiState.Loading -> Text("Loading…", modifier = Modifier.padding(48.dp))
        is HomeUiState.Error -> Column(Modifier.padding(48.dp)) {
            Text(s.message)
            Button(onClick = { viewModel.refresh() }) { Text("Retry") }
        }
        is HomeUiState.Ready -> HomeContent(
            continueWatching = s.continueWatching,
            recentlyAdded = s.recentlyAdded,
            onPlayMovie = onPlayMovie,
            onOpenShow = onOpenShow,
        )
    }
}

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
private fun HomeContent(
    continueWatching: List<ContinueWatchingEntryJson>,
    recentlyAdded: List<RecentlyAddedEntryJson>,
    onPlayMovie: (mediaId: Int, resumeSec: Float) -> Unit,
    onOpenShow: (libraryId: Int, showKey: String) -> Unit,
) {
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = 48.dp, vertical = 32.dp),
        verticalArrangement = Arrangement.spacedBy(28.dp),
    ) {
        item {
            Text("Continue watching", modifier = Modifier.padding(bottom = 8.dp))
        }
        item {
            LazyRow(
                horizontalArrangement = Arrangement.spacedBy(16.dp),
                contentPadding = PaddingValues(end = 48.dp),
            ) {
                items(continueWatching, key = { it.media.id }) { entry ->
                    MediaEntryCard(
                        entry.media,
                        subtitle = entry.episodeLabel ?: entry.showTitle,
                        onClick = {
                            when (entry.kind) {
                                "movie" -> {
                                    val resume = (entry.media.progressSeconds ?: 0.0).toFloat()
                                    onPlayMovie(entry.media.id, resume)
                                }
                                "show" -> {
                                    val key = entry.showKey
                                    val lib = entry.media.libraryId ?: 0
                                    if (key != null) onOpenShow(lib, key)
                                }
                            }
                        },
                    )
                }
            }
        }
        item {
            Text("Recently added", modifier = Modifier.padding(top = 8.dp, bottom = 8.dp))
        }
        item {
            LazyRow(
                horizontalArrangement = Arrangement.spacedBy(16.dp),
                contentPadding = PaddingValues(end = 48.dp),
            ) {
                items(recentlyAdded, key = { it.media.id }) { entry ->
                    MediaEntryCard(
                        entry.media,
                        subtitle = entry.episodeLabel ?: entry.showTitle,
                        onClick = {
                            when (entry.kind) {
                                "movie" -> {
                                    val resume = (entry.media.progressSeconds ?: 0.0).toFloat()
                                    onPlayMovie(entry.media.id, resume)
                                }
                                "show" -> {
                                    val key = entry.showKey
                                    val lib = entry.media.libraryId ?: 0
                                    if (key != null) onOpenShow(lib, key)
                                }
                            }
                        },
                    )
                }
            }
        }
    }
}

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
private fun MediaEntryCard(
    media: MediaItemJson,
    subtitle: String?,
    onClick: () -> Unit,
) {
    val url = media.posterUrl ?: media.showPosterUrl ?: media.thumbnailUrl
    Card(
        onClick = onClick,
        modifier = Modifier
            .width(200.dp)
            .height(300.dp),
        scale = CardDefaults.scale(focusedScale = 1.08f),
        colors = CardDefaults.colors(),
    ) {
        Column {
            if (url != null) {
                AsyncImage(
                    model = url,
                    contentDescription = media.title,
                    modifier = Modifier
                        .width(200.dp)
                        .height(240.dp),
                    contentScale = ContentScale.Crop,
                )
            } else {
                Text(
                    text = media.title,
                    modifier = Modifier
                        .width(200.dp)
                        .height(240.dp)
                        .padding(8.dp),
                    maxLines = 4,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Text(
                text = media.title,
                modifier = Modifier.padding(8.dp),
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
            subtitle?.let {
                Text(
                    text = it,
                    modifier = Modifier.padding(horizontal = 8.dp),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
}
