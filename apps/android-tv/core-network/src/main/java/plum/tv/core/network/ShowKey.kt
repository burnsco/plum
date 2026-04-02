package plum.tv.core.network

private val titleShowPrefix = Regex("^(.+?)\\s*-\\s*S\\d+", RegexOption.IGNORE_CASE)

private fun normalizeShowKeyTitle(title: String): String {
    val base = titleShowPrefix.find(title)?.groupValues?.get(1)?.trim() ?: title
    return base.lowercase().replace(Regex("[^a-z0-9]+"), "")
}

/** Mirrors web `getShowKey` for a single browse item. */
fun showKeyForBrowseItem(item: LibraryBrowseItemJson): String {
    if (item.type != "tv" && item.type != "anime") return ""
    val tid = item.tmdbId
    if (tid != null && tid > 0) {
        return "tmdb-$tid"
    }
    return "title-${normalizeShowKeyTitle(item.title)}"
}
