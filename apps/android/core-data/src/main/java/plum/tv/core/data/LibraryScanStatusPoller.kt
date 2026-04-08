package plum.tv.core.data

import android.util.Log
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import plum.tv.core.network.LibraryScanStatusJson

/**
 * Polls [GET /api/libraries/{id}/scan] like the web [ScanQueueProvider] does when a scan is active,
 * feeding [LibraryCatalogRefreshCoordinator] so Android TV does not rely solely on WebSocket
 * `library_scan_update` (which can be dropped across proxies or during reconnect).
 */
@Singleton
class LibraryScanStatusPoller @Inject constructor(
    private val sessionRepository: SessionRepository,
    private val catalogRefreshCoordinator: LibraryCatalogRefreshCoordinator,
) {
    private companion object {
        const val TAG = "PlumTV"
        const val ACTIVE_POLL_MS = 2_000L
        const val IDLE_POLL_MS = 30_000L
        const val NO_AUTH_WAIT_MS = 3_000L
        const val ERROR_BACKOFF_MS = 5_000L
    }

    private var job: Job? = null

    fun start(scope: CoroutineScope) {
        job?.cancel()
        job =
            scope.launch(Dispatchers.IO) {
                while (isActive) {
                    try {
                        if (sessionRepository.sessionToken.first().isNullOrBlank()) {
                            delay(NO_AUTH_WAIT_MS)
                            continue
                        }
                        val api = sessionRepository.getPlumApi()
                        val libsRes = api.libraries()
                        if (!libsRes.isSuccessful) {
                            delay(ERROR_BACKOFF_MS)
                            continue
                        }
                        val libraryIds = libsRes.body()?.map { it.id }.orEmpty()
                        var anyActive = false
                        for (libraryId in libraryIds) {
                            val scanRes = api.libraryScanStatus(libraryId)
                            val body = scanRes.body()
                            if (scanRes.isSuccessful && body != null) {
                                catalogRefreshCoordinator.applyScanStatusFromRest(body)
                                if (isScanActivelyProcessing(body)) {
                                    anyActive = true
                                }
                            }
                        }
                        delay(if (anyActive) ACTIVE_POLL_MS else IDLE_POLL_MS)
                    } catch (e: CancellationException) {
                        throw e
                    } catch (e: IllegalStateException) {
                        Log.d(TAG, "scan poll waiting for server: ${e.message}")
                        delay(NO_AUTH_WAIT_MS)
                    } catch (e: Exception) {
                        Log.w(TAG, "scan poll error: ${e.message}", e)
                        delay(ERROR_BACKOFF_MS)
                    }
                }
            }
    }

    fun stop() {
        job?.cancel()
        job = null
    }
}

private fun isScanActivelyProcessing(scan: LibraryScanStatusJson): Boolean {
    val enrichment =
        when {
            scan.enrichmentPhase == "queued" || scan.enrichmentPhase == "running" -> checkNotNull(scan.enrichmentPhase)
            scan.enriching -> "running"
            else -> "idle"
        }
    return scan.phase == "queued" ||
        scan.phase == "scanning" ||
        enrichment == "queued" ||
        enrichment == "running" ||
        scan.identifyPhase == "queued" ||
        scan.identifyPhase == "identifying"
}
