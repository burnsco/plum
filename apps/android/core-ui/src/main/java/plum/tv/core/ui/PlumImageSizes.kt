package plum.tv.core.ui

/** TMDb `image.tmdb.org/t/p/{size}` tiers for 10-foot UI (decode larger, scale in Coil). */
object PlumImageSizes {
    /**
     * Dense grids ([PlumTvMetrics.posterCompactWidth] ≈ 138dp): ~w342 matches 2× density + slight TV
     * focus scale without downloading w500 assets for every cell.
     */
    const val POSTER_GRID_COMPACT = "w342"
    /** Full rail / search tiles ([PlumTvMetrics.posterWidth] ≈ 170dp) at 2–3× density. */
    const val POSTER_GRID = "w500"
    const val POSTER_DETAIL = "w500"
    /** Wide hero / player backdrops on 10-foot UI (4K-safe; URLs rewritten from server `w500` assets). */
    const val BACKDROP_HERO = "w1920"
    const val BACKDROP_DETAIL = "w780"
    const val THUMB_SMALL = "w185"
}
