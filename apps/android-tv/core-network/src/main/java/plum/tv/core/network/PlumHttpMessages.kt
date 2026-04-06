package plum.tv.core.network

import org.json.JSONException
import org.json.JSONObject

/**
 * User-visible strings for failed Plum API calls from Retrofit. Shapes match the `errorMessage`
 * callbacks in `createPlumApiClient` (`packages/shared/src/api.ts`) so TV and web show the same
 * fallbacks when the response body is empty. Playback and other structured handlers use
 * `httputil.PlumJSONError` (`{"error":"machine_key","message":"human"}`) — prefer `message` for UI.
 */
object PlumHttpMessages {
    /** Most endpoints: prefer response body, else "Label: status". */
    fun preferBody(label: String, httpCode: Int, errorBody: String?): String {
        val t = errorBody?.trim().orEmpty()
        return if (t.isNotEmpty()) t else "$label: $httpCode"
    }

    /** Libraries, home, and library media: "Label: status" with optional appended body text. */
    fun statusWithAppendedBody(label: String, httpCode: Int, errorBody: String?): String {
        val t = errorBody?.trim().orEmpty()
        return if (t.isNotEmpty()) "$label: $httpCode $t" else "$label: $httpCode"
    }

    /** Device login: prefer body, else "Invalid credentials." */
    fun deviceLoginFailed(errorBody: String?): String {
        val t = errorBody?.trim().orEmpty()
        return if (t.isNotEmpty()) t else "Invalid credentials."
    }

    /**
     * Prefer a structured JSON `message` (or human-looking `details` / `error`) when present; otherwise
     * stable user-facing text by status so plain `http.Error` bodies are not shown verbatim for busy/server faults.
     */
    fun userFacingHttpError(label: String, httpCode: Int, errorBody: String?): String {
        val trimmed = errorBody?.trim().orEmpty()
        val fromJson =
            if (trimmed.startsWith("{")) {
                runCatching { parseJsonErrorHuman(trimmed) }.getOrNull()
            } else {
                null
            }
        if (!fromJson.isNullOrBlank()) return fromJson

        return when (httpCode) {
            429 -> "The server is busy. Try again in a moment."
            503 -> "The server is temporarily unavailable. Try again."
            in 500..599 -> "Something went wrong. Try again."
            401 -> "Sign in again to continue."
            403 -> "You don't have permission for this."
            404 -> "Not found."
            else -> preferBody(label, httpCode, errorBody)
        }
    }

    private fun parseJsonErrorHuman(json: String): String? {
        val o =
            try {
                JSONObject(json)
            } catch (_: JSONException) {
                return null
            }
        val message = o.optString("message").trim().takeIf { it.isNotEmpty() }
        if (message != null) return message
        val details = o.optString("details").trim().takeIf { it.isNotEmpty() }
            ?: o.optString("detail").trim().takeIf { it.isNotEmpty() }
        if (details != null) return details
        val err = o.optString("error").trim().takeIf { it.isNotEmpty() } ?: return null
        return if (err.matches(MACHINE_ERROR_KEY)) null else err
    }

    private val MACHINE_ERROR_KEY = Regex("^[a-z][a-z0-9_]*$")
}
