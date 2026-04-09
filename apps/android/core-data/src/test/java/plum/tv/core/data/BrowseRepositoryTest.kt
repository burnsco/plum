package plum.tv.core.data

import io.mockk.coEvery
import io.mockk.mockk
import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import plum.tv.core.network.LibraryMediaPageJson
import plum.tv.core.network.LibraryBrowseItemJson
import plum.tv.core.network.PlumApi
import retrofit2.Response

class BrowseRepositoryTest {

    @Test
    fun peekContiguousLibraryMediaPages_returnsEveryCachedPageInOrder() = runBlocking {
        val api = mockk<PlumApi>()
        coEvery { api.libraryMedia(1, offset = 0, limit = 60) } returns Response.success(page(1, 0, 60, 60))
        coEvery { api.libraryMedia(1, offset = 60, limit = 60) } returns Response.success(page(2, 60, 60, null))

        val sessionRepository = mockk<SessionRepository>()
        coEvery { sessionRepository.getPlumApi() } returns api

        val repository = BrowseRepository(sessionRepository, mockk(relaxed = true))

        repository.libraryMedia(1, offset = 0, limit = 60)
        repository.libraryMedia(1, offset = 60, limit = 60)

        val cached = repository.peekContiguousLibraryMediaPages(1, 60)

        assertEquals(2, cached.size)
        assertEquals(listOf(1), cached[0].items.map { it.id })
        assertEquals(listOf(2), cached[1].items.map { it.id })
        assertTrue(cached[1].nextOffset == null)
    }

    private fun page(
        id: Int,
        offset: Int,
        limit: Int,
        nextOffset: Int?,
    ): LibraryMediaPageJson =
        LibraryMediaPageJson(
            items =
                listOf(
                    LibraryBrowseItemJson(
                        id = id,
                        libraryId = 1,
                        title = "Show $id - S01E01",
                        path = "/library/$id.mkv",
                        duration = 1800,
                        type = "anime",
                        season = 1,
                        episode = 1,
                    ),
                ),
            nextOffset = nextOffset,
            hasMore = nextOffset != null,
            total = limit + offset,
        )
}
