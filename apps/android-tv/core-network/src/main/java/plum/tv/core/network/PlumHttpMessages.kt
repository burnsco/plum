package plum.tv.core.network

/**
 * User-visible strings for failed Plum API calls from Retrofit. Shapes match the `errorMessage`
 * callbacks in `createPlumApiClient` (`packages/shared/src/api.ts`) so TV and web show the same
 * fallbacks when the response body is empty.
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
}
