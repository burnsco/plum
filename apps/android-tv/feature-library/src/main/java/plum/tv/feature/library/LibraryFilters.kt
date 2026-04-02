package plum.tv.feature.library

import plum.tv.core.network.LibraryJson

/** Same rules as web `getLibraryTabLabel` / library sidebar filters. */
fun filterLibrariesByType(
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

/** Rail key: `tv`, `movie`, `anime`, `music`, or null if no sidebar bucket matches. */
fun libraryRailType(lib: LibraryJson): String? =
    when {
        lib.type == "movie" -> "movie"
        lib.type == "music" -> "music"
        lib.type == "anime" || (lib.type == "tv" && lib.name.contains("anime", ignoreCase = true)) -> "anime"
        lib.type == "tv" -> "tv"
        else -> null
    }
