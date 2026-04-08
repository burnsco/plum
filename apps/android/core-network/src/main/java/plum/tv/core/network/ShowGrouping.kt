package plum.tv.core.network

/**
 * Builds a map from normalized episode title → `tmdb-…` key for episodes that have TMDb id,
 * so unmatched episodes with the same title normalize can merge into the same show (web parity).
 */
fun tmdbTitleBridgeMap(items: Iterable<LibraryBrowseItemJson>): Map<String, String> {
    val out = mutableMapOf<String, String>()
    for (item in items) {
        val tid = item.tmdbId
        if (tid == null || tid <= 0) continue
        if (item.type != "tv" && item.type != "anime") continue
        out[normalizeShowKeyTitle(item.title)] = "tmdb-$tid"
    }
    return out
}

fun resolvedShowKeyForBrowseItem(
    item: LibraryBrowseItemJson,
    titleBridge: Map<String, String>,
): String {
    if (item.type != "tv" && item.type != "anime") return ""
    val tid = item.tmdbId
    if (tid != null && tid > 0) {
        return "tmdb-$tid"
    }
    val norm = normalizeShowKeyTitle(item.title)
    return titleBridge[norm] ?: "title-$norm"
}

private fun compareEpisodes(a: LibraryBrowseItemJson, b: LibraryBrowseItemJson): Int {
    val sa = a.season ?: 0
    val sb = b.season ?: 0
    if (sa != sb) return sa.compareTo(sb)
    val ea = a.episode ?: 0
    val eb = b.episode ?: 0
    if (ea != eb) return ea.compareTo(eb)
    return a.title.compareTo(b.title)
}

data class LibraryShowBrowseRow(
    val showKey: String,
    val displayTitle: String,
    /** Episode used for poster / library_id (show-level artwork). */
    val posterItem: LibraryBrowseItemJson,
    val episodes: List<LibraryBrowseItemJson>,
)

/**
 * One row per series, order = first time an episode for that show appears in [items] order.
 */
fun groupLibraryBrowseItemsByShow(items: List<LibraryBrowseItemJson>): List<LibraryShowBrowseRow> {
    if (items.isEmpty()) return emptyList()
    val titleBridge = tmdbTitleBridgeMap(items)
    val keyOrder = mutableListOf<String>()
    val buckets = mutableMapOf<String, MutableList<LibraryBrowseItemJson>>()
    for (item in items) {
        if (item.type != "tv" && item.type != "anime") continue
        val key = resolvedShowKeyForBrowseItem(item, titleBridge)
        if (key.isEmpty()) continue
        if (!buckets.containsKey(key)) {
            keyOrder.add(key)
            buckets[key] = mutableListOf()
        }
        buckets[key]!!.add(item)
    }
    return keyOrder.map { key ->
        val eps = buckets[key]!!.sortedWith(::compareEpisodes)
        val first = eps.first()
        val posterEp =
            eps.firstOrNull { ep ->
                !ep.showPosterPath.isNullOrBlank() ||
                    !ep.showPosterUrl.isNullOrBlank() ||
                    !ep.posterPath.isNullOrBlank() ||
                    !ep.posterUrl.isNullOrBlank()
            } ?: first
        LibraryShowBrowseRow(
            showKey = key,
            displayTitle = getShowName(first.title),
            posterItem = posterEp,
            episodes = eps,
        )
    }
}

/** True when every item is a TV or anime episode (typical TV/anime libraries). */
fun isShowOnlyBrowseLibrary(items: List<LibraryBrowseItemJson>): Boolean =
    items.isNotEmpty() && items.all { it.type == "tv" || it.type == "anime" }
