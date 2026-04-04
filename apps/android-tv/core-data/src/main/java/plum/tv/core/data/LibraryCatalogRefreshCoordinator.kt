package plum.tv.core.data

import android.util.Log
import com.squareup.moshi.JsonAdapter
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.channels.BufferOverflow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import plum.tv.core.network.LibraryScanStatusJson
import plum.tv.core.network.LibraryScanUpdateWsEventJson

/**
 * Handles `library_scan_update` WebSocket payloads (same event the web app uses in ScanQueueProvider):
 * drops stale [BrowseRepository] pages and signals ViewModels to refetch so media IDs/paths match the server
 * after library rescans or filesystem renames.
 */
@Singleton
class LibraryCatalogRefreshCoordinator @Inject constructor(
    private val browseRepository: BrowseRepository,
) {
    private companion object {
        const val TAG = "PlumTV"
    }

    private val lock = Any()
    private val lastByLibrary = mutableMapOf<Int, ScanStatusSnapshot>()

    private val _events =
        MutableSharedFlow<CatalogRefreshEvent>(
            extraBufferCapacity = 32,
            onBufferOverflow = BufferOverflow.DROP_OLDEST,
        )
    val catalogRefreshEvents: SharedFlow<CatalogRefreshEvent> = _events.asSharedFlow()

    /** Clears deduplication memory (e.g. after logout). */
    fun resetScanState() {
        synchronized(lock) {
            lastByLibrary.clear()
        }
    }

    /**
     * Applies the same logic as [handleWebSocketText] for [GET /api/libraries/{id}/scan] poll results
     * so catalog invalidation still runs when WebSocket updates are missed (TV networks / reconnects).
     */
    fun applyScanStatusFromRest(scan: LibraryScanStatusJson) {
        onLibraryScanStatus(scan.toSnapshot())
    }

    /**
     * @return true if this was a handled library scan message (playback parsers should skip).
     */
    fun handleWebSocketText(adapter: JsonAdapter<LibraryScanUpdateWsEventJson>, text: String): Boolean {
        val parsed = runCatching { adapter.fromJson(text) }.getOrNull() ?: return false
        if (parsed.type != "library_scan_update") return false
        onLibraryScanStatus(parsed.scan.toSnapshot())
        return true
    }

    private fun onLibraryScanStatus(next: ScanStatusSnapshot) {
        val previous =
            synchronized(lock) {
                val p = lastByLibrary[next.libraryId]
                lastByLibrary[next.libraryId] = next
                p
            }
        if (!hasMeaningfulStatusChange(previous, next)) return
        if (!isLibraryProcessing(next) && next.phase != "completed" && next.phase != "failed") return

        browseRepository.invalidateLibrariesCache()
        val invalidateDiscover = next.phase == "completed" || next.phase == "failed"
        if (!_events.tryEmit(CatalogRefreshEvent(libraryId = next.libraryId, invalidateDiscover = invalidateDiscover))) {
            Log.w(TAG, "catalog refresh event dropped (buffer full) libraryId=${next.libraryId}")
        }
    }
}

data class CatalogRefreshEvent(
    val libraryId: Int,
    val invalidateDiscover: Boolean,
)

/** Minimal copy for comparisons; ignores activity and other fields the web client does not use for invalidation. */
private data class ScanStatusSnapshot(
    val libraryId: Int,
    val phase: String,
    val enrichmentPhase: String?,
    val enriching: Boolean,
    val identifyPhase: String?,
    val identified: Int,
    val identifyFailed: Int,
    val processed: Int,
    val added: Int,
    val updated: Int,
    val removed: Int,
    val unmatched: Int,
    val skipped: Int,
    val identifyRequested: Boolean,
    val error: String?,
    val lastError: String?,
    val finishedAt: String?,
)

private fun plum.tv.core.network.LibraryScanStatusJson.toSnapshot(): ScanStatusSnapshot =
    ScanStatusSnapshot(
        libraryId = libraryId,
        phase = phase,
        enrichmentPhase = enrichmentPhase,
        enriching = enriching,
        identifyPhase = identifyPhase,
        identified = identified,
        identifyFailed = identifyFailed,
        processed = processed,
        added = added,
        updated = updated,
        removed = removed,
        unmatched = unmatched,
        skipped = skipped,
        identifyRequested = identifyRequested,
        error = error,
        lastError = lastError,
        finishedAt = finishedAt,
    )

private fun enrichmentPhaseActive(status: ScanStatusSnapshot): String {
    if (status.enrichmentPhase == "queued" || status.enrichmentPhase == "running") {
        return checkNotNull(status.enrichmentPhase)
    }
    return if (status.enriching) "running" else "idle"
}

private fun isLibraryProcessing(status: ScanStatusSnapshot): Boolean {
    val enrichment = enrichmentPhaseActive(status)
    return status.phase == "queued" ||
        status.phase == "scanning" ||
        enrichment == "queued" ||
        enrichment == "running" ||
        status.identifyPhase == "queued" ||
        status.identifyPhase == "identifying"
}

private fun hasMeaningfulStatusChange(previous: ScanStatusSnapshot?, next: ScanStatusSnapshot): Boolean {
    if (previous == null) return true
    return previous.phase != next.phase ||
        previous.enrichmentPhase != next.enrichmentPhase ||
        previous.enriching != next.enriching ||
        previous.identifyPhase != next.identifyPhase ||
        previous.identified != next.identified ||
        previous.identifyFailed != next.identifyFailed ||
        previous.processed != next.processed ||
        previous.added != next.added ||
        previous.updated != next.updated ||
        previous.removed != next.removed ||
        previous.unmatched != next.unmatched ||
        previous.skipped != next.skipped ||
        previous.identifyRequested != next.identifyRequested ||
        previous.error != next.error ||
        previous.lastError != next.lastError ||
        previous.finishedAt != next.finishedAt
}
