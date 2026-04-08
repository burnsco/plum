package plum.tv.core.network

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

/** Payload shape for `type: "library_scan_update"` WebSocket messages (matches server `libraryScanStatus`). */
@JsonClass(generateAdapter = true)
data class LibraryScanStatusJson(
    @param:Json(name = "libraryId") val libraryId: Int,
    @param:Json(name = "phase") val phase: String,
    @param:Json(name = "enrichmentPhase") val enrichmentPhase: String? = null,
    @param:Json(name = "enriching") val enriching: Boolean = false,
    @param:Json(name = "identifyPhase") val identifyPhase: String? = null,
    @param:Json(name = "identified") val identified: Int = 0,
    @param:Json(name = "identifyFailed") val identifyFailed: Int = 0,
    @param:Json(name = "processed") val processed: Int = 0,
    @param:Json(name = "added") val added: Int = 0,
    @param:Json(name = "updated") val updated: Int = 0,
    @param:Json(name = "removed") val removed: Int = 0,
    @param:Json(name = "unmatched") val unmatched: Int = 0,
    @param:Json(name = "skipped") val skipped: Int = 0,
    @param:Json(name = "identifyRequested") val identifyRequested: Boolean = false,
    @param:Json(name = "error") val error: String? = null,
    @param:Json(name = "lastError") val lastError: String? = null,
    @param:Json(name = "finishedAt") val finishedAt: String? = null,
)

@JsonClass(generateAdapter = true)
data class LibraryScanUpdateWsEventJson(
    @param:Json(name = "type") val type: String,
    @param:Json(name = "scan") val scan: LibraryScanStatusJson,
)

@JsonClass(generateAdapter = true)
data class LibraryCatalogChangedWsEventJson(
    @param:Json(name = "type") val type: String,
    @param:Json(name = "libraryId") val libraryId: Int,
)
