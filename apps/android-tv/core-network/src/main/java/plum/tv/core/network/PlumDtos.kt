package plum.tv.core.network

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class SubtitleJson(
    @param:Json(name = "id") val id: Int = 0,
    @param:Json(name = "title") val title: String = "",
    @param:Json(name = "language") val language: String = "",
    @param:Json(name = "format") val format: String = "",
)

@JsonClass(generateAdapter = true)
data class EmbeddedSubtitleJson(
    @param:Json(name = "streamIndex") val streamIndex: Int = 0,
    @param:Json(name = "language") val language: String = "",
    @param:Json(name = "title") val title: String = "",
    @param:Json(name = "codec") val codec: String? = null,
    /** When false, server rejects WebVTT extract; PGS may still sideload via pgsBinaryEligible and /sup. */
    @param:Json(name = "supported") val supported: Boolean? = null,
    /** Playback session: always set by server. Browse rows may omit keys — Moshi defaults apply. */
    @param:Json(name = "vttEligible") val vttEligible: Boolean = false,
    /** Playback session: always set by server. Browse rows may omit keys — Moshi defaults apply. */
    @param:Json(name = "pgsBinaryEligible") val pgsBinaryEligible: Boolean = false,
    /** Playback session: raw ASS for native ASS renderers (see contracts EmbeddedSubtitle.assEligible). */
    @param:Json(name = "assEligible") val assEligible: Boolean? = null,
)

@JsonClass(generateAdapter = true)
data class EmbeddedAudioTrackJson(
    @param:Json(name = "streamIndex") val streamIndex: Int = 0,
    @param:Json(name = "language") val language: String = "",
    @param:Json(name = "title") val title: String = "",
)

/** Home dashboard and browse responses use the same core media shape as the server. */
@JsonClass(generateAdapter = true)
data class MediaItemJson(
    @param:Json(name = "id") val id: Int,
    @param:Json(name = "library_id") val libraryId: Int? = null,
    @param:Json(name = "title") val title: String,
    @param:Json(name = "path") val path: String,
    @param:Json(name = "duration") val duration: Int,
    @param:Json(name = "type") val type: String,
    @param:Json(name = "match_status") val matchStatus: String? = null,
    @param:Json(name = "identify_state") val identifyState: String? = null,
    @param:Json(name = "subtitles") val subtitles: List<SubtitleJson> = emptyList(),
    @param:Json(name = "embeddedSubtitles") val embeddedSubtitles: List<EmbeddedSubtitleJson> = emptyList(),
    @param:Json(name = "embeddedAudioTracks") val embeddedAudioTracks: List<EmbeddedAudioTrackJson> = emptyList(),
    @param:Json(name = "tmdb_id") val tmdbId: Int? = null,
    @param:Json(name = "tvdb_id") val tvdbId: String? = null,
    @param:Json(name = "overview") val overview: String? = null,
    @param:Json(name = "poster_path") val posterPath: String? = null,
    @param:Json(name = "backdrop_path") val backdropPath: String? = null,
    @param:Json(name = "poster_url") val posterUrl: String? = null,
    @param:Json(name = "backdrop_url") val backdropUrl: String? = null,
    @param:Json(name = "show_poster_path") val showPosterPath: String? = null,
    @param:Json(name = "show_poster_url") val showPosterUrl: String? = null,
    @param:Json(name = "release_date") val releaseDate: String? = null,
    @param:Json(name = "show_vote_average") val showVoteAverage: Double? = null,
    @param:Json(name = "show_imdb_rating") val showImdbRating: Double? = null,
    @param:Json(name = "vote_average") val voteAverage: Double? = null,
    @param:Json(name = "imdb_id") val imdbId: String? = null,
    @param:Json(name = "imdb_rating") val imdbRating: Double? = null,
    @param:Json(name = "progress_seconds") val progressSeconds: Double? = null,
    @param:Json(name = "progress_percent") val progressPercent: Double? = null,
    @param:Json(name = "remaining_seconds") val remainingSeconds: Double? = null,
    @param:Json(name = "completed") val completed: Boolean? = null,
    @param:Json(name = "last_watched_at") val lastWatchedAt: String? = null,
    @param:Json(name = "season") val season: Int? = null,
    @param:Json(name = "episode") val episode: Int? = null,
    @param:Json(name = "metadata_review_needed") val metadataReviewNeeded: Boolean? = null,
    @param:Json(name = "metadata_confirmed") val metadataConfirmed: Boolean? = null,
    @param:Json(name = "thumbnail_path") val thumbnailPath: String? = null,
    @param:Json(name = "thumbnail_url") val thumbnailUrl: String? = null,
    @param:Json(name = "missing") val missing: Boolean? = null,
    @param:Json(name = "missing_since") val missingSince: String? = null,
    @param:Json(name = "intro_start_seconds") val introStartSeconds: Double? = null,
    @param:Json(name = "intro_end_seconds") val introEndSeconds: Double? = null,
)

@JsonClass(generateAdapter = true)
data class ContinueWatchingEntryJson(
    @param:Json(name = "kind") val kind: String,
    @param:Json(name = "media") val media: MediaItemJson,
    @param:Json(name = "show_key") val showKey: String? = null,
    @param:Json(name = "show_title") val showTitle: String? = null,
    @param:Json(name = "episode_label") val episodeLabel: String? = null,
    @param:Json(name = "remaining_seconds") val remainingSeconds: Double,
)

@JsonClass(generateAdapter = true)
data class RecentlyAddedEntryJson(
    @param:Json(name = "kind") val kind: String,
    @param:Json(name = "media") val media: MediaItemJson,
    @param:Json(name = "show_key") val showKey: String? = null,
    @param:Json(name = "show_title") val showTitle: String? = null,
    @param:Json(name = "episode_label") val episodeLabel: String? = null,
)

@JsonClass(generateAdapter = true)
data class HomeDashboardJson(
    @param:Json(name = "continueWatching") val continueWatching: List<ContinueWatchingEntryJson> = emptyList(),
    @param:Json(name = "recentlyAddedTvEpisodes") val recentlyAddedTvEpisodes: List<RecentlyAddedEntryJson> = emptyList(),
    @param:Json(name = "recentlyAddedTvShows") val recentlyAddedTvShows: List<RecentlyAddedEntryJson> = emptyList(),
    @param:Json(name = "recentlyAddedMovies") val recentlyAddedMovies: List<RecentlyAddedEntryJson> = emptyList(),
    @param:Json(name = "recentlyAddedAnimeEpisodes") val recentlyAddedAnimeEpisodes: List<RecentlyAddedEntryJson> = emptyList(),
    @param:Json(name = "recentlyAddedAnimeShows") val recentlyAddedAnimeShows: List<RecentlyAddedEntryJson> = emptyList(),
)

/** Wrapper for on-disk home dashboard cache (scoped to [serverUrl]). */
@JsonClass(generateAdapter = true)
data class CachedHomeDashboardEnvelope(
    @param:Json(name = "server_url") val serverUrl: String,
    @param:Json(name = "dashboard") val dashboard: HomeDashboardJson,
)

@JsonClass(generateAdapter = true)
data class LibraryJson(
    @param:Json(name = "id") val id: Int,
    @param:Json(name = "name") val name: String,
    @param:Json(name = "type") val type: String,
    @param:Json(name = "path") val path: String,
    @param:Json(name = "user_id") val userId: Int,
    @param:Json(name = "intro_skip_mode") val introSkipMode: String? = null,
)

@JsonClass(generateAdapter = true)
data class LibraryBrowseItemJson(
    @param:Json(name = "id") val id: Int,
    @param:Json(name = "library_id") val libraryId: Int? = null,
    @param:Json(name = "title") val title: String,
    @param:Json(name = "path") val path: String,
    @param:Json(name = "duration") val duration: Int,
    @param:Json(name = "type") val type: String,
    @param:Json(name = "match_status") val matchStatus: String? = null,
    @param:Json(name = "identify_state") val identifyState: String? = null,
    @param:Json(name = "tmdb_id") val tmdbId: Int? = null,
    @param:Json(name = "tvdb_id") val tvdbId: String? = null,
    @param:Json(name = "overview") val overview: String? = null,
    @param:Json(name = "poster_path") val posterPath: String? = null,
    @param:Json(name = "backdrop_path") val backdropPath: String? = null,
    @param:Json(name = "poster_url") val posterUrl: String? = null,
    @param:Json(name = "backdrop_url") val backdropUrl: String? = null,
    @param:Json(name = "show_poster_path") val showPosterPath: String? = null,
    @param:Json(name = "show_poster_url") val showPosterUrl: String? = null,
    @param:Json(name = "release_date") val releaseDate: String? = null,
    @param:Json(name = "show_vote_average") val showVoteAverage: Double? = null,
    @param:Json(name = "show_imdb_rating") val showImdbRating: Double? = null,
    @param:Json(name = "vote_average") val voteAverage: Double? = null,
    @param:Json(name = "imdb_id") val imdbId: String? = null,
    @param:Json(name = "imdb_rating") val imdbRating: Double? = null,
    @param:Json(name = "progress_seconds") val progressSeconds: Double? = null,
    @param:Json(name = "progress_percent") val progressPercent: Double? = null,
    @param:Json(name = "remaining_seconds") val remainingSeconds: Double? = null,
    @param:Json(name = "completed") val completed: Boolean? = null,
    @param:Json(name = "last_watched_at") val lastWatchedAt: String? = null,
    @param:Json(name = "season") val season: Int? = null,
    @param:Json(name = "episode") val episode: Int? = null,
    @param:Json(name = "metadata_review_needed") val metadataReviewNeeded: Boolean? = null,
    @param:Json(name = "metadata_confirmed") val metadataConfirmed: Boolean? = null,
    @param:Json(name = "thumbnail_path") val thumbnailPath: String? = null,
    @param:Json(name = "thumbnail_url") val thumbnailUrl: String? = null,
    @param:Json(name = "missing") val missing: Boolean? = null,
    @param:Json(name = "intro_start_seconds") val introStartSeconds: Double? = null,
    @param:Json(name = "intro_end_seconds") val introEndSeconds: Double? = null,
)

@JsonClass(generateAdapter = true)
data class LibraryMediaPageJson(
    @param:Json(name = "items") val items: List<LibraryBrowseItemJson> = emptyList(),
    @param:Json(name = "next_offset") val nextOffset: Int? = null,
    @param:Json(name = "has_more") val hasMore: Boolean = false,
    @param:Json(name = "total") val total: Int? = null,
)

@JsonClass(generateAdapter = true)
data class TitleCastMemberJson(
    @param:Json(name = "name") val name: String,
    @param:Json(name = "character") val character: String? = null,
    @param:Json(name = "order") val order: Int? = null,
    @param:Json(name = "profile_path") val profilePath: String? = null,
)

@JsonClass(generateAdapter = true)
data class LibraryMovieDetailsJson(
    @param:Json(name = "media_id") val mediaId: Int,
    @param:Json(name = "library_id") val libraryId: Int,
    @param:Json(name = "title") val title: String,
    @param:Json(name = "overview") val overview: String,
    @param:Json(name = "poster_path") val posterPath: String? = null,
    @param:Json(name = "poster_url") val posterUrl: String? = null,
    @param:Json(name = "backdrop_path") val backdropPath: String? = null,
    @param:Json(name = "backdrop_url") val backdropUrl: String? = null,
    @param:Json(name = "release_date") val releaseDate: String? = null,
    @param:Json(name = "vote_average") val voteAverage: Double? = null,
    @param:Json(name = "imdb_id") val imdbId: String? = null,
    @param:Json(name = "imdb_rating") val imdbRating: Double? = null,
    @param:Json(name = "runtime") val runtime: Int? = null,
    @param:Json(name = "subtitles") val subtitles: List<SubtitleJson> = emptyList(),
    @param:Json(name = "embeddedSubtitles") val embeddedSubtitles: List<EmbeddedSubtitleJson> = emptyList(),
    @param:Json(name = "embeddedAudioTracks") val embeddedAudioTracks: List<EmbeddedAudioTrackJson> = emptyList(),
    @param:Json(name = "genres") val genres: List<String> = emptyList(),
    @param:Json(name = "cast") val cast: List<TitleCastMemberJson>? = null,
    @param:Json(name = "progress_seconds") val progressSeconds: Double? = null,
    @param:Json(name = "progress_percent") val progressPercent: Double? = null,
    @param:Json(name = "completed") val completed: Boolean? = null,
)

@JsonClass(generateAdapter = true)
data class LibraryShowDetailsJson(
    @param:Json(name = "library_id") val libraryId: Int,
    @param:Json(name = "show_key") val showKey: String,
    @param:Json(name = "name") val name: String,
    @param:Json(name = "overview") val overview: String,
    @param:Json(name = "poster_path") val posterPath: String? = null,
    @param:Json(name = "poster_url") val posterUrl: String? = null,
    @param:Json(name = "backdrop_path") val backdropPath: String? = null,
    @param:Json(name = "backdrop_url") val backdropUrl: String? = null,
    @param:Json(name = "first_air_date") val firstAirDate: String = "",
    @param:Json(name = "vote_average") val voteAverage: Double? = null,
    @param:Json(name = "imdb_id") val imdbId: String? = null,
    @param:Json(name = "imdb_rating") val imdbRating: Double? = null,
    @param:Json(name = "runtime") val runtime: Int? = null,
    @param:Json(name = "number_of_seasons") val numberOfSeasons: Int = 0,
    @param:Json(name = "number_of_episodes") val numberOfEpisodes: Int = 0,
    @param:Json(name = "genres") val genres: List<String> = emptyList(),
    @param:Json(name = "cast") val cast: List<TitleCastMemberJson>? = null,
)

@JsonClass(generateAdapter = true)
data class ShowSeasonEpisodesJson(
    @param:Json(name = "seasonNumber") val seasonNumber: Int,
    @param:Json(name = "label") val label: String,
    @param:Json(name = "episodes") val episodes: List<LibraryBrowseItemJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class ShowEpisodesResponseJson(
    @param:Json(name = "intro_skip_mode") val introSkipMode: String? = null,
    @param:Json(name = "seasons") val seasons: List<ShowSeasonEpisodesJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class DiscoverLibraryMatchJson(
    @param:Json(name = "library_id") val libraryId: Int,
    @param:Json(name = "library_name") val libraryName: String,
    @param:Json(name = "library_type") val libraryType: String,
    @param:Json(name = "kind") val kind: String,
    @param:Json(name = "show_key") val showKey: String? = null,
)

@JsonClass(generateAdapter = true)
data class DiscoverAcquisitionJson(
    @param:Json(name = "state") val state: String,
    @param:Json(name = "source") val source: String? = null,
    @param:Json(name = "can_add") val canAdd: Boolean? = null,
    @param:Json(name = "is_configured") val isConfigured: Boolean? = null,
)

@JsonClass(generateAdapter = true)
data class DiscoverItemJson(
    @param:Json(name = "media_type") val mediaType: String,
    @param:Json(name = "tmdb_id") val tmdbId: Int,
    @param:Json(name = "title") val title: String,
    @param:Json(name = "overview") val overview: String? = null,
    @param:Json(name = "poster_path") val posterPath: String? = null,
    @param:Json(name = "backdrop_path") val backdropPath: String? = null,
    @param:Json(name = "release_date") val releaseDate: String? = null,
    @param:Json(name = "first_air_date") val firstAirDate: String? = null,
    @param:Json(name = "vote_average") val voteAverage: Double? = null,
    @param:Json(name = "library_matches") val libraryMatches: List<DiscoverLibraryMatchJson> = emptyList(),
    @param:Json(name = "acquisition") val acquisition: DiscoverAcquisitionJson? = null,
)

@JsonClass(generateAdapter = true)
data class DiscoverShelfJson(
    @param:Json(name = "id") val id: String,
    @param:Json(name = "title") val title: String,
    @param:Json(name = "items") val items: List<DiscoverItemJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class DiscoverResponseJson(
    @param:Json(name = "shelves") val shelves: List<DiscoverShelfJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class DiscoverGenreJson(
    @param:Json(name = "id") val id: Int,
    @param:Json(name = "name") val name: String,
)

@JsonClass(generateAdapter = true)
data class DiscoverGenresResponseJson(
    @param:Json(name = "movie_genres") val movieGenres: List<DiscoverGenreJson> = emptyList(),
    @param:Json(name = "tv_genres") val tvGenres: List<DiscoverGenreJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class DiscoverSearchResponseJson(
    @param:Json(name = "movies") val movies: List<DiscoverItemJson> = emptyList(),
    @param:Json(name = "tv") val tv: List<DiscoverItemJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class DiscoverBrowseResponseJson(
    @param:Json(name = "items") val items: List<DiscoverItemJson> = emptyList(),
    @param:Json(name = "page") val page: Int,
    @param:Json(name = "total_pages") val totalPages: Int,
    @param:Json(name = "total_results") val totalResults: Int,
    @param:Json(name = "media_type") val mediaType: String? = null,
    @param:Json(name = "genre") val genre: DiscoverGenreJson? = null,
    @param:Json(name = "category") val category: String? = null,
)

@JsonClass(generateAdapter = true)
data class DiscoverTitleVideoJson(
    @param:Json(name = "name") val name: String,
    @param:Json(name = "site") val site: String,
    @param:Json(name = "key") val key: String,
    @param:Json(name = "type") val type: String,
    @param:Json(name = "official") val official: Boolean? = null,
)

@JsonClass(generateAdapter = true)
data class DiscoverTitleDetailsJson(
    @param:Json(name = "media_type") val mediaType: String,
    @param:Json(name = "tmdb_id") val tmdbId: Int,
    @param:Json(name = "title") val title: String,
    @param:Json(name = "overview") val overview: String,
    @param:Json(name = "poster_path") val posterPath: String? = null,
    @param:Json(name = "backdrop_path") val backdropPath: String? = null,
    @param:Json(name = "release_date") val releaseDate: String? = null,
    @param:Json(name = "first_air_date") val firstAirDate: String? = null,
    @param:Json(name = "vote_average") val voteAverage: Double? = null,
    @param:Json(name = "imdb_id") val imdbId: String? = null,
    @param:Json(name = "imdb_rating") val imdbRating: Double? = null,
    @param:Json(name = "status") val status: String? = null,
    @param:Json(name = "genres") val genres: List<String> = emptyList(),
    @param:Json(name = "runtime") val runtime: Int? = null,
    @param:Json(name = "number_of_seasons") val numberOfSeasons: Int? = null,
    @param:Json(name = "number_of_episodes") val numberOfEpisodes: Int? = null,
    @param:Json(name = "videos") val videos: List<DiscoverTitleVideoJson> = emptyList(),
    @param:Json(name = "library_matches") val libraryMatches: List<DiscoverLibraryMatchJson> = emptyList(),
    @param:Json(name = "acquisition") val acquisition: DiscoverAcquisitionJson? = null,
)

@JsonClass(generateAdapter = true)
data class DownloadItemJson(
    @param:Json(name = "id") val id: String,
    @param:Json(name = "title") val title: String,
    @param:Json(name = "media_type") val mediaType: String,
    @param:Json(name = "source") val source: String,
    @param:Json(name = "status_text") val statusText: String,
    @param:Json(name = "progress") val progress: Double? = null,
    @param:Json(name = "size_left_bytes") val sizeLeftBytes: Long? = null,
    @param:Json(name = "eta_seconds") val etaSeconds: Double? = null,
    @param:Json(name = "error_message") val errorMessage: String? = null,
)

@JsonClass(generateAdapter = true)
data class DownloadsResponseJson(
    @param:Json(name = "configured") val configured: Boolean,
    @param:Json(name = "items") val items: List<DownloadItemJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class RemoveDownloadPayloadJson(
    @param:Json(name = "id") val id: String,
)

@JsonClass(generateAdapter = true)
data class SearchFacetValueJson(
    @param:Json(name = "value") val value: String,
    @param:Json(name = "label") val label: String,
    @param:Json(name = "count") val count: Int,
)

@JsonClass(generateAdapter = true)
data class SearchFacetsJson(
    @param:Json(name = "libraries") val libraries: List<SearchFacetValueJson> = emptyList(),
    @param:Json(name = "types") val types: List<SearchFacetValueJson> = emptyList(),
    @param:Json(name = "genres") val genres: List<SearchFacetValueJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class SearchResultJson(
    @param:Json(name = "kind") val kind: String,
    @param:Json(name = "library_id") val libraryId: Int,
    @param:Json(name = "library_name") val libraryName: String,
    @param:Json(name = "library_type") val libraryType: String,
    @param:Json(name = "title") val title: String,
    @param:Json(name = "subtitle") val subtitle: String? = null,
    @param:Json(name = "poster_path") val posterPath: String? = null,
    @param:Json(name = "poster_url") val posterUrl: String? = null,
    @param:Json(name = "imdb_rating") val imdbRating: Double? = null,
    @param:Json(name = "match_reason") val matchReason: String,
    @param:Json(name = "matched_actor") val matchedActor: String? = null,
    @param:Json(name = "href") val href: String,
    @param:Json(name = "genres") val genres: List<String> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class SearchResponseJson(
    @param:Json(name = "query") val query: String,
    @param:Json(name = "results") val results: List<SearchResultJson> = emptyList(),
    @param:Json(name = "total") val total: Int = 0,
    @param:Json(name = "facets") val facets: SearchFacetsJson = SearchFacetsJson(),
)
