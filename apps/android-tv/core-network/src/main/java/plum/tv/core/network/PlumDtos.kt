package plum.tv.core.network

import com.squareup.moshi.Json
import com.squareup.moshi.JsonClass

@JsonClass(generateAdapter = true)
data class SubtitleJson(
    @Json(name = "id") val id: Int = 0,
    @Json(name = "title") val title: String = "",
    @Json(name = "language") val language: String = "",
    @Json(name = "format") val format: String = "",
)

@JsonClass(generateAdapter = true)
data class EmbeddedSubtitleJson(
    @Json(name = "streamIndex") val streamIndex: Int = 0,
    @Json(name = "language") val language: String = "",
    @Json(name = "title") val title: String = "",
    @Json(name = "codec") val codec: String? = null,
    /** When false, server rejects WebVTT extract; PGS may still sideload via pgsBinaryEligible and /sup. */
    @Json(name = "supported") val supported: Boolean? = null,
    /** Playback session: false for bitmap subs; omit on older servers → treat as eligible. */
    @Json(name = "vttEligible") val vttEligible: Boolean = true,
    /** Playback session: raw PGS demux for Media3 (Jellyfin-style); default false when omitted. */
    @Json(name = "pgsBinaryEligible") val pgsBinaryEligible: Boolean = false,
)

@JsonClass(generateAdapter = true)
data class EmbeddedAudioTrackJson(
    @Json(name = "streamIndex") val streamIndex: Int = 0,
    @Json(name = "language") val language: String = "",
    @Json(name = "title") val title: String = "",
)

/** Home dashboard and browse responses use the same core media shape as the server. */
@JsonClass(generateAdapter = true)
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
    @Json(name = "show_imdb_rating") val showImdbRating: Double? = null,
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
    @Json(name = "intro_start_seconds") val introStartSeconds: Double? = null,
    @Json(name = "intro_end_seconds") val introEndSeconds: Double? = null,
)

@JsonClass(generateAdapter = true)
data class ContinueWatchingEntryJson(
    @Json(name = "kind") val kind: String,
    @Json(name = "media") val media: MediaItemJson,
    @Json(name = "show_key") val showKey: String? = null,
    @Json(name = "show_title") val showTitle: String? = null,
    @Json(name = "episode_label") val episodeLabel: String? = null,
    @Json(name = "remaining_seconds") val remainingSeconds: Double,
)

@JsonClass(generateAdapter = true)
data class RecentlyAddedEntryJson(
    @Json(name = "kind") val kind: String,
    @Json(name = "media") val media: MediaItemJson,
    @Json(name = "show_key") val showKey: String? = null,
    @Json(name = "show_title") val showTitle: String? = null,
    @Json(name = "episode_label") val episodeLabel: String? = null,
)

@JsonClass(generateAdapter = true)
data class HomeDashboardJson(
    @Json(name = "continueWatching") val continueWatching: List<ContinueWatchingEntryJson> = emptyList(),
    @Json(name = "recentlyAddedTvEpisodes") val recentlyAddedTvEpisodes: List<RecentlyAddedEntryJson> = emptyList(),
    @Json(name = "recentlyAddedTvShows") val recentlyAddedTvShows: List<RecentlyAddedEntryJson> = emptyList(),
    @Json(name = "recentlyAddedMovies") val recentlyAddedMovies: List<RecentlyAddedEntryJson> = emptyList(),
    @Json(name = "recentlyAddedAnimeEpisodes") val recentlyAddedAnimeEpisodes: List<RecentlyAddedEntryJson> = emptyList(),
    @Json(name = "recentlyAddedAnimeShows") val recentlyAddedAnimeShows: List<RecentlyAddedEntryJson> = emptyList(),
)

/** Wrapper for on-disk home dashboard cache (scoped to [serverUrl]). */
@JsonClass(generateAdapter = true)
data class CachedHomeDashboardEnvelope(
    @Json(name = "server_url") val serverUrl: String,
    @Json(name = "dashboard") val dashboard: HomeDashboardJson,
)

@JsonClass(generateAdapter = true)
data class LibraryJson(
    @Json(name = "id") val id: Int,
    @Json(name = "name") val name: String,
    @Json(name = "type") val type: String,
    @Json(name = "path") val path: String,
    @Json(name = "user_id") val userId: Int,
    @Json(name = "intro_skip_mode") val introSkipMode: String? = null,
)

@JsonClass(generateAdapter = true)
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
    @Json(name = "show_imdb_rating") val showImdbRating: Double? = null,
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
    @Json(name = "intro_start_seconds") val introStartSeconds: Double? = null,
    @Json(name = "intro_end_seconds") val introEndSeconds: Double? = null,
)

@JsonClass(generateAdapter = true)
data class LibraryMediaPageJson(
    @Json(name = "items") val items: List<LibraryBrowseItemJson> = emptyList(),
    @Json(name = "next_offset") val nextOffset: Int? = null,
    @Json(name = "has_more") val hasMore: Boolean = false,
    @Json(name = "total") val total: Int? = null,
)

@JsonClass(generateAdapter = true)
data class TitleCastMemberJson(
    @Json(name = "name") val name: String,
    @Json(name = "character") val character: String? = null,
    @Json(name = "order") val order: Int? = null,
    @Json(name = "profile_path") val profilePath: String? = null,
)

@JsonClass(generateAdapter = true)
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
    @Json(name = "cast") val cast: List<TitleCastMemberJson>? = null,
    @Json(name = "progress_seconds") val progressSeconds: Double? = null,
    @Json(name = "progress_percent") val progressPercent: Double? = null,
    @Json(name = "completed") val completed: Boolean? = null,
)

@JsonClass(generateAdapter = true)
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
    @Json(name = "cast") val cast: List<TitleCastMemberJson>? = null,
)

@JsonClass(generateAdapter = true)
data class ShowSeasonEpisodesJson(
    @Json(name = "seasonNumber") val seasonNumber: Int,
    @Json(name = "label") val label: String,
    @Json(name = "episodes") val episodes: List<LibraryBrowseItemJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class ShowEpisodesResponseJson(
    @Json(name = "intro_skip_mode") val introSkipMode: String? = null,
    @Json(name = "seasons") val seasons: List<ShowSeasonEpisodesJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class DiscoverLibraryMatchJson(
    @Json(name = "library_id") val libraryId: Int,
    @Json(name = "library_name") val libraryName: String,
    @Json(name = "library_type") val libraryType: String,
    @Json(name = "kind") val kind: String,
    @Json(name = "show_key") val showKey: String? = null,
)

@JsonClass(generateAdapter = true)
data class DiscoverAcquisitionJson(
    @Json(name = "state") val state: String,
    @Json(name = "source") val source: String? = null,
    @Json(name = "can_add") val canAdd: Boolean? = null,
    @Json(name = "is_configured") val isConfigured: Boolean? = null,
)

@JsonClass(generateAdapter = true)
data class DiscoverItemJson(
    @Json(name = "media_type") val mediaType: String,
    @Json(name = "tmdb_id") val tmdbId: Int,
    @Json(name = "title") val title: String,
    @Json(name = "overview") val overview: String? = null,
    @Json(name = "poster_path") val posterPath: String? = null,
    @Json(name = "backdrop_path") val backdropPath: String? = null,
    @Json(name = "release_date") val releaseDate: String? = null,
    @Json(name = "first_air_date") val firstAirDate: String? = null,
    @Json(name = "vote_average") val voteAverage: Double? = null,
    @Json(name = "library_matches") val libraryMatches: List<DiscoverLibraryMatchJson> = emptyList(),
    @Json(name = "acquisition") val acquisition: DiscoverAcquisitionJson? = null,
)

@JsonClass(generateAdapter = true)
data class DiscoverShelfJson(
    @Json(name = "id") val id: String,
    @Json(name = "title") val title: String,
    @Json(name = "items") val items: List<DiscoverItemJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class DiscoverResponseJson(
    @Json(name = "shelves") val shelves: List<DiscoverShelfJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class DiscoverGenreJson(
    @Json(name = "id") val id: Int,
    @Json(name = "name") val name: String,
)

@JsonClass(generateAdapter = true)
data class DiscoverGenresResponseJson(
    @Json(name = "movie_genres") val movieGenres: List<DiscoverGenreJson> = emptyList(),
    @Json(name = "tv_genres") val tvGenres: List<DiscoverGenreJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class DiscoverSearchResponseJson(
    @Json(name = "movies") val movies: List<DiscoverItemJson> = emptyList(),
    @Json(name = "tv") val tv: List<DiscoverItemJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class DiscoverBrowseResponseJson(
    @Json(name = "items") val items: List<DiscoverItemJson> = emptyList(),
    @Json(name = "page") val page: Int,
    @Json(name = "total_pages") val totalPages: Int,
    @Json(name = "total_results") val totalResults: Int,
    @Json(name = "media_type") val mediaType: String? = null,
    @Json(name = "genre") val genre: DiscoverGenreJson? = null,
    @Json(name = "category") val category: String? = null,
)

@JsonClass(generateAdapter = true)
data class DiscoverTitleVideoJson(
    @Json(name = "name") val name: String,
    @Json(name = "site") val site: String,
    @Json(name = "key") val key: String,
    @Json(name = "type") val type: String,
    @Json(name = "official") val official: Boolean? = null,
)

@JsonClass(generateAdapter = true)
data class DiscoverTitleDetailsJson(
    @Json(name = "media_type") val mediaType: String,
    @Json(name = "tmdb_id") val tmdbId: Int,
    @Json(name = "title") val title: String,
    @Json(name = "overview") val overview: String,
    @Json(name = "poster_path") val posterPath: String? = null,
    @Json(name = "backdrop_path") val backdropPath: String? = null,
    @Json(name = "release_date") val releaseDate: String? = null,
    @Json(name = "first_air_date") val firstAirDate: String? = null,
    @Json(name = "vote_average") val voteAverage: Double? = null,
    @Json(name = "imdb_id") val imdbId: String? = null,
    @Json(name = "imdb_rating") val imdbRating: Double? = null,
    @Json(name = "status") val status: String? = null,
    @Json(name = "genres") val genres: List<String> = emptyList(),
    @Json(name = "runtime") val runtime: Int? = null,
    @Json(name = "number_of_seasons") val numberOfSeasons: Int? = null,
    @Json(name = "number_of_episodes") val numberOfEpisodes: Int? = null,
    @Json(name = "videos") val videos: List<DiscoverTitleVideoJson> = emptyList(),
    @Json(name = "library_matches") val libraryMatches: List<DiscoverLibraryMatchJson> = emptyList(),
    @Json(name = "acquisition") val acquisition: DiscoverAcquisitionJson? = null,
)

@JsonClass(generateAdapter = true)
data class DownloadItemJson(
    @Json(name = "id") val id: String,
    @Json(name = "title") val title: String,
    @Json(name = "media_type") val mediaType: String,
    @Json(name = "source") val source: String,
    @Json(name = "status_text") val statusText: String,
    @Json(name = "progress") val progress: Double? = null,
    @Json(name = "size_left_bytes") val sizeLeftBytes: Long? = null,
    @Json(name = "eta_seconds") val etaSeconds: Double? = null,
    @Json(name = "error_message") val errorMessage: String? = null,
)

@JsonClass(generateAdapter = true)
data class DownloadsResponseJson(
    @Json(name = "configured") val configured: Boolean,
    @Json(name = "items") val items: List<DownloadItemJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
data class RemoveDownloadPayloadJson(
    @Json(name = "id") val id: String,
)

@JsonClass(generateAdapter = true)
data class SearchFacetValueJson(
    @Json(name = "value") val value: String,
    @Json(name = "label") val label: String,
    @Json(name = "count") val count: Int,
)

@JsonClass(generateAdapter = true)
data class SearchFacetsJson(
    @Json(name = "libraries") val libraries: List<SearchFacetValueJson> = emptyList(),
    @Json(name = "types") val types: List<SearchFacetValueJson> = emptyList(),
    @Json(name = "genres") val genres: List<SearchFacetValueJson> = emptyList(),
)

@JsonClass(generateAdapter = true)
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

@JsonClass(generateAdapter = true)
data class SearchResponseJson(
    @Json(name = "query") val query: String,
    @Json(name = "results") val results: List<SearchResultJson> = emptyList(),
    @Json(name = "total") val total: Int = 0,
    @Json(name = "facets") val facets: SearchFacetsJson = SearchFacetsJson(),
)
