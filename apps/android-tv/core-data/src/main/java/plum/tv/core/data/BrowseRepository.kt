package plum.tv.core.data

import java.net.URLEncoder
import java.nio.charset.StandardCharsets
import javax.inject.Inject
import javax.inject.Singleton
import plum.tv.core.network.HomeDashboardJson
import plum.tv.core.network.LibraryJson
import plum.tv.core.network.LibraryMediaPageJson
import plum.tv.core.network.LibraryMovieDetailsJson
import plum.tv.core.network.LibraryShowDetailsJson
import plum.tv.core.network.ShowEpisodesResponseJson

@Singleton
class BrowseRepository @Inject constructor(
    private val sessionRepository: SessionRepository,
) {
    private val librariesCacheLock = Any()
    @Volatile
    private var cachedLibraries: List<LibraryJson>? = null

    fun invalidateLibrariesCache() {
        synchronized(librariesCacheLock) {
            cachedLibraries = null
        }
    }
    suspend fun homeDashboard(): Result<HomeDashboardJson> = runCatching {
        val res = sessionRepository.getPlumApi().homeDashboard()
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "Home: HTTP ${res.code()}")
        }
        res.body() ?: error("Empty home response")
    }

    suspend fun libraries(forceRefresh: Boolean = false): Result<List<LibraryJson>> {
        if (!forceRefresh) {
            synchronized(librariesCacheLock) {
                cachedLibraries?.let { return Result.success(it) }
            }
        }
        return runCatching {
            val res = sessionRepository.getPlumApi().libraries()
            if (!res.isSuccessful) {
                error(res.errorBody()?.string() ?: "Libraries: HTTP ${res.code()}")
            }
            val body = res.body() ?: emptyList()
            synchronized(librariesCacheLock) {
                cachedLibraries = body
            }
            body
        }
    }

    suspend fun libraryMedia(libraryId: Int, offset: Int? = null, limit: Int? = null): Result<LibraryMediaPageJson> =
        runCatching {
            val res = sessionRepository.getPlumApi().libraryMedia(libraryId, offset, limit)
            if (!res.isSuccessful) {
                error(res.errorBody()?.string() ?: "Library media: HTTP ${res.code()}")
            }
            res.body() ?: error("Empty library media")
        }

    suspend fun movieDetails(libraryId: Int, mediaId: Int): Result<LibraryMovieDetailsJson> = runCatching {
        val res = sessionRepository.getPlumApi().movieDetails(libraryId, mediaId)
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "Movie details: HTTP ${res.code()}")
        }
        res.body() ?: error("Empty movie details")
    }

    suspend fun showDetails(libraryId: Int, showKey: String): Result<LibraryShowDetailsJson> = runCatching {
        val enc = encodeShowKey(showKey)
        val res = sessionRepository.getPlumApi().showDetails(libraryId, enc)
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "Show details: HTTP ${res.code()}")
        }
        res.body() ?: error("Empty show details")
    }

    suspend fun showEpisodes(libraryId: Int, showKey: String): Result<ShowEpisodesResponseJson> = runCatching {
        val enc = encodeShowKey(showKey)
        val res = sessionRepository.getPlumApi().showEpisodes(libraryId, enc)
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "Show episodes: HTTP ${res.code()}")
        }
        res.body() ?: error("Empty show episodes")
    }

    private fun encodeShowKey(showKey: String): String =
        URLEncoder.encode(showKey, StandardCharsets.UTF_8.toString())
            .replace("+", "%20")
}
