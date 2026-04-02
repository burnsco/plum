package plum.tv.core.data

import javax.inject.Inject
import javax.inject.Singleton
import plum.tv.core.network.DiscoverBrowseResponseJson
import plum.tv.core.network.DiscoverGenresResponseJson
import plum.tv.core.network.DiscoverResponseJson
import plum.tv.core.network.DiscoverSearchResponseJson
import plum.tv.core.network.DiscoverTitleDetailsJson
import plum.tv.core.network.DownloadsResponseJson

@Singleton
class DiscoverRepository @Inject constructor(
    private val sessionRepository: SessionRepository,
) {
    suspend fun discover(): Result<DiscoverResponseJson> = runCatching {
        val res = sessionRepository.getPlumApi().discover()
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "Discover: HTTP ${res.code()}")
        }
        res.body() ?: error("Empty discover response")
    }

    suspend fun discoverGenres(): Result<DiscoverGenresResponseJson> = runCatching {
        val res = sessionRepository.getPlumApi().discoverGenres()
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "Discover genres: HTTP ${res.code()}")
        }
        res.body() ?: error("Empty discover genres response")
    }

    suspend fun browseDiscover(
        category: String? = null,
        mediaType: String? = null,
        genreId: Int? = null,
        page: Int? = null,
    ): Result<DiscoverBrowseResponseJson> = runCatching {
        val res = sessionRepository.getPlumApi().browseDiscover(category, mediaType, genreId, page)
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "Discover browse: HTTP ${res.code()}")
        }
        res.body() ?: error("Empty discover browse response")
    }

    suspend fun searchDiscover(query: String): Result<DiscoverSearchResponseJson> = runCatching {
        val res = sessionRepository.getPlumApi().searchDiscover(query)
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "Discover search: HTTP ${res.code()}")
        }
        res.body() ?: error("Empty discover search response")
    }

    suspend fun discoverTitleDetails(mediaType: String, tmdbId: Int): Result<DiscoverTitleDetailsJson?> = runCatching {
        val res = sessionRepository.getPlumApi().discoverTitleDetails(mediaType, tmdbId)
        if (res.code() == 404) return@runCatching null
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "Discover title: HTTP ${res.code()}")
        }
        res.body()
    }

    suspend fun addDiscoverTitle(mediaType: String, tmdbId: Int): Result<Unit> = runCatching {
        val res = sessionRepository.getPlumApi().addDiscoverTitle(mediaType, tmdbId)
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "Add discover title: HTTP ${res.code()}")
        }
    }

    suspend fun downloads(): Result<DownloadsResponseJson> = runCatching {
        val res = sessionRepository.getPlumApi().downloads()
        if (!res.isSuccessful) {
            error(res.errorBody()?.string() ?: "Downloads: HTTP ${res.code()}")
        }
        res.body() ?: error("Empty downloads response")
    }
}
