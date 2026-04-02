package plum.tv.app

import androidx.lifecycle.ViewModel
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import plum.tv.core.data.BrowseRepository
import plum.tv.feature.library.filterLibrariesByType
import plum.tv.feature.library.libraryRailType

@HiltViewModel
class MainNavViewModel @Inject constructor(
    private val browseRepository: BrowseRepository,
) : ViewModel() {

    suspend fun firstLibraryIdForType(type: String): Int? {
        val libs = browseRepository.libraries().getOrNull() ?: return null
        return filterLibrariesByType(libs, type).firstOrNull()?.id
    }

    suspend fun railTypeForBrowseLibraryId(libraryId: Int): String? {
        val libs = browseRepository.libraries().getOrNull() ?: return null
        val lib = libs.find { it.id == libraryId } ?: return null
        return libraryRailType(lib)
    }
}
