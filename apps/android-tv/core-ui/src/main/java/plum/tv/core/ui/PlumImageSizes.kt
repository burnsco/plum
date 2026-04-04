package plum.tv.core.ui

/** TMDb `image.tmdb.org/t/p/{size}` tiers for 10-foot UI (decode larger, scale in Coil). */
object PlumImageSizes {
    /** Large enough for ~170dp-wide TV posters at 2× density without mushy scaling. */
    const val POSTER_GRID = "w500"
    const val POSTER_DETAIL = "w500"
    /** Wide hero / player backdrops on 10-foot UI (4K-safe; URLs rewritten from server `w500` assets). */
    const val BACKDROP_HERO = "w1920"
    const val BACKDROP_DETAIL = "w780"
    const val THUMB_SMALL = "w185"
}
