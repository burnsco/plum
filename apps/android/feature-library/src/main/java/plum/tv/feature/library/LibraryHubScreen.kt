package plum.tv.feature.library

import androidx.compose.runtime.Composable
import androidx.hilt.lifecycle.viewmodel.compose.hiltViewModel

/**
 * Side-rail destination for a category (TV, Movies, Anime): choose a library of that type, then open its shelf.
 * [onPlayMedia] and [onOpenShow] are reserved for future hub content (e.g. type-scoped rails).
 */
@Suppress("UNUSED_PARAMETER")
@Composable
fun LibraryHubRoute(
    libraryType: String?,
    onPlayMedia: (
        mediaId: Int,
        resumeSec: Float,
        libraryId: Int?,
        showKey: String?,
        displayTitle: String?,
        displaySubtitle: String?,
    ) -> Unit,
    onOpenShow: (libraryId: Int, showKey: String) -> Unit,
    onOpenLibrary: (libraryId: Int) -> Unit,
    viewModel: LibraryListViewModel = hiltViewModel(),
) {
    LibraryListRoute(
        onOpenLibrary = onOpenLibrary,
        libraryType = libraryType,
        viewModel = viewModel,
    )
}
