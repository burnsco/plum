package plum.tv.core.data

import java.net.URLEncoder
import java.nio.charset.StandardCharsets
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.launch
import kotlinx.coroutines.supervisorScope
import plum.tv.core.network.HomeDashboardJson
import plum.tv.core.network.PlumHttpMessages
import plum.tv.core.network.LibraryJson
import plum.tv.core.network.LibraryMediaPageJson
import plum.tv.core.network.LibraryMovieDetailsJson
import plum.tv.core.network.LibraryShowDetailsJson
import plum.tv.core.network.ShowEpisodesResponseJson

private data class LibraryMediaCacheKey(val libraryId: Int, val offset: Int, val limit: Int)

@Singleton
class BrowseRepository @Inject constructor(
    private val sessionRepository: SessionRepository,
    private val homeDashboardDiskCache: HomeDashboardDiskCache,
) {
    private val librariesCacheLock = Any()
    @Volatile
    private var cachedLibraries: List<LibraryJson>? = null

    private val prefetchLock = Any()
    @Volatile
    private var prefetchInProgress = false

    private val mediaCacheLock = Any()
    private val mediaPageCache =
        object : LinkedHashMap<LibraryMediaCacheKey, LibraryMediaPageJson>(64, 0.75f, true) {
            override fun removeEldestEntry(eldest: MutableMap.MutableEntry<LibraryMediaCacheKey, LibraryMediaPageJson>?): Boolean =
                size > 60
        }

    /** Synchronous read for instant UI when [libraryMedia] was fetched earlier in the session. */
    fun peekLibraryMediaPage(libraryId: Int, offset: Int, limit: Int): LibraryMediaPageJson? {
        val key = LibraryMediaCacheKey(libraryId, offset, limit)
        synchronized(mediaCacheLock) {
            return mediaPageCache[key]
        }
    }

    /**
     * Returns cached pages for a library starting at offset 0 and continuing contiguously.
     *
     * Library browse screens can rebuild their loaded rows from this when the user leaves and
     * comes back, so we do not collapse back to just the first page if more pages were already
     * fetched in this session.
     */
    fun peekContiguousLibraryMediaPages(libraryId: Int, limit: Int): List<LibraryMediaPageJson> {
        val pages = mutableListOf<LibraryMediaPageJson>()
        var offset = 0
        synchronized(mediaCacheLock) {
            while (true) {
                val page = mediaPageCache[LibraryMediaCacheKey(libraryId, offset, limit)] ?: break
                pages += page
                val nextOffset = page.nextOffset ?: break
                if (nextOffset <= offset) break
                offset = nextOffset
            }
        }
        return pages
    }

    fun invalidateLibraryMediaCache() {
        synchronized(mediaCacheLock) {
            mediaPageCache.clear()
        }
    }

    fun invalidateLibrariesCache() {
        synchronized(librariesCacheLock) {
            cachedLibraries = null
        }
        invalidateLibraryMediaCache()
        homeDashboardDiskCache.clear()
    }
    suspend fun homeDashboard(): Result<HomeDashboardJson> = runCatching {
        val res = sessionRepository.getPlumApi().homeDashboard()
        if (!res.isSuccessful) {
            error(PlumHttpMessages.statusWithAppendedBody("Home", res.code(), res.errorBody()?.string()))
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
                error(PlumHttpMessages.statusWithAppendedBody("Libraries", res.code(), res.errorBody()?.string()))
            }
            val body = res.body() ?: emptyList()
            synchronized(librariesCacheLock) {
                cachedLibraries = body
            }
            body
        }
    }

    suspend fun libraryMedia(
        libraryId: Int,
        offset: Int? = null,
        limit: Int? = null,
        forceRefresh: Boolean = false,
    ): Result<LibraryMediaPageJson> {
        val cacheOffset = offset ?: 0
        val cacheLimit = limit ?: 0
        val cacheKey = LibraryMediaCacheKey(libraryId, cacheOffset, cacheLimit)
        if (!forceRefresh) {
            synchronized(mediaCacheLock) {
                mediaPageCache[cacheKey]?.let { return Result.success(it) }
            }
        }
        return runCatching {
            val res = sessionRepository.getPlumApi().libraryMedia(libraryId, offset, limit)
            if (!res.isSuccessful) {
                error(
                    PlumHttpMessages.statusWithAppendedBody(
                        "Library media",
                        res.code(),
                        res.errorBody()?.string(),
                    ),
                )
            }
            val body = res.body() ?: error("Empty library media")
            synchronized(mediaCacheLock) {
                mediaPageCache[cacheKey] = body
            }
            body
        }
    }

    /**
     * Warms the [libraryMedia] cache with the first page of every library so TV / Movies / Anime
     * shelves are instant on first open. Skips IDs already cached; uses limited parallelism.
     */
    suspend fun prefetchFirstLibraryMediaPages(
        firstPageLimit: Int = 60,
        maxConcurrent: Int = 3,
    ) {
        synchronized(prefetchLock) {
            if (prefetchInProgress) return
            prefetchInProgress = true
        }
        try {
            supervisorScope {
                val libs = libraries(forceRefresh = false).getOrElse { return@supervisorScope }
                if (libs.isEmpty()) return@supervisorScope
                val pendingIds =
                    libs.map { it.id }.filter { libraryId ->
                        peekLibraryMediaPage(libraryId, offset = 0, limit = firstPageLimit) == null
                    }
                if (pendingIds.isEmpty()) return@supervisorScope
                val workers = maxConcurrent.coerceAtLeast(1)
                val channel = Channel<Int>(Channel.UNLIMITED)
                val jobs =
                    List(workers) {
                        launch(Dispatchers.IO) {
                            for (libraryId in channel) {
                                runCatching {
                                    libraryMedia(libraryId, offset = 0, limit = firstPageLimit, forceRefresh = false)
                                }
                            }
                        }
                    }
                pendingIds.forEach { channel.send(it) }
                channel.close()
                jobs.forEach { it.join() }
            }
        } finally {
            synchronized(prefetchLock) {
                prefetchInProgress = false
            }
        }
    }

    suspend fun movieDetails(libraryId: Int, mediaId: Int): Result<LibraryMovieDetailsJson> = runCatching {
        val res = sessionRepository.getPlumApi().movieDetails(libraryId, mediaId)
        if (!res.isSuccessful) {
            error(PlumHttpMessages.preferBody("Movie details", res.code(), res.errorBody()?.string()))
        }
        res.body() ?: error("Empty movie details")
    }

    suspend fun showDetails(libraryId: Int, showKey: String): Result<LibraryShowDetailsJson> = runCatching {
        val enc = encodeShowKey(showKey)
        val res = sessionRepository.getPlumApi().showDetails(libraryId, enc)
        if (!res.isSuccessful) {
            error(PlumHttpMessages.preferBody("Show details", res.code(), res.errorBody()?.string()))
        }
        res.body() ?: error("Empty show details")
    }

    suspend fun showEpisodes(libraryId: Int, showKey: String): Result<ShowEpisodesResponseJson> = runCatching {
        val enc = encodeShowKey(showKey)
        val res = sessionRepository.getPlumApi().showEpisodes(libraryId, enc)
        if (!res.isSuccessful) {
            error(PlumHttpMessages.preferBody("Show episodes", res.code(), res.errorBody()?.string()))
        }
        res.body() ?: error("Empty show episodes")
    }

    private fun encodeShowKey(showKey: String): String =
        URLEncoder.encode(showKey, StandardCharsets.UTF_8.toString())
            .replace("+", "%20")
}
