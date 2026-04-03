package plum.tv.core.network

/** Mirrors web `getShowKey` for a single browse item. */
fun showKeyForBrowseItem(item: LibraryBrowseItemJson): String {
    if (item.type != "tv" && item.type != "anime") return ""
    val tid = item.tmdbId
    if (tid != null && tid > 0) {
        return "tmdb-$tid"
    }
    return "title-${normalizeShowKeyTitle(item.title)}"
}
