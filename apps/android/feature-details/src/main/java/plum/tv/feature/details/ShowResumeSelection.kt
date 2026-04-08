package plum.tv.feature.details

import plum.tv.core.network.LibraryBrowseItemJson
import plum.tv.core.network.ShowSeasonEpisodesJson

/**
 * Picks the season index and episode index within that season to open first: resume in-progress,
 * else next unwatched in air order, else the last episode when everything is watched.
 */
internal fun computeResumeSeasonAndEpisode(seasons: List<ShowSeasonEpisodesJson>): Pair<Int, Int> {
    if (seasons.isEmpty()) return 0 to 0

    data class Ref(val seasonIdx: Int, val epIdx: Int, val ep: LibraryBrowseItemJson)

    val ordered = buildList {
        seasons.forEachIndexed { sIdx, season ->
            season.episodes.forEachIndexed { eIdx, ep ->
                add(Ref(sIdx, eIdx, ep))
            }
        }
    }
    if (ordered.isEmpty()) return 0 to 0

    fun LibraryBrowseItemJson.watched(): Boolean = completed == true
    fun LibraryBrowseItemJson.hasProgress(): Boolean =
        (progressSeconds ?: 0.0) > 0.0 || (progressPercent ?: 0.0) > 0.0

    val inProgress = ordered.filter { !it.ep.watched() && it.ep.hasProgress() }
    if (inProgress.isNotEmpty()) {
        val best =
            inProgress.maxWith(
                compareBy<Ref> { it.ep.lastWatchedAt.orEmpty() }
                    .thenBy { it.seasonIdx }
                    .thenBy { it.epIdx },
            )
        return best.seasonIdx to best.epIdx
    }

    val nextUnwatched = ordered.firstOrNull { !it.ep.watched() }
    if (nextUnwatched != null) {
        return nextUnwatched.seasonIdx to nextUnwatched.epIdx
    }

    val last = ordered.last()
    return last.seasonIdx to last.epIdx
}

internal fun seasonWatchSuffix(episodes: List<LibraryBrowseItemJson>): String {
    if (episodes.isEmpty()) return ""
    val unwatched = episodes.count { it.completed != true }
    return when {
        unwatched == 0 -> " · Watched"
        unwatched == episodes.size -> " · $unwatched unwatched"
        else -> " · $unwatched left"
    }
}
