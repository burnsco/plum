package plum.tv.core.data

import com.squareup.moshi.Moshi
import com.squareup.moshi.kotlin.reflect.KotlinJsonAdapterFactory
import io.mockk.mockk
import io.mockk.verify
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import plum.tv.core.network.LibraryScanUpdateWsEventJson

class LibraryCatalogRefreshCoordinatorTest {

    private val moshi =
        Moshi.Builder()
            .addLast(KotlinJsonAdapterFactory())
            .build()

    private val adapter = moshi.adapter(LibraryScanUpdateWsEventJson::class.java)

    @Test
    fun handleWebSocketText_invalidJson_returnsFalse() {
        val browse = mockk<BrowseRepository>(relaxed = true)
        val coordinator = LibraryCatalogRefreshCoordinator(browse)
        assertFalse(coordinator.handleWebSocketText(adapter, "not json"))
        verify(exactly = 0) { browse.invalidateLibrariesCache() }
    }

    @Test
    fun scanningStatus_invalidatesOncePerMeaningfulChange() {
        val browse = mockk<BrowseRepository>(relaxed = true)
        val coordinator = LibraryCatalogRefreshCoordinator(browse)
        val json = """{"type":"library_scan_update","scan":{"libraryId":1,"phase":"scanning"}}"""
        assertTrue(coordinator.handleWebSocketText(adapter, json))
        verify(exactly = 1) { browse.invalidateLibrariesCache() }
        assertTrue(coordinator.handleWebSocketText(adapter, json))
        verify(exactly = 1) { browse.invalidateLibrariesCache() }
    }

    @Test
    fun idleStatus_doesNotInvalidate() {
        val browse = mockk<BrowseRepository>(relaxed = true)
        val coordinator = LibraryCatalogRefreshCoordinator(browse)
        val json = """{"type":"library_scan_update","scan":{"libraryId":1,"phase":"idle"}}"""
        assertTrue(coordinator.handleWebSocketText(adapter, json))
        verify(exactly = 0) { browse.invalidateLibrariesCache() }
    }
}
