package plum.tv.feature.search

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.Card
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.ExperimentalTvMaterial3Api
import coil.compose.AsyncImage
import plum.tv.core.network.SearchResultJson

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
fun SearchRoute(
    onOpenMovie: (libraryId: Int, mediaId: Int) -> Unit,
    onOpenShow: (libraryId: Int, showKey: String) -> Unit,
    viewModel: SearchViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val trimmedQuery = state.query.trim()

    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(48.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text("Search", modifier = Modifier.padding(bottom = 4.dp))

        OutlinedTextField(
            value = state.query,
            onValueChange = viewModel::setQuery,
            singleLine = true,
            label = { Text("Query") },
            modifier = Modifier.fillMaxWidth(),
        )

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

        when {
            state.loading -> Text("Searching…")
            trimmedQuery.length < 2 -> Text("Type at least 2 characters to search.")
            state.error != null -> {
                Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    Text(state.error ?: "Search failed")
                    Button(
                        onClick = { viewModel.retry() },
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(52.dp),
                        scale = ButtonDefaults.scale(focusedScale = 1.08f),
                    ) {
                        Text("Retry")
                    }
                }
            }
            state.results.isEmpty() -> Text("No results for \"$trimmedQuery\".")
            else -> {
                LazyVerticalGrid(
                    columns = GridCells.Fixed(6),
                    modifier = Modifier.fillMaxWidth(),
                    contentPadding = PaddingValues(top = 8.dp),
                    horizontalArrangement = Arrangement.spacedBy(16.dp),
                    verticalArrangement = Arrangement.spacedBy(16.dp),
                ) {
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
}

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
private fun TypeButton(selected: Boolean, label: String, onClick: () -> Unit) {
    Button(
        onClick = onClick,
        modifier = Modifier.height(48.dp),
        scale = ButtonDefaults.scale(focusedScale = 1.08f),
    ) {
        Text(if (selected) "▶ $label" else label)
    }
}

@OptIn(ExperimentalTvMaterial3Api::class)
@Composable
private fun SearchResultCard(result: SearchResultJson, onClick: () -> Unit) {
    val posterUrl = result.posterUrl ?: result.posterPath
    Card(
        onClick = onClick,
        modifier = Modifier
            .width(180.dp)
            .height(270.dp),
        scale = CardDefaults.scale(focusedScale = 1.08f),
    ) {
        Column {
            if (posterUrl != null) {
                AsyncImage(
                    model = posterUrl,
                    contentDescription = result.title,
                    modifier = Modifier
                        .width(180.dp)
                        .height(210.dp),
                )
            } else {
                Text(
                    text = result.title,
                    modifier = Modifier.padding(8.dp),
                    maxLines = 4,
                    overflow = TextOverflow.Ellipsis,
                )
            }

            Text(
                text = result.title,
                modifier = Modifier.padding(8.dp),
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )

            result.subtitle?.takeIf { it.isNotBlank() }?.let { sub ->
                Text(
                    text = sub,
                    modifier = Modifier.padding(horizontal = 8.dp),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
        }
    }
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
            "show" -> {
                val showKey = parts[3]
                onOpenShow(libraryId, showKey)
            }
        }
        return
    }

    // Fallback: try based on server-provided kind.
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

