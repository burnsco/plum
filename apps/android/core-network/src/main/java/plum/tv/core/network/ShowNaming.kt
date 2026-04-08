package plum.tv.core.network

private val showTitlePrefixRegex = Regex("^(.+?)\\s*-\\s*S\\d+", RegexOption.IGNORE_CASE)
private val nonAlphaNumericRegex = Regex("[^a-z0-9]+")

/** Extract display show name from episode title (e.g. "Show Name - S01E05 - …" → "Show Name"). */
fun getShowName(title: String): String {
    val match = showTitlePrefixRegex.find(title)
    return match?.groupValues?.getOrNull(1)?.trim() ?: title
}

fun normalizeShowKeyTitle(title: String): String =
    getShowName(title).lowercase().replace(nonAlphaNumericRegex, "")
