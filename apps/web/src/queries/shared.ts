import type { QueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { recentlyAddedEntryKey } from "@/lib/libraryReadyNotifications";
import type {
  DiscoverBrowseCategory,
  DiscoverMediaType,
  HomeDashboard,
  MediaItem,
  MovieDetails,
  RecentlyAddedEntry,
  ShowDetails,
} from "@/api";
import { fetchLibraryMedia } from "@/api";

/**
 * JSON is validated in `@plum/shared` with `@plum/contracts` schemas. Effect schema `Type` is deeply
 * readonly; hooks use mutable DTO interfaces from the same contracts — widen here only for TypeScript.
 * Runtime shape is assumed to match the Plum server and shared decode path; there is no second client-side decode here.
 */
export function contractsView<T>(value: unknown): T {
  return value as T;
}

export function notifyMutationError(error: Error, fallback: string): void {
  toast.error(error.message || fallback);
}

export type LibraryMediaPageResult = Exclude<Awaited<ReturnType<typeof fetchLibraryMedia>>, MediaItem[]>;

export function normalizeLibraryMediaPage(
  response: Awaited<ReturnType<typeof fetchLibraryMedia>>,
): LibraryMediaPageResult {
  if (Array.isArray(response)) {
    return {
      items: response,
      has_more: false,
      total: response.length,
    };
  }
  return response;
}

/** Library browse lists do not render sidecar / embedded subtitle rows; playback merges tracks from the session response. */
export function stripLibraryBrowseMediaItem(item: MediaItem): MediaItem {
  return { ...item, subtitles: undefined, embeddedSubtitles: undefined };
}

/** Fields read by `ShowDetail` from `useShowDetails` (avoids caching unused counts/runtime/ids). */
export type ShowDetailsPage = Pick<
  ShowDetails,
  | "name"
  | "poster_path"
  | "poster_url"
  | "backdrop_path"
  | "backdrop_url"
  | "first_air_date"
  | "overview"
  | "genres"
  | "cast"
  | "vote_average"
  | "imdb_rating"
>;

export function selectShowDetailsForPage(data: ShowDetails | null): ShowDetailsPage | null {
  if (data == null) return null;
  return {
    name: data.name,
    poster_path: data.poster_path,
    poster_url: data.poster_url,
    backdrop_path: data.backdrop_path,
    backdrop_url: data.backdrop_url,
    first_air_date: data.first_air_date,
    overview: data.overview,
    genres: data.genres,
    cast: data.cast,
    vote_average: data.vote_average,
    imdb_rating: data.imdb_rating,
  };
}

/** Fields `MovieDetail` reads from the details query (IDs come from the route). */
export type MovieDetailsPage = Pick<
  MovieDetails,
  | "title"
  | "source_path"
  | "overview"
  | "poster_path"
  | "poster_url"
  | "backdrop_path"
  | "backdrop_url"
  | "release_date"
  | "vote_average"
  | "imdb_id"
  | "imdb_rating"
  | "runtime"
  | "progress_seconds"
  | "progress_percent"
  | "completed"
  | "subtitles"
  | "embeddedSubtitles"
  | "embeddedAudioTracks"
  | "genres"
  | "cast"
>;

export function selectMovieDetailsForPage(data: MovieDetails | null): MovieDetailsPage | null {
  if (data == null) return null;
  return {
    title: data.title,
    source_path: data.source_path,
    overview: data.overview,
    poster_path: data.poster_path,
    poster_url: data.poster_url,
    backdrop_path: data.backdrop_path,
    backdrop_url: data.backdrop_url,
    release_date: data.release_date,
    vote_average: data.vote_average,
    imdb_id: data.imdb_id,
    imdb_rating: data.imdb_rating,
    runtime: data.runtime,
    progress_seconds: data.progress_seconds,
    progress_percent: data.progress_percent,
    completed: data.completed,
    subtitles: data.subtitles,
    embeddedSubtitles: data.embeddedSubtitles,
    embeddedAudioTracks: data.embeddedAudioTracks,
    genres: data.genres,
    cast: data.cast,
  };
}

function mergeDashboardRecentlyAddedForNotifier(
  tvEpisodes: RecentlyAddedEntry[],
  tvShows: RecentlyAddedEntry[],
  movies: RecentlyAddedEntry[],
  animeEpisodes: RecentlyAddedEntry[],
  animeShows: RecentlyAddedEntry[],
): RecentlyAddedEntry[] {
  const merged = [...tvEpisodes, ...tvShows, ...movies, ...animeEpisodes, ...animeShows];
  const seen = new Set<string>();
  const out: RecentlyAddedEntry[] = [];
  for (const e of merged) {
    const k = recentlyAddedEntryKey(e);
    if (seen.has(k)) continue;
    seen.add(k);
    out.push(e);
  }
  return out;
}

export function buildHomeDashboard(dashboard: HomeDashboard): HomeDashboard {
  const recentlyAddedTvEpisodes = (dashboard.recentlyAddedTvEpisodes ?? []) as RecentlyAddedEntry[];
  const recentlyAddedTvShows = (dashboard.recentlyAddedTvShows ?? []) as RecentlyAddedEntry[];
  const recentlyAddedMovies = (dashboard.recentlyAddedMovies ?? []) as RecentlyAddedEntry[];
  const recentlyAddedAnimeEpisodes = (dashboard.recentlyAddedAnimeEpisodes ?? []) as RecentlyAddedEntry[];
  const recentlyAddedAnimeShows = (dashboard.recentlyAddedAnimeShows ?? []) as RecentlyAddedEntry[];
  return {
    ...dashboard,
    recentlyAdded: mergeDashboardRecentlyAddedForNotifier(
      recentlyAddedTvEpisodes,
      recentlyAddedTvShows,
      recentlyAddedMovies,
      recentlyAddedAnimeEpisodes,
      recentlyAddedAnimeShows,
    ),
  } as HomeDashboard;
}

export const queryKeys = {
  discover: (originCountry: string) => ["discover", originCountry] as const,
  /** Prefix: invalidates all `discover` shelf queries regardless of origin. */
  discoverAll: ["discover"] as const,
  discoverBrowse: (
    category: DiscoverBrowseCategory | "",
    mediaType: DiscoverMediaType | "",
    genreId: number | null,
    originCountry: string,
  ) => ["discover-browse", category, mediaType, genreId ?? 0, originCountry] as const,
  discoverBrowseAll: ["discover-browse"] as const,
  discoverGenres: ["discover-genres"] as const,
  discoverSearch: (query: string) => ["discover-search", query] as const,
  discoverSearchAll: ["discover-search"] as const,
  discoverTitle: (mediaType: DiscoverMediaType, tmdbId: number) =>
    ["discover-title", mediaType, tmdbId] as const,
  discoverTitleAll: ["discover-title"] as const,
  downloads: ["downloads"] as const,
  home: ["home"] as const,
  libraries: ["libraries"] as const,
  unidentifiedSummary: ["libraries", "unidentified-summary"] as const,
  introScanSummary: ["intro-scan-summary"] as const,
  introScanShows: (libraryId: number) => ["intro-scan-shows", libraryId] as const,
  introRefreshStatus: ["intro-refresh-status"] as const,
  library: (id: number, pageSize?: number) =>
    pageSize == null ? (["library", id] as const) : (["library", id, pageSize] as const),
  movieDetails: (libraryId: number, mediaId: number) => ["movie-details", libraryId, mediaId] as const,
  movieDetailsByLibrary: (libraryId: number) => ["movie-details", libraryId] as const,
  moviePosterCandidates: (libraryId: number, mediaId: number) =>
    ["movie-poster-candidates", libraryId, mediaId] as const,
  metadataArtworkSettings: ["metadata-artwork-settings"] as const,
  mediaStackSettings: ["media-stack-settings"] as const,
  serverEnvSettings: ["server-env-settings"] as const,
  /** Library id first so `searchByLibrary(id)` invalidates scoped results without touching other libraries. */
  search: (libraryId: number, query: string, type: string, genre: string) =>
    ["search", libraryId, query, type, genre] as const,
  /** Prefix: invalidates all `useLibrarySearch` queries. */
  searchAll: ["search"] as const,
  /** Prefix: scoped library search + global search (`libraryId` 0 = all libraries). */
  searchByLibrary: (libraryId: number) => ["search", libraryId] as const,
  series: (tmdbId: number) => ["series", tmdbId] as const,
  showPosterCandidates: (libraryId: number, showKey: string) =>
    ["show-poster-candidates", libraryId, showKey] as const,
  showDetails: (libraryId: number, showKey: string) => ["show-details", libraryId, showKey] as const,
  showDetailsByLibrary: (libraryId: number) => ["show-details", libraryId] as const,
  showEpisodes: (libraryId: number, showKey: string) =>
    ["library", libraryId, "show-episodes", showKey] as const,
  transcodingSettings: ["transcoding-settings"] as const,
};

export function invalidateSearchAfterLibraryDataChange(
  queryClient: QueryClient,
  libraryId: number,
): void {
  void queryClient.invalidateQueries({ queryKey: queryKeys.searchByLibrary(libraryId) });
  void queryClient.invalidateQueries({ queryKey: queryKeys.searchByLibrary(0) });
}

/**
 * Refetch library browse, home, and per-title movie/show detail caches for one library.
 * Used when scan status changes over WebSocket *and* when status is refreshed via polling so
 * behavior stays the same if `/ws` is down or stale after deploy.
 */
export function invalidateLibraryCatalogQueries(queryClient: QueryClient, libraryId: number): void {
  void queryClient.invalidateQueries({ queryKey: queryKeys.library(libraryId) });
  void queryClient.invalidateQueries({ queryKey: queryKeys.libraries });
  void queryClient.invalidateQueries({ queryKey: queryKeys.unidentifiedSummary });
  void queryClient.invalidateQueries({ queryKey: queryKeys.home });
  void queryClient.invalidateQueries({ queryKey: queryKeys.showDetailsByLibrary(libraryId) });
  void queryClient.invalidateQueries({ queryKey: queryKeys.movieDetailsByLibrary(libraryId) });
}

/** Refetch Discover shelves, search, and title detail queries (e.g. after downloads or library scans). */
export function invalidateDiscoverRelatedQueries(queryClient: QueryClient): void {
  void queryClient.invalidateQueries({ queryKey: queryKeys.discoverAll });
  void queryClient.invalidateQueries({ queryKey: queryKeys.discoverBrowseAll });
  void queryClient.invalidateQueries({ queryKey: queryKeys.discoverGenres });
  void queryClient.invalidateQueries({ queryKey: queryKeys.discoverSearchAll });
  void queryClient.invalidateQueries({ queryKey: queryKeys.discoverTitleAll });
}

/** Grids and lists that shift when scans, identification, or playback progress updates land. */
export const SCAN_SENSITIVE_STALE_MS = 30 * 1000;
/** TMDB-backed title payloads; explicit mutations invalidate. */
export const METADATA_DETAIL_STALE_MS = 5 * 60 * 1000;
/** Settings/admin JSON from the server; changes only via Settings or rare env updates. */
export const SERVER_SETTINGS_STALE_MS = 60 * 1000;
export const DISCOVER_STALE_MS = 5 * 60 * 1000;
