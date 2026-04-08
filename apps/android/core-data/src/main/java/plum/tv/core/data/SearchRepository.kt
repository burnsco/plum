package plum.tv.core.data

import javax.inject.Inject
import javax.inject.Singleton
import plum.tv.core.network.PlumHttpMessages
import plum.tv.core.network.SearchResponseJson

@Singleton
class SearchRepository @Inject constructor(
    private val sessionRepository: SessionRepository,
) {
    suspend fun searchLibraryMedia(
        query: String,
        libraryId: Int? = null,
        type: String? = null,
        genre: String? = null,
        limit: Int = 30,
    ): Result<SearchResponseJson> = runCatching {
        val api = sessionRepository.getPlumApi()
        val res = api.searchLibraryMedia(
            q = query,
            libraryId = libraryId,
            limit = limit,
            mediaType = type,
            genre = genre,
        )
        if (!res.isSuccessful) {
            error(PlumHttpMessages.preferBody("Search", res.code(), res.errorBody()?.string()))
        }
        res.body() ?: error("Empty search response")
    }
}

