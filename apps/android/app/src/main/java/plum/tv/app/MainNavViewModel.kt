package plum.tv.app

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.launch
import plum.tv.core.data.BrowseRepository
import plum.tv.feature.library.filterLibrariesByType
import plum.tv.feature.library.libraryRailType

@HiltViewModel
class MainNavViewModel @Inject constructor(
    private val browseRepository: BrowseRepository,
) : ViewModel() {

    /**
     * When exactly one library matches [type] (TV / Movies / Anime / Music), return its id so the
     * side rail can open browse directly; otherwise callers should show the type hub picker.
     */
    suspend fun soleLibraryIdForType(type: String): Int? {
        val libs = browseRepository.libraries().getOrNull() ?: return null
        return filterLibrariesByType(libs, type).singleOrNull()?.id
    }

    /** Warm first page of every library as soon as the shell is up (not only after home loads). */
    fun scheduleLibraryMediaPrefetch() {
        viewModelScope.launch {
            browseRepository.prefetchFirstLibraryMediaPages()
        }
    }

    suspend fun railTypeForBrowseLibraryId(libraryId: Int): String? {
        val libs = browseRepository.libraries().getOrNull() ?: return null
        val lib = libs.find { it.id == libraryId } ?: return null
        return libraryRailType(lib)
    }
}
