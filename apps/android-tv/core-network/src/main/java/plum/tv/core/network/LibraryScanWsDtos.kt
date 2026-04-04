package plum.tv.core.network

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

/** Payload shape for `type: "library_scan_update"` WebSocket messages (matches server `libraryScanStatus`). */
@JsonClass(generateAdapter = true)
data class LibraryScanStatusJson(
    @Json(name = "libraryId") val libraryId: Int,
    @Json(name = "phase") val phase: String,
    @Json(name = "enrichmentPhase") val enrichmentPhase: String? = null,
    @Json(name = "enriching") val enriching: Boolean = false,
    @Json(name = "identifyPhase") val identifyPhase: String? = null,
    @Json(name = "identified") val identified: Int = 0,
    @Json(name = "identifyFailed") val identifyFailed: Int = 0,
    @Json(name = "processed") val processed: Int = 0,
    @Json(name = "added") val added: Int = 0,
    @Json(name = "updated") val updated: Int = 0,
    @Json(name = "removed") val removed: Int = 0,
    @Json(name = "unmatched") val unmatched: Int = 0,
    @Json(name = "skipped") val skipped: Int = 0,
    @Json(name = "identifyRequested") val identifyRequested: Boolean = false,
    @Json(name = "error") val error: String? = null,
    @Json(name = "lastError") val lastError: String? = null,
    @Json(name = "finishedAt") val finishedAt: String? = null,
)

@JsonClass(generateAdapter = true)
data class LibraryScanUpdateWsEventJson(
    @Json(name = "type") val type: String,
    @Json(name = "scan") val scan: LibraryScanStatusJson,
)
