package plum.tv.core.network

import com.squareup.moshi.Json

data class SubtitleJson(
    @Json(name = "id") val id: Int = 0,
    @Json(name = "title") val title: String = "",
    @Json(name = "language") val language: String = "",
    @Json(name = "format") val format: String = "",
)

data class EmbeddedSubtitleJson(
    @Json(name = "streamIndex") val streamIndex: Int = 0,
    @Json(name = "language") val language: String = "",
    @Json(name = "title") val title: String = "",
    @Json(name = "codec") val codec: String? = null,
)

data class EmbeddedAudioTrackJson(
    @Json(name = "streamIndex") val streamIndex: Int = 0,
    @Json(name = "language") val language: String = "",
    @Json(name = "title") val title: String = "",
)

/** Home dashboard and browse responses use the same core media shape as the server. */
data class MediaItemJson(
    @Json(name = "id") val id: Int,
    @Json(name = "library_id") val libraryId: Int? = null,
    @Json(name = "title") val title: String,
    @Json(name = "path") val path: String,
    @Json(name = "duration") val duration: Int,
    @Json(name = "type") val type: String,
    @Json(name = "match_status") val matchStatus: String? = null,
    @Json(name = "identify_state") val identifyState: String? = null,
    @Json(name = "subtitles") val subtitles: List<SubtitleJson> = emptyList(),
    @Json(name = "embeddedSubtitles") val embeddedSubtitles: List<EmbeddedSubtitleJson> = emptyList(),
    @Json(name = "embeddedAudioTracks") val embeddedAudioTracks: List<EmbeddedAudioTrackJson> = emptyList(),
    @Json(name = "tmdb_id") val tmdbId: Int? = null,
    @Json(name = "tvdb_id") val tvdbId: String? = null,
    @Json(name = "overview") val overview: String? = null,
    @Json(name = "poster_path") val posterPath: String? = null,
    @Json(name = "backdrop_path") val backdropPath: String? = null,
    @Json(name = "poster_url") val posterUrl: String? = null,
    @Json(name = "backdrop_url") val backdropUrl: String? = null,
    @Json(name = "show_poster_path") val showPosterPath: String? = null,
    @Json(name = "show_poster_url") val showPosterUrl: String? = null,
    @Json(name = "release_date") val releaseDate: String? = null,
    @Json(name = "show_vote_average") val showVoteAverage: Double? = null,
    @Json(name = "vote_average") val voteAverage: Double? = null,
    @Json(name = "imdb_id") val imdbId: String? = null,
    @Json(name = "imdb_rating") val imdbRating: Double? = null,
    @Json(name = "progress_seconds") val progressSeconds: Double? = null,
    @Json(name = "progress_percent") val progressPercent: Double? = null,
    @Json(name = "remaining_seconds") val remainingSeconds: Double? = null,
    @Json(name = "completed") val completed: Boolean? = null,
    @Json(name = "last_watched_at") val lastWatchedAt: String? = null,
    @Json(name = "season") val season: Int? = null,
    @Json(name = "episode") val episode: Int? = null,
    @Json(name = "metadata_review_needed") val metadataReviewNeeded: Boolean? = null,
    @Json(name = "metadata_confirmed") val metadataConfirmed: Boolean? = null,
    @Json(name = "thumbnail_path") val thumbnailPath: String? = null,
    @Json(name = "thumbnail_url") val thumbnailUrl: String? = null,
    @Json(name = "missing") val missing: Boolean? = null,
    @Json(name = "missing_since") val missingSince: String? = null,
)

data class ContinueWatchingEntryJson(
    @Json(name = "kind") val kind: String,
    @Json(name = "media") val media: MediaItemJson,
    @Json(name = "show_key") val showKey: String? = null,
    @Json(name = "show_title") val showTitle: String? = null,
    @Json(name = "episode_label") val episodeLabel: String? = null,
    @Json(name = "remaining_seconds") val remainingSeconds: Double,
)

data class RecentlyAddedEntryJson(
    @Json(name = "kind") val kind: String,
    @Json(name = "media") val media: MediaItemJson,
    @Json(name = "show_key") val showKey: String? = null,
    @Json(name = "show_title") val showTitle: String? = null,
    @Json(name = "episode_label") val episodeLabel: String? = null,
)

data class HomeDashboardJson(
    @Json(name = "continueWatching") val continueWatching: List<ContinueWatchingEntryJson> = emptyList(),
    @Json(name = "recentlyAdded") val recentlyAdded: List<RecentlyAddedEntryJson> = emptyList(),
)

data class LibraryJson(
    @Json(name = "id") val id: Int,
    @Json(name = "name") val name: String,
    @Json(name = "type") val type: String,
    @Json(name = "path") val path: String,
    @Json(name = "user_id") val userId: Int,
)

data class LibraryBrowseItemJson(
    @Json(name = "id") val id: Int,
    @Json(name = "library_id") val libraryId: Int? = null,
    @Json(name = "title") val title: String,
    @Json(name = "path") val path: String,
    @Json(name = "duration") val duration: Int,
    @Json(name = "type") val type: String,
    @Json(name = "match_status") val matchStatus: String? = null,
    @Json(name = "identify_state") val identifyState: String? = null,
    @Json(name = "tmdb_id") val tmdbId: Int? = null,
    @Json(name = "tvdb_id") val tvdbId: String? = null,
    @Json(name = "overview") val overview: String? = null,
    @Json(name = "poster_path") val posterPath: String? = null,
    @Json(name = "backdrop_path") val backdropPath: String? = null,
    @Json(name = "poster_url") val posterUrl: String? = null,
    @Json(name = "backdrop_url") val backdropUrl: String? = null,
    @Json(name = "show_poster_path") val showPosterPath: String? = null,
    @Json(name = "show_poster_url") val showPosterUrl: String? = null,
    @Json(name = "release_date") val releaseDate: String? = null,
    @Json(name = "show_vote_average") val showVoteAverage: Double? = null,
    @Json(name = "vote_average") val voteAverage: Double? = null,
    @Json(name = "imdb_id") val imdbId: String? = null,
    @Json(name = "imdb_rating") val imdbRating: Double? = null,
    @Json(name = "progress_seconds") val progressSeconds: Double? = null,
    @Json(name = "progress_percent") val progressPercent: Double? = null,
    @Json(name = "remaining_seconds") val remainingSeconds: Double? = null,
    @Json(name = "completed") val completed: Boolean? = null,
    @Json(name = "last_watched_at") val lastWatchedAt: String? = null,
    @Json(name = "season") val season: Int? = null,
    @Json(name = "episode") val episode: Int? = null,
    @Json(name = "metadata_review_needed") val metadataReviewNeeded: Boolean? = null,
    @Json(name = "metadata_confirmed") val metadataConfirmed: Boolean? = null,
    @Json(name = "thumbnail_path") val thumbnailPath: String? = null,
    @Json(name = "thumbnail_url") val thumbnailUrl: String? = null,
    @Json(name = "missing") val missing: Boolean? = null,
)

data class LibraryMediaPageJson(
    @Json(name = "items") val items: List<LibraryBrowseItemJson> = emptyList(),
    @Json(name = "next_offset") val nextOffset: Int? = null,
    @Json(name = "has_more") val hasMore: Boolean = false,
    @Json(name = "total") val total: Int? = null,
)

data class TitleCastMemberJson(
    @Json(name = "name") val name: String,
    @Json(name = "character") val character: String? = null,
    @Json(name = "order") val order: Int? = null,
    @Json(name = "profile_path") val profilePath: String? = null,
)

data class LibraryMovieDetailsJson(
    @Json(name = "media_id") val mediaId: Int,
    @Json(name = "library_id") val libraryId: Int,
    @Json(name = "title") val title: String,
    @Json(name = "overview") val overview: String,
    @Json(name = "poster_path") val posterPath: String? = null,
    @Json(name = "poster_url") val posterUrl: String? = null,
    @Json(name = "backdrop_path") val backdropPath: String? = null,
    @Json(name = "backdrop_url") val backdropUrl: String? = null,
    @Json(name = "release_date") val releaseDate: String? = null,
    @Json(name = "vote_average") val voteAverage: Double? = null,
    @Json(name = "imdb_id") val imdbId: String? = null,
    @Json(name = "imdb_rating") val imdbRating: Double? = null,
    @Json(name = "runtime") val runtime: Int? = null,
    @Json(name = "subtitles") val subtitles: List<SubtitleJson> = emptyList(),
    @Json(name = "embeddedSubtitles") val embeddedSubtitles: List<EmbeddedSubtitleJson> = emptyList(),
    @Json(name = "embeddedAudioTracks") val embeddedAudioTracks: List<EmbeddedAudioTrackJson> = emptyList(),
    @Json(name = "genres") val genres: List<String> = emptyList(),
    @Json(name = "cast") val cast: List<TitleCastMemberJson> = emptyList(),
)

data class LibraryShowDetailsJson(
    @Json(name = "library_id") val libraryId: Int,
    @Json(name = "show_key") val showKey: String,
    @Json(name = "name") val name: String,
    @Json(name = "overview") val overview: String,
    @Json(name = "poster_path") val posterPath: String? = null,
    @Json(name = "poster_url") val posterUrl: String? = null,
    @Json(name = "backdrop_path") val backdropPath: String? = null,
    @Json(name = "backdrop_url") val backdropUrl: String? = null,
    @Json(name = "first_air_date") val firstAirDate: String = "",
    @Json(name = "vote_average") val voteAverage: Double? = null,
    @Json(name = "imdb_id") val imdbId: String? = null,
    @Json(name = "imdb_rating") val imdbRating: Double? = null,
    @Json(name = "runtime") val runtime: Int? = null,
    @Json(name = "number_of_seasons") val numberOfSeasons: Int = 0,
    @Json(name = "number_of_episodes") val numberOfEpisodes: Int = 0,
    @Json(name = "genres") val genres: List<String> = emptyList(),
    @Json(name = "cast") val cast: List<TitleCastMemberJson> = emptyList(),
)

data class ShowSeasonEpisodesJson(
    @Json(name = "seasonNumber") val seasonNumber: Int,
    @Json(name = "label") val label: String,
    @Json(name = "episodes") val episodes: List<LibraryBrowseItemJson> = emptyList(),
)

data class ShowEpisodesResponseJson(
    @Json(name = "seasons") val seasons: List<ShowSeasonEpisodesJson> = emptyList(),
)

data class SearchFacetValueJson(
    @Json(name = "value") val value: String,
    @Json(name = "label") val label: String,
    @Json(name = "count") val count: Int,
)

data class SearchFacetsJson(
    @Json(name = "libraries") val libraries: List<SearchFacetValueJson> = emptyList(),
    @Json(name = "types") val types: List<SearchFacetValueJson> = emptyList(),
    @Json(name = "genres") val genres: List<SearchFacetValueJson> = emptyList(),
)

data class SearchResultJson(
    @Json(name = "kind") val kind: String,
    @Json(name = "library_id") val libraryId: Int,
    @Json(name = "library_name") val libraryName: String,
    @Json(name = "library_type") val libraryType: String,
    @Json(name = "title") val title: String,
    @Json(name = "subtitle") val subtitle: String? = null,
    @Json(name = "poster_path") val posterPath: String? = null,
    @Json(name = "poster_url") val posterUrl: String? = null,
    @Json(name = "imdb_rating") val imdbRating: Double? = null,
    @Json(name = "match_reason") val matchReason: String,
    @Json(name = "matched_actor") val matchedActor: String? = null,
    @Json(name = "href") val href: String,
    @Json(name = "genres") val genres: List<String> = emptyList(),
)

data class SearchResponseJson(
    @Json(name = "query") val query: String,
    @Json(name = "results") val results: List<SearchResultJson> = emptyList(),
    @Json(name = "total") val total: Int = 0,
    @Json(name = "facets") val facets: SearchFacetsJson = SearchFacetsJson(),
)
