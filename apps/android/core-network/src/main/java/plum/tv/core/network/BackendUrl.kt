package plum.tv.core.network

import java.net.URL

/** Resolves a Plum API path (or absolute URL) against the configured server base URL. */
fun resolveBackendUrl(baseUrl: String, pathOrUrl: String): String {
    val trimmed = pathOrUrl.trim()
    if (trimmed.startsWith("http://") || trimmed.startsWith("https://")) {
        return trimmed
    }
    val base = baseUrl.trim().trimEnd('/')
    val path = trimmed.trimStart('/')
    return "$base/$path"
}

/** Builds `ws://host/.../ws` or `wss://.../ws` from an HTTP(S) base URL. */
fun buildPlumWebSocketUrl(httpBaseUrl: String): String {
    val url = URL(httpBaseUrl.trim().trimEnd('/'))
    val wsScheme = if (url.protocol.equals("https", ignoreCase = true)) "wss" else "ws"
    val path = (url.path ?: "").trimEnd('/')
    val wsPath = if (path.isEmpty()) "/ws" else "$path/ws"
    val portPart =
        when (url.port) {
            -1 -> ""
            80 -> if (url.protocol.equals("http", ignoreCase = true)) "" else ":80"
            443 -> if (url.protocol.equals("https", ignoreCase = true)) "" else ":443"
            else -> ":${url.port}"
        }
    return "$wsScheme://${url.host}$portPart$wsPath"
}
