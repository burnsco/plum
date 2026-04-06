import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import {
  addDiscoverTitle,
  browseDiscover,
  confirmShow,
  getDownloads,
  removeDownload,
  getDiscover,
  getDiscoverGenres,
  getDiscoverTitleDetails,
  getMovieDetails,
  getMoviePosterCandidates,
  getMetadataArtworkSettings,
  getMediaStackSettings,
  getServerEnvSettings,
  getShowDetails,
  getShowEpisodes,
  getShowPosterCandidates,
  getUnidentifiedLibrarySummaries,
  getIntroScanSummary,
  getIntroScanShowSummary,
  getIntroRefreshStatus,
  fetchLibraryMedia,
  fetchSeriesByTmdbId,
  getHomeDashboard,
  searchLibraryMedia,
  searchDiscover,
  type HomeDashboard,
  type MovieDetails,
  type SearchResponse,
  getTranscodingSettings,
  identifyLibrary,
  listLibraries,
  refreshLibraryPlaybackTracks,
  refreshPlaybackTracks,
  refreshShow,
  scanLibraryById,
  type DiscoverBrowseCategory,
  type DiscoverMediaType,
  type DiscoverAcquisition,
  type DiscoverBrowseResponse,
  type DiscoverGenresResponse,
  type DiscoverResponse,
  type DiscoverSearchResponse,
  type DiscoverTitleDetails,
  type DownloadsResponse,
  type IdentifyResult,
  type IntroRefreshStatusResponse,
  type IntroScanSummaryResponse,
  type IntroScanShowSummaryResponse,
  type Library,
  type MetadataArtworkSettings,
  type MetadataArtworkSettingsResponse,
  type MediaStackSettings,
  type MediaStackValidationResult,
  type MediaItem,
  type PosterCandidatesResponse,
  type ScanLibraryResult,
  type ServerEnvSettingsResponse,
  type ServerEnvSettingsUpdate,
  resetMoviePosterSelection,
  resetShowPosterSelection,
  setMoviePosterSelection,
  setShowPosterSelection,
  type SeriesDetails,
  type LibraryPlaybackTracksRefreshResult,
  type ShowActionResult,
  type ShowDetails,
  type ShowEpisodesResponse,
  type TranscodingSettings,
  type TranscodingSettingsResponse,
  type UnidentifiedLibrariesResponse,
  type UpdateLibraryPlaybackPreferencesPayload,
  updateLibraryPlaybackPreferences,
  updateMetadataArtworkSettings,
  updateMediaStackSettings,
  updateServerEnvSettings,
  updateTranscodingSettings,
  validateMediaStackSettings,
  type RecentlyAddedEntry,
} from "./api";
import { recentlyAddedEntryKey } from "@/lib/libraryReadyNotifications";
import { normalizeDiscoverOriginKey } from "@/lib/discover";

/**
 * JSON is validated in `@plum/shared` with `@plum/contracts` schemas. Effect schema `Type` is deeply
 * readonly; hooks use mutable DTO interfaces from the same contracts — widen here only for TypeScript.
 */
export function contractsView<T>(value: unknown): T {
  return value as T;
}

type LibraryMediaPageResult = Exclude<Awaited<ReturnType<typeof fetchLibraryMedia>>, MediaItem[]>;

function normalizeLibraryMediaPage(
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
function stripLibraryBrowseMediaItem(item: MediaItem): MediaItem {
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

function selectShowDetailsForPage(data: ShowDetails | null): ShowDetailsPage | null {
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
  | "subtitles"
  | "embeddedSubtitles"
  | "embeddedAudioTracks"
  | "genres"
  | "cast"
>;

function selectMovieDetailsForPage(data: MovieDetails | null): MovieDetailsPage | null {
  if (data == null) return null;
  return {
    title: data.title,
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
    subtitles: data.subtitles,
    embeddedSubtitles: data.embeddedSubtitles,
    embeddedAudioTracks: data.embeddedAudioTracks,
    genres: data.genres,
    cast: data.cast,
  };
}

/** Deduped merge for LibraryReadyNotifier (TV episodes → shows → movies → anime episodes → anime shows). */
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

function buildHomeDashboard(dashboard: HomeDashboard): HomeDashboard {
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
  showEpisodes: (libraryId: number, showKey: string) => ["library", libraryId, "show-episodes", showKey] as const,
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
const METADATA_DETAIL_STALE_MS = 5 * 60 * 1000;
/** Settings/admin JSON from the server; changes only via Settings or rare env updates. */
const SERVER_SETTINGS_STALE_MS = 60 * 1000;
const DISCOVER_STALE_MS = 5 * 60 * 1000;

export function useLibraries(): UseQueryResult<Library[], Error> {
  return useQuery({
    queryKey: queryKeys.libraries,
    queryFn: async () => contractsView<Library[]>(await listLibraries()),
    staleTime: SCAN_SENSITIVE_STALE_MS,
  });
}

export function useUnidentifiedLibrarySummaries(): UseQueryResult<
  UnidentifiedLibrariesResponse,
  Error
> {
  return useQuery({
    queryKey: queryKeys.unidentifiedSummary,
    queryFn: async () =>
      contractsView<UnidentifiedLibrariesResponse>(await getUnidentifiedLibrarySummaries()),
    staleTime: SCAN_SENSITIVE_STALE_MS,
  });
}

export function useDiscover(options?: {
  enabled?: boolean;
  refetchInterval?: number | false;
  originCountry?: string;
}): UseQueryResult<DiscoverResponse, Error> {
  const originKey = normalizeDiscoverOriginKey(options?.originCountry);
  return useQuery({
    queryKey: queryKeys.discover(originKey),
    queryFn: async () =>
      contractsView<DiscoverResponse>(
        await getDiscover(originKey ? { originCountry: originKey } : undefined),
      ),
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
    staleTime: DISCOVER_STALE_MS,
  });
}

export function useDiscoverGenres(options?: {
  enabled?: boolean;
  refetchInterval?: number | false;
}): UseQueryResult<DiscoverGenresResponse, Error> {
  return useQuery({
    queryKey: queryKeys.discoverGenres,
    queryFn: async () => contractsView<DiscoverGenresResponse>(await getDiscoverGenres()),
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
    staleTime: DISCOVER_STALE_MS,
  });
}

export function useDiscoverBrowse(
  options: {
    category?: DiscoverBrowseCategory | "";
    mediaType?: DiscoverMediaType | "";
    genreId?: number | null;
    originCountry?: string;
    enabled?: boolean;
    refetchInterval?: number | false;
  },
) {
  const category = options.category ?? "";
  const mediaType = options.mediaType ?? "";
  const genreId = options.genreId ?? null;
  const originKey = normalizeDiscoverOriginKey(options.originCountry);
  return useInfiniteQuery({
    queryKey: queryKeys.discoverBrowse(category, mediaType, genreId, originKey),
    queryFn: async ({ pageParam }) =>
      contractsView<DiscoverBrowseResponse>(
        await browseDiscover({
          category: category === "" ? undefined : category,
          mediaType: mediaType === "" ? undefined : mediaType,
          genreId: genreId ?? undefined,
          page: Number(pageParam ?? 1),
          ...(originKey ? { originCountry: originKey } : {}),
        }),
      ),
    initialPageParam: 1,
    getNextPageParam: (lastPage) =>
      lastPage.page < lastPage.total_pages ? lastPage.page + 1 : undefined,
    enabled: options.enabled ?? true,
    refetchInterval: options.refetchInterval,
    staleTime: DISCOVER_STALE_MS,
  });
}

export function useDiscoverSearch(
  query: string,
  options?: { enabled?: boolean; refetchInterval?: number | false },
): UseQueryResult<DiscoverSearchResponse, Error> {
  const normalizedQuery = query.trim();
  return useQuery({
    queryKey: queryKeys.discoverSearch(normalizedQuery),
    queryFn: async () =>
      contractsView<DiscoverSearchResponse>(await searchDiscover(normalizedQuery)),
    enabled: (options?.enabled ?? true) && normalizedQuery.length >= 2,
    refetchInterval: options?.refetchInterval,
    staleTime: DISCOVER_STALE_MS,
  });
}

export function useDiscoverTitleDetails(
  mediaType: DiscoverMediaType | null,
  tmdbId: number | null,
  options?: { enabled?: boolean; refetchInterval?: number | false },
): UseQueryResult<DiscoverTitleDetails | null, Error> {
  return useQuery({
    queryKey: queryKeys.discoverTitle(mediaType ?? "movie", tmdbId ?? 0),
    queryFn: async () =>
      contractsView<DiscoverTitleDetails | null>(await getDiscoverTitleDetails(mediaType!, tmdbId!)),
    enabled: (options?.enabled ?? true) && mediaType != null && tmdbId != null && tmdbId > 0,
    refetchInterval: options?.refetchInterval,
    staleTime: DISCOVER_STALE_MS,
  });
}

export function useDownloads(options?: {
  enabled?: boolean;
  refetchInterval?: number | false;
}): UseQueryResult<DownloadsResponse, Error> {
  return useQuery({
    queryKey: queryKeys.downloads,
    queryFn: async () => contractsView<DownloadsResponse>(await getDownloads()),
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
    staleTime: 5_000,
  });
}

export function useRemoveDownload(): UseMutationResult<void, Error, { id: string }> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ id }) => {
      await removeDownload({ id });
    },
    onSuccess: () => {
      invalidateDiscoverRelatedQueries(queryClient);
      void queryClient.invalidateQueries({ queryKey: queryKeys.downloads });
    },
  });
}

export function useHomeDashboard(options?: {
  enabled?: boolean;
}): UseQueryResult<HomeDashboard, Error> {
  return useQuery({
    queryKey: queryKeys.home,
    queryFn: async () => buildHomeDashboard(contractsView<HomeDashboard>(await getHomeDashboard())),
    enabled: options?.enabled ?? true,
    staleTime: SCAN_SENSITIVE_STALE_MS,
  });
}

export function useLibraryMedia(
  libraryId: number | null,
  options?: { enabled?: boolean; refetchInterval?: number | false; pageSize?: number },
) {
  const pageSize = options?.pageSize ?? 60;
  return useInfiniteQuery({
    queryKey: queryKeys.library(libraryId ?? 0, pageSize),
    queryFn: async ({ pageParam }) =>
      contractsView<LibraryMediaPageResult>(
        normalizeLibraryMediaPage(
          await fetchLibraryMedia(libraryId!, { offset: Number(pageParam ?? 0), limit: pageSize }),
        ),
      ),
    select: (data) => ({
      pages: data.pages.map((page) => ({
        ...page,
        items: page.items.map(stripLibraryBrowseMediaItem),
      })),
      pageParams: data.pageParams,
    }),
    enabled: (options?.enabled ?? true) && libraryId != null,
    initialPageParam: 0,
    getNextPageParam: (lastPage) => lastPage.next_offset,
    refetchInterval: options?.refetchInterval,
    staleTime: SCAN_SENSITIVE_STALE_MS,
  });
}

export function useScanLibrary(): UseMutationResult<
  ScanLibraryResult,
  Error,
  { libraryId: number; identify?: boolean; subpath?: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ libraryId, identify, subpath }) =>
      scanLibraryById(libraryId, { identify, subpath }),
    onSuccess: (_, { libraryId }) => {
      invalidateLibraryCatalogQueries(queryClient, libraryId);
    },
  });
}

export function useIdentifyLibrary(): UseMutationResult<
  IdentifyResult,
  Error,
  { libraryId: number; signal?: AbortSignal }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ libraryId, signal }) => identifyLibrary(libraryId, { signal }),
    onSuccess: (_, { libraryId }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.library(libraryId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.libraries });
      void queryClient.invalidateQueries({ queryKey: queryKeys.unidentifiedSummary });
    },
  });
}

export function useUpdateLibraryPlaybackPreferences(): UseMutationResult<
  Library,
  Error,
  { libraryId: number; payload: UpdateLibraryPlaybackPreferencesPayload }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ libraryId, payload }) => updateLibraryPlaybackPreferences(libraryId, payload),
    onSuccess: (library) => {
      queryClient.setQueryData<Library[]>(
        queryKeys.libraries,
        (current) =>
          current?.map((item) => (item.id === library.id ? { ...item, ...library } : item)) ?? [
            { ...library },
          ],
      );
    },
  });
}

export function useSeries(
  tmdbId: number | null,
  options?: { enabled?: boolean },
): UseQueryResult<SeriesDetails | null, Error> {
  return useQuery({
    queryKey: queryKeys.series(tmdbId ?? 0),
    queryFn: async () => contractsView<SeriesDetails | null>(await fetchSeriesByTmdbId(tmdbId!)),
    enabled: (options?.enabled ?? true) && tmdbId != null && tmdbId > 0,
    staleTime: METADATA_DETAIL_STALE_MS,
  });
}

export function useMovieDetails(
  libraryId: number | null,
  mediaId: number | null,
  options?: { enabled?: boolean },
): UseQueryResult<MovieDetailsPage | null, Error> {
  return useQuery({
    queryKey: queryKeys.movieDetails(libraryId ?? 0, mediaId ?? 0),
    queryFn: async () =>
      contractsView<MovieDetails | null>(await getMovieDetails(libraryId!, mediaId!)),
    select: selectMovieDetailsForPage,
    enabled: (options?.enabled ?? true) && libraryId != null && mediaId != null && mediaId > 0,
    staleTime: METADATA_DETAIL_STALE_MS,
  });
}

export function useShowDetails(
  libraryId: number | null,
  showKey: string | null,
  options?: { enabled?: boolean },
): UseQueryResult<ShowDetailsPage | null, Error> {
  return useQuery({
    queryKey: queryKeys.showDetails(libraryId ?? 0, showKey ?? ""),
    queryFn: async () =>
      contractsView<ShowDetails | null>(await getShowDetails(libraryId!, showKey!)),
    select: selectShowDetailsForPage,
    enabled: (options?.enabled ?? true) && libraryId != null && Boolean(showKey),
    staleTime: METADATA_DETAIL_STALE_MS,
  });
}

export function useShowEpisodes(
  libraryId: number | null,
  showKey: string | null,
  options?: { enabled?: boolean },
): UseQueryResult<ShowEpisodesResponse, Error> {
  return useQuery({
    queryKey: queryKeys.showEpisodes(libraryId ?? 0, showKey ?? ""),
    queryFn: async () =>
      contractsView<ShowEpisodesResponse>(await getShowEpisodes(libraryId!, showKey!)),
    enabled: (options?.enabled ?? true) && libraryId != null && Boolean(showKey),
    staleTime: SCAN_SENSITIVE_STALE_MS,
  });
}

export function useMoviePosterCandidates(
  libraryId: number | null,
  mediaId: number | null,
  options?: { enabled?: boolean },
): UseQueryResult<PosterCandidatesResponse, Error> {
  return useQuery({
    queryKey: queryKeys.moviePosterCandidates(libraryId ?? 0, mediaId ?? 0),
    queryFn: async () =>
      contractsView<PosterCandidatesResponse>(
        await getMoviePosterCandidates(libraryId!, mediaId!),
      ),
    enabled: (options?.enabled ?? true) && libraryId != null && mediaId != null && mediaId > 0,
    staleTime: METADATA_DETAIL_STALE_MS,
  });
}

export function useShowPosterCandidates(
  libraryId: number | null,
  showKey: string | null,
  options?: { enabled?: boolean },
): UseQueryResult<PosterCandidatesResponse, Error> {
  return useQuery({
    queryKey: queryKeys.showPosterCandidates(libraryId ?? 0, showKey ?? ""),
    queryFn: async () =>
      contractsView<PosterCandidatesResponse>(
        await getShowPosterCandidates(libraryId!, showKey!),
      ),
    enabled: (options?.enabled ?? true) && libraryId != null && Boolean(showKey),
    staleTime: METADATA_DETAIL_STALE_MS,
  });
}

export function useLibrarySearch(
  query: string,
  options?: {
    enabled?: boolean;
    libraryId?: number | null;
    type?: "movie" | "show" | "";
    genre?: string;
    limit?: number;
  },
): UseQueryResult<SearchResponse, Error> {
  const normalizedQuery = query.trim();
  const normalizedType = options?.type ?? "";
  const normalizedGenre = options?.genre?.trim() ?? "";
  const normalizedLibraryId = options?.libraryId ?? null;
  return useQuery({
    queryKey: queryKeys.search(
      normalizedLibraryId ?? 0,
      normalizedQuery,
      normalizedType,
      normalizedGenre,
    ),
    queryFn: async () =>
      contractsView<SearchResponse>(
        await searchLibraryMedia(normalizedQuery, {
          libraryId: normalizedLibraryId ?? undefined,
          type: normalizedType === "" ? undefined : normalizedType,
          genre: normalizedGenre || undefined,
          limit: options?.limit,
        }),
      ),
    enabled: (options?.enabled ?? true) && normalizedQuery.length >= 2,
    staleTime: SCAN_SENSITIVE_STALE_MS,
  });
}

export function useRefreshShow(): UseMutationResult<
  ShowActionResult,
  Error,
  { libraryId: number; showKey: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ libraryId, showKey }) => refreshShow(libraryId, showKey),
    onSuccess: (_, { libraryId, showKey }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.library(libraryId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.showDetails(libraryId, showKey) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.showEpisodes(libraryId, showKey) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.unidentifiedSummary });
      invalidateSearchAfterLibraryDataChange(queryClient, libraryId);
    },
  });
}

/** Re-probes every item in the library (same as per-title refresh, server-side). */
export function useRefreshLibraryPlaybackTracks(): UseMutationResult<
  LibraryPlaybackTracksRefreshResult,
  Error,
  { libraryId: number }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ libraryId }) => refreshLibraryPlaybackTracks(libraryId),
    onSuccess: (_, { libraryId }) => {
      invalidateLibraryCatalogQueries(queryClient, libraryId);
      invalidateSearchAfterLibraryDataChange(queryClient, libraryId);
    },
  });
}

/** Re-probes each file (ffprobe), rescans sidecar subtitles, and persists embedded audio/subtitle streams. */
export function useRefreshPlaybackTrackMetadata(): UseMutationResult<
  void,
  Error,
  { libraryId: number; mediaIds: number[] }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ mediaIds }) => {
      for (const id of mediaIds) {
        await refreshPlaybackTracks(id);
      }
    },
    onSuccess: (_, { libraryId }) => {
      invalidateLibraryCatalogQueries(queryClient, libraryId);
      invalidateSearchAfterLibraryDataChange(queryClient, libraryId);
    },
  });
}

export function useConfirmShow(): UseMutationResult<
  ShowActionResult,
  Error,
  { libraryId: number; showKey: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ libraryId, showKey }) => confirmShow(libraryId, { showKey }),
    onSuccess: (_, { libraryId, showKey }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.library(libraryId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.showDetails(libraryId, showKey) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.showEpisodes(libraryId, showKey) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.unidentifiedSummary });
      invalidateSearchAfterLibraryDataChange(queryClient, libraryId);
    },
  });
}

export function useTranscodingSettings(options?: {
  enabled?: boolean;
}): UseQueryResult<TranscodingSettingsResponse, Error> {
  return useQuery({
    queryKey: queryKeys.transcodingSettings,
    queryFn: async () =>
      contractsView<TranscodingSettingsResponse>(await getTranscodingSettings()),
    enabled: options?.enabled ?? true,
    staleTime: SERVER_SETTINGS_STALE_MS,
  });
}

export function useMetadataArtworkSettings(options?: {
  enabled?: boolean;
}): UseQueryResult<MetadataArtworkSettingsResponse, Error> {
  return useQuery({
    queryKey: queryKeys.metadataArtworkSettings,
    queryFn: async () =>
      contractsView<MetadataArtworkSettingsResponse>(await getMetadataArtworkSettings()),
    enabled: options?.enabled ?? true,
    staleTime: SERVER_SETTINGS_STALE_MS,
  });
}

export function useMediaStackSettings(options?: {
  enabled?: boolean;
}): UseQueryResult<MediaStackSettings, Error> {
  return useQuery({
    queryKey: queryKeys.mediaStackSettings,
    queryFn: async () => contractsView<MediaStackSettings>(await getMediaStackSettings()),
    enabled: options?.enabled ?? true,
    staleTime: SERVER_SETTINGS_STALE_MS,
  });
}

export function useServerEnvSettings(options?: {
  enabled?: boolean;
}): UseQueryResult<ServerEnvSettingsResponse, Error> {
  return useQuery({
    queryKey: queryKeys.serverEnvSettings,
    queryFn: async () =>
      contractsView<ServerEnvSettingsResponse>(await getServerEnvSettings()),
    enabled: options?.enabled ?? true,
    staleTime: SERVER_SETTINGS_STALE_MS,
  });
}

export function useUpdateTranscodingSettings(): UseMutationResult<
  TranscodingSettingsResponse,
  Error,
  TranscodingSettings
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (settings) =>
      contractsView<TranscodingSettingsResponse>(await updateTranscodingSettings(settings)),
    onSuccess: (data) => {
      queryClient.setQueryData(queryKeys.transcodingSettings, data);
    },
  });
}

export function useUpdateMetadataArtworkSettings(): UseMutationResult<
  MetadataArtworkSettingsResponse,
  Error,
  MetadataArtworkSettings
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (settings) =>
      contractsView<MetadataArtworkSettingsResponse>(
        await updateMetadataArtworkSettings(settings),
      ),
    onSuccess: (data) => {
      queryClient.setQueryData(queryKeys.metadataArtworkSettings, data);
    },
  });
}

export function useUpdateMediaStackSettings(): UseMutationResult<
  MediaStackSettings,
  Error,
  MediaStackSettings
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: updateMediaStackSettings,
    onSuccess: (data) => {
      queryClient.setQueryData(queryKeys.mediaStackSettings, data);
      invalidateDiscoverRelatedQueries(queryClient);
      void queryClient.invalidateQueries({ queryKey: queryKeys.downloads });
    },
  });
}

export function useValidateMediaStackSettings(): UseMutationResult<
  MediaStackValidationResult,
  Error,
  MediaStackSettings
> {
  return useMutation({
    mutationFn: async (settings) =>
      contractsView<MediaStackValidationResult>(await validateMediaStackSettings(settings)),
  });
}

export function useUpdateServerEnvSettings(): UseMutationResult<
  ServerEnvSettingsResponse,
  Error,
  ServerEnvSettingsUpdate
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (payload) =>
      contractsView<ServerEnvSettingsResponse>(await updateServerEnvSettings(payload)),
    onSuccess: (data) => {
      queryClient.setQueryData(queryKeys.serverEnvSettings, data);
      invalidateDiscoverRelatedQueries(queryClient);
      void queryClient.invalidateQueries({ queryKey: queryKeys.metadataArtworkSettings });
    },
  });
}

export function useAddDiscoverTitle(): UseMutationResult<
  DiscoverAcquisition,
  Error,
  { mediaType: DiscoverMediaType; tmdbId: number }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ mediaType, tmdbId }) =>
      contractsView<DiscoverAcquisition>(await addDiscoverTitle(mediaType, tmdbId)),
    onSuccess: () => {
      invalidateDiscoverRelatedQueries(queryClient);
      void queryClient.invalidateQueries({ queryKey: queryKeys.downloads });
    },
  });
}

export function useSetMoviePosterSelection(): UseMutationResult<
  void,
  Error,
  { libraryId: number; mediaId: number; sourceUrl: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ libraryId, mediaId, sourceUrl }) =>
      setMoviePosterSelection(libraryId, mediaId, { source_url: sourceUrl }),
    onSuccess: (_, { libraryId, mediaId }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.library(libraryId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.movieDetails(libraryId, mediaId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.moviePosterCandidates(libraryId, mediaId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.home });
      invalidateSearchAfterLibraryDataChange(queryClient, libraryId);
    },
  });
}

export function useResetMoviePosterSelection(): UseMutationResult<
  void,
  Error,
  { libraryId: number; mediaId: number }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ libraryId, mediaId }) => resetMoviePosterSelection(libraryId, mediaId),
    onSuccess: (_, { libraryId, mediaId }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.library(libraryId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.movieDetails(libraryId, mediaId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.moviePosterCandidates(libraryId, mediaId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.home });
      invalidateSearchAfterLibraryDataChange(queryClient, libraryId);
    },
  });
}

export function useSetShowPosterSelection(): UseMutationResult<
  void,
  Error,
  { libraryId: number; showKey: string; sourceUrl: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ libraryId, showKey, sourceUrl }) =>
      setShowPosterSelection(libraryId, showKey, { source_url: sourceUrl }),
    onSuccess: (_, { libraryId, showKey }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.library(libraryId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.showDetails(libraryId, showKey) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.showEpisodes(libraryId, showKey) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.showPosterCandidates(libraryId, showKey) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.home });
      invalidateSearchAfterLibraryDataChange(queryClient, libraryId);
    },
  });
}

export function useResetShowPosterSelection(): UseMutationResult<
  void,
  Error,
  { libraryId: number; showKey: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ libraryId, showKey }) => resetShowPosterSelection(libraryId, showKey),
    onSuccess: (_, { libraryId, showKey }) => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.library(libraryId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.showDetails(libraryId, showKey) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.showEpisodes(libraryId, showKey) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.showPosterCandidates(libraryId, showKey) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.home });
      invalidateSearchAfterLibraryDataChange(queryClient, libraryId);
    },
  });
}

export function useIntroScanSummary(): UseQueryResult<IntroScanSummaryResponse, Error> {
  return useQuery({
    queryKey: queryKeys.introScanSummary,
    queryFn: async () =>
      contractsView<IntroScanSummaryResponse>(await getIntroScanSummary()),
    staleTime: SCAN_SENSITIVE_STALE_MS,
  });
}

export function useIntroScanShowSummary(
  libraryId: number | null,
): UseQueryResult<IntroScanShowSummaryResponse, Error> {
  return useQuery({
    queryKey: queryKeys.introScanShows(libraryId ?? 0),
    queryFn: async () =>
      contractsView<IntroScanShowSummaryResponse>(
        await getIntroScanShowSummary(libraryId!),
      ),
    enabled: libraryId != null,
    staleTime: SCAN_SENSITIVE_STALE_MS,
  });
}

export function useIntroRefreshStatus(
  enabled: boolean,
): UseQueryResult<IntroRefreshStatusResponse, Error> {
  return useQuery({
    queryKey: queryKeys.introRefreshStatus,
    queryFn: async () =>
      contractsView<IntroRefreshStatusResponse>(
        await getIntroRefreshStatus(),
      ),
    enabled,
    refetchInterval: 1000,
    staleTime: 0,
  });
}

export function useRefreshLibraryIntros(): UseMutationResult<
  LibraryPlaybackTracksRefreshResult,
  Error,
  { libraryId: number }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ libraryId }) =>
      contractsView<LibraryPlaybackTracksRefreshResult>(
        await refreshLibraryPlaybackTracks(libraryId),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.introScanSummary });
    },
  });
}
