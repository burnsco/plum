package plum.tv.feature.library

import plum.tv.core.network.LibraryBrowseItemJson
import plum.tv.core.network.LibraryShowBrowseRow

sealed interface LibraryBrowseGridRow {
    data class Movie(val item: LibraryBrowseItemJson) : LibraryBrowseGridRow

    data class Show(val row: LibraryShowBrowseRow) : LibraryBrowseGridRow
}
