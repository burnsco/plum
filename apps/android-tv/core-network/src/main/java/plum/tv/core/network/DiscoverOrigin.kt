package plum.tv.core.network

/**
 * TMDB ISO 3166-1 alpha-2 origin filters (`origin_country` query).
 * Must match Go `parseDiscoverOriginCountry` / `normalizeDiscoverOrigin` and TS `normalizeDiscoverOriginKey`
 * (`packages/shared/src/discover.ts`).
 */
object DiscoverOrigin {
    fun normalizeKey(raw: String?): String {
        if (raw == null) return ""
        val s = raw.trim().uppercase()
        if (s.length != 2) return ""
        for (i in s.indices) {
            val c = s[i]
            if (c !in 'A'..'Z') return ""
        }
        return s
    }
}
