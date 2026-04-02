package plum.tv.feature.library

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.tv.material3.Text
import plum.tv.core.network.LibraryJson
import plum.tv.core.ui.PlumActionButton
import plum.tv.core.ui.PlumButtonVariant
import plum.tv.core.ui.PlumPanel
import plum.tv.core.ui.PlumScreenPadding
import plum.tv.core.ui.PlumScreenTitle
import plum.tv.core.ui.PlumTheme

@Composable
fun LibraryListRoute(
    onOpenLibrary: (libraryId: Int) -> Unit,
    libraryType: String? = null,
    viewModel: LibraryListViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    when (val s = state) {
        is LibraryListUiState.Loading -> Text("Loading libraries...", modifier = Modifier.fillMaxSize())
        is LibraryListUiState.Error -> LazyVerticalGrid(
            columns = GridCells.Fixed(1),
            modifier = Modifier.fillMaxSize(),
            contentPadding = PlumScreenPadding(),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            item { PlumScreenTitle("Libraries", "We could not load your library list.") }
            item { Text(s.message, color = PlumTheme.palette.muted) }
            item {
                PlumActionButton(
                    label = "Retry",
                    onClick = { viewModel.refresh() },
                    variant = PlumButtonVariant.Primary,
                    leadingBadge = "R",
                )
            }
        }
        is LibraryListUiState.Ready -> {
            val libraries = filterLibraries(s.libraries, libraryType)
            LazyVerticalGrid(
                columns = GridCells.Fixed(3),
                modifier = Modifier.fillMaxSize(),
                contentPadding = PlumScreenPadding(),
                horizontalArrangement = Arrangement.spacedBy(24.dp),
                verticalArrangement = Arrangement.spacedBy(24.dp),
            ) {
                item(span = { GridItemSpan(maxLineSpan) }) {
                    PlumScreenTitle(
                        title = libraryTypeLabel(libraryType),
                        subtitle = "Jump into movies, shows, anime, and music shelves.",
                    )
                }
                items(libraries, key = { it.id }) { lib ->
                    PlumPanel(modifier = Modifier.fillMaxWidth()) {
                        Column(verticalArrangement = Arrangement.spacedBy(14.dp)) {
                            PlumScreenTitle(
                                title = lib.name,
                                subtitle = libraryLabel(lib),
                            )
                            PlumActionButton(
                                label = "Open library",
                                onClick = { onOpenLibrary(lib.id) },
                                variant = PlumButtonVariant.Secondary,
                                leadingBadge = "GO",
                            )
                        }
                    }
                }
            }
        }
    }
}

private fun filterLibraries(
    libraries: List<LibraryJson>,
    libraryType: String?,
): List<LibraryJson> {
    if (libraryType.isNullOrBlank()) return libraries
    return libraries.filter { lib ->
        when (libraryType) {
            "movie" -> lib.type == "movie"
            "music" -> lib.type == "music"
            "anime" -> lib.type == "anime" || (lib.type == "tv" && lib.name.contains("anime", ignoreCase = true))
            "tv" -> lib.type == "tv" && !lib.name.contains("anime", ignoreCase = true)
            else -> true
        }
    }
}

private fun libraryTypeLabel(libraryType: String?): String =
    when (libraryType) {
        "movie" -> "Movies"
        "music" -> "Music"
        "anime" -> "Anime"
        "tv" -> "TV"
        else -> "Libraries"
    }

private fun libraryLabel(lib: LibraryJson): String =
    when {
        lib.type == "movie" -> "Movies"
        lib.type == "music" -> "Music"
        lib.type == "anime" || (lib.type == "tv" && lib.name.contains("anime", ignoreCase = true)) -> "Anime"
        lib.type == "tv" -> "TV"
        else -> lib.type.replaceFirstChar { it.uppercase() }
    }
