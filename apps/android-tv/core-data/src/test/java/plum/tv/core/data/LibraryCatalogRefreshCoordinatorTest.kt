package plum.tv.core.data

import com.squareup.moshi.Moshi
import com.squareup.moshi.kotlin.reflect.KotlinJsonAdapterFactory
import io.mockk.mockk
import io.mockk.verify
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import plum.tv.core.network.LibraryCatalogChangedWsEventJson
import plum.tv.core.network.LibraryScanStatusJson
import plum.tv.core.network.LibraryScanUpdateWsEventJson

class LibraryCatalogRefreshCoordinatorTest {

    private val moshi =
        Moshi.Builder()
            .addLast(KotlinJsonAdapterFactory())
            .build()

    private val scanAdapter = moshi.adapter(LibraryScanUpdateWsEventJson::class.java)
    private val catalogAdapter = moshi.adapter(LibraryCatalogChangedWsEventJson::class.java)

    @Test
    fun handleWebSocketText_invalidJson_returnsFalse() {
        val browse = mockk<BrowseRepository>(relaxed = true)
        val coordinator = LibraryCatalogRefreshCoordinator(browse)
        assertFalse(coordinator.handleWebSocketText(scanAdapter, catalogAdapter, "not json"))
        verify(exactly = 0) { browse.invalidateLibrariesCache() }
    }

    @Test
    fun scanningStatus_invalidatesOncePerMeaningfulChange() {
        val browse = mockk<BrowseRepository>(relaxed = true)
        val coordinator = LibraryCatalogRefreshCoordinator(browse)
        val json = """{"type":"library_scan_update","scan":{"libraryId":1,"phase":"scanning"}}"""
        assertTrue(coordinator.handleWebSocketText(scanAdapter, catalogAdapter, json))
        verify(exactly = 1) { browse.invalidateLibrariesCache() }
        assertTrue(coordinator.handleWebSocketText(scanAdapter, catalogAdapter, json))
        verify(exactly = 1) { browse.invalidateLibrariesCache() }
    }

    @Test
    fun idleStatus_doesNotInvalidate() {
        val browse = mockk<BrowseRepository>(relaxed = true)
        val coordinator = LibraryCatalogRefreshCoordinator(browse)
        val json = """{"type":"library_scan_update","scan":{"libraryId":1,"phase":"idle"}}"""
        assertTrue(coordinator.handleWebSocketText(scanAdapter, catalogAdapter, json))
        verify(exactly = 0) { browse.invalidateLibrariesCache() }
    }

    @Test
    fun catalogChanged_invalidatesCache() {
        val browse = mockk<BrowseRepository>(relaxed = true)
        val coordinator = LibraryCatalogRefreshCoordinator(browse)
        val json = """{"type":"library_catalog_changed","libraryId":3}"""
        assertTrue(coordinator.handleWebSocketText(scanAdapter, catalogAdapter, json))
        verify(exactly = 1) { browse.invalidateLibrariesCache() }
    }

    @Test
    fun applyScanStatusFromRest_matchesWebSocketScanningPath() {
        val browse = mockk<BrowseRepository>(relaxed = true)
        val coordinator = LibraryCatalogRefreshCoordinator(browse)
        coordinator.applyScanStatusFromRest(LibraryScanStatusJson(libraryId = 1, phase = "scanning"))
        verify(exactly = 1) { browse.invalidateLibrariesCache() }
        coordinator.applyScanStatusFromRest(LibraryScanStatusJson(libraryId = 1, phase = "scanning"))
        verify(exactly = 1) { browse.invalidateLibrariesCache() }
    }
}
