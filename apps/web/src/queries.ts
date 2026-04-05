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

/** Schema-decoded API payloads are deeply readonly; cache entries use mutable view types. */
function decodeAs<T>(value: unknown): T {
  return value as T;
}

type LibraryMediaPageResult = Exclude<Awaited<ReturnType<typeof fetchLibraryMedia>>, MediaItem[]>;
type HomeDashboardResult = Awaited<ReturnType<typeof getHomeDashboard>>;

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

function buildHomeDashboard(dashboard: HomeDashboardResult): HomeDashboard {
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
  discover: ["discover"] as const,
  discoverBrowse: (
    category: DiscoverBrowseCategory | "",
    mediaType: DiscoverMediaType | "",
    genreId: number | null,
  ) => ["discover-browse", category, mediaType, genreId ?? 0] as const,
  discoverGenres: ["discover-genres"] as const,
  discoverSearch: (query: string) => ["discover-search", query] as const,
  discoverTitle: (mediaType: DiscoverMediaType, tmdbId: number) =>
    ["discover-title", mediaType, tmdbId] as const,
  downloads: ["downloads"] as const,
  home: ["home"] as const,
  libraries: ["libraries"] as const,
  unidentifiedSummary: ["libraries", "unidentified-summary"] as const,
  library: (id: number, pageSize?: number) =>
    pageSize == null ? (["library", id] as const) : (["library", id, pageSize] as const),
  movieDetails: (libraryId: number, mediaId: number) => ["movie-details", libraryId, mediaId] as const,
  moviePosterCandidates: (libraryId: number, mediaId: number) =>
    ["movie-poster-candidates", libraryId, mediaId] as const,
  metadataArtworkSettings: ["metadata-artwork-settings"] as const,
  mediaStackSettings: ["media-stack-settings"] as const,
  serverEnvSettings: ["server-env-settings"] as const,
  search: (query: string, libraryId: number | null, type: string, genre: string) =>
    ["search", query, libraryId ?? 0, type, genre] as const,
  series: (tmdbId: number) => ["series", tmdbId] as const,
  showPosterCandidates: (libraryId: number, showKey: string) =>
    ["show-poster-candidates", libraryId, showKey] as const,
  showDetails: (libraryId: number, showKey: string) => ["show-details", libraryId, showKey] as const,
  showEpisodes: (libraryId: number, showKey: string) => ["library", libraryId, "show-episodes", showKey] as const,
  transcodingSettings: ["transcoding-settings"] as const,
};

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
  void queryClient.invalidateQueries({ queryKey: ["show-details", libraryId] });
  void queryClient.invalidateQueries({ queryKey: ["movie-details", libraryId] });
}

/** Refetch Discover shelves, search, and title detail queries (e.g. after downloads or library scans). */
export function invalidateDiscoverRelatedQueries(queryClient: QueryClient): void {
  void queryClient.invalidateQueries({ queryKey: queryKeys.discover });
  void queryClient.invalidateQueries({ queryKey: ["discover-browse"] });
  void queryClient.invalidateQueries({ queryKey: queryKeys.discoverGenres });
  void queryClient.invalidateQueries({ queryKey: ["discover-search"] });
  void queryClient.invalidateQueries({ queryKey: ["discover-title"] });
}

const LIBRARIES_STALE_MS = 60 * 1000;
const LIBRARY_MEDIA_STALE_MS = 60 * 1000;
const DISCOVER_STALE_MS = 5 * 60 * 1000;

export function useLibraries(): UseQueryResult<Library[], Error> {
  return useQuery({
    queryKey: queryKeys.libraries,
    queryFn: async () => decodeAs<Library[]>(await listLibraries()),
    staleTime: LIBRARIES_STALE_MS,
  });
}

export function useUnidentifiedLibrarySummaries(): UseQueryResult<
  UnidentifiedLibrariesResponse,
  Error
> {
  return useQuery({
    queryKey: queryKeys.unidentifiedSummary,
    queryFn: async () => decodeAs<UnidentifiedLibrariesResponse>(await getUnidentifiedLibrarySummaries()),
    staleTime: LIBRARIES_STALE_MS,
  });
}

export function useDiscover(options?: {
  enabled?: boolean;
  refetchInterval?: number | false;
}): UseQueryResult<DiscoverResponse, Error> {
  return useQuery({
    queryKey: queryKeys.discover,
    queryFn: async () => decodeAs<DiscoverResponse>(await getDiscover()),
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
    queryFn: async () => decodeAs<DiscoverGenresResponse>(await getDiscoverGenres()),
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
    enabled?: boolean;
    refetchInterval?: number | false;
  },
) {
  const category = options.category ?? "";
  const mediaType = options.mediaType ?? "";
  const genreId = options.genreId ?? null;
  return useInfiniteQuery({
    queryKey: queryKeys.discoverBrowse(category, mediaType, genreId),
    queryFn: async ({ pageParam }) =>
      decodeAs<DiscoverBrowseResponse>(
        await browseDiscover({
          category: category === "" ? undefined : category,
          mediaType: mediaType === "" ? undefined : mediaType,
          genreId: genreId ?? undefined,
          page: Number(pageParam ?? 1),
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
    queryFn: async () => decodeAs<DiscoverSearchResponse>(await searchDiscover(normalizedQuery)),
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
      decodeAs<DiscoverTitleDetails | null>(await getDiscoverTitleDetails(mediaType!, tmdbId!)),
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
    queryFn: async () => decodeAs<DownloadsResponse>(await getDownloads()),
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
    queryFn: async () => buildHomeDashboard(await getHomeDashboard()),
    enabled: options?.enabled ?? true,
    staleTime: LIBRARY_MEDIA_STALE_MS,
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
      normalizeLibraryMediaPage(
        await fetchLibraryMedia(libraryId!, { offset: Number(pageParam ?? 0), limit: pageSize }),
      ),
    enabled: (options?.enabled ?? true) && libraryId != null,
    initialPageParam: 0,
    getNextPageParam: (lastPage) => lastPage.next_offset,
    refetchInterval: options?.refetchInterval,
    staleTime: LIBRARY_MEDIA_STALE_MS,
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

const SERIES_STALE_MS = 5 * 60 * 1000;

export function useSeries(
  tmdbId: number | null,
  options?: { enabled?: boolean },
): UseQueryResult<SeriesDetails | null, Error> {
  return useQuery({
    queryKey: queryKeys.series(tmdbId ?? 0),
    queryFn: async () => decodeAs<SeriesDetails | null>(await fetchSeriesByTmdbId(tmdbId!)),
    enabled: (options?.enabled ?? true) && tmdbId != null && tmdbId > 0,
    staleTime: SERIES_STALE_MS,
  });
}

export function useMovieDetails(
  libraryId: number | null,
  mediaId: number | null,
  options?: { enabled?: boolean },
): UseQueryResult<MovieDetails | null, Error> {
  return useQuery({
    queryKey: queryKeys.movieDetails(libraryId ?? 0, mediaId ?? 0),
    queryFn: async () => decodeAs<MovieDetails | null>(await getMovieDetails(libraryId!, mediaId!)),
    enabled: (options?.enabled ?? true) && libraryId != null && mediaId != null && mediaId > 0,
    staleTime: SERIES_STALE_MS,
  });
}

export function useShowDetails(
  libraryId: number | null,
  showKey: string | null,
  options?: { enabled?: boolean },
): UseQueryResult<ShowDetails | null, Error> {
  return useQuery({
    queryKey: queryKeys.showDetails(libraryId ?? 0, showKey ?? ""),
    queryFn: async () => decodeAs<ShowDetails | null>(await getShowDetails(libraryId!, showKey!)),
    enabled: (options?.enabled ?? true) && libraryId != null && Boolean(showKey),
    staleTime: SERIES_STALE_MS,
  });
}

export function useShowEpisodes(
  libraryId: number | null,
  showKey: string | null,
  options?: { enabled?: boolean },
): UseQueryResult<ShowEpisodesResponse, Error> {
  return useQuery({
    queryKey: queryKeys.showEpisodes(libraryId ?? 0, showKey ?? ""),
    queryFn: async () => decodeAs<ShowEpisodesResponse>(await getShowEpisodes(libraryId!, showKey!)),
    enabled: (options?.enabled ?? true) && libraryId != null && Boolean(showKey),
    staleTime: LIBRARY_MEDIA_STALE_MS,
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
      decodeAs<PosterCandidatesResponse>(await getMoviePosterCandidates(libraryId!, mediaId!)),
    enabled: (options?.enabled ?? true) && libraryId != null && mediaId != null && mediaId > 0,
    staleTime: 30_000,
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
      decodeAs<PosterCandidatesResponse>(await getShowPosterCandidates(libraryId!, showKey!)),
    enabled: (options?.enabled ?? true) && libraryId != null && Boolean(showKey),
    staleTime: 30_000,
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
      normalizedQuery,
      normalizedLibraryId,
      normalizedType,
      normalizedGenre,
    ),
    queryFn: async () =>
      decodeAs<SearchResponse>(
        await searchLibraryMedia(normalizedQuery, {
          libraryId: normalizedLibraryId ?? undefined,
          type: normalizedType === "" ? undefined : normalizedType,
          genre: normalizedGenre || undefined,
          limit: options?.limit,
        }),
      ),
    enabled: (options?.enabled ?? true) && normalizedQuery.length >= 2,
    staleTime: 30_000,
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
      void queryClient.invalidateQueries({ queryKey: ["search"] });
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
      void queryClient.invalidateQueries({ queryKey: ["search"] });
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
      void queryClient.invalidateQueries({ queryKey: ["search"] });
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
      void queryClient.invalidateQueries({ queryKey: ["search"] });
    },
  });
}

export function useTranscodingSettings(options?: {
  enabled?: boolean;
}): UseQueryResult<TranscodingSettingsResponse, Error> {
  return useQuery({
    queryKey: queryKeys.transcodingSettings,
    queryFn: async () => decodeAs<TranscodingSettingsResponse>(await getTranscodingSettings()),
    enabled: options?.enabled ?? true,
    staleTime: 30_000,
  });
}

export function useMetadataArtworkSettings(options?: {
  enabled?: boolean;
}): UseQueryResult<MetadataArtworkSettingsResponse, Error> {
  return useQuery({
    queryKey: queryKeys.metadataArtworkSettings,
    queryFn: async () =>
      decodeAs<MetadataArtworkSettingsResponse>(await getMetadataArtworkSettings()),
    enabled: options?.enabled ?? true,
    staleTime: 30_000,
  });
}

export function useMediaStackSettings(options?: {
  enabled?: boolean;
}): UseQueryResult<MediaStackSettings, Error> {
  return useQuery({
    queryKey: queryKeys.mediaStackSettings,
    queryFn: async () => decodeAs<MediaStackSettings>(await getMediaStackSettings()),
    enabled: options?.enabled ?? true,
    staleTime: 30_000,
  });
}

export function useServerEnvSettings(options?: {
  enabled?: boolean;
}): UseQueryResult<ServerEnvSettingsResponse, Error> {
  return useQuery({
    queryKey: queryKeys.serverEnvSettings,
    queryFn: async () => decodeAs<ServerEnvSettingsResponse>(await getServerEnvSettings()),
    enabled: options?.enabled ?? true,
    staleTime: 15_000,
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
      decodeAs<TranscodingSettingsResponse>(await updateTranscodingSettings(settings)),
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
      decodeAs<MetadataArtworkSettingsResponse>(await updateMetadataArtworkSettings(settings)),
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
      decodeAs<MediaStackValidationResult>(await validateMediaStackSettings(settings)),
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
      decodeAs<ServerEnvSettingsResponse>(await updateServerEnvSettings(payload)),
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
      decodeAs<DiscoverAcquisition>(await addDiscoverTitle(mediaType, tmdbId)),
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
      void queryClient.invalidateQueries({ queryKey: ["search"] });
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
      void queryClient.invalidateQueries({ queryKey: ["search"] });
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
      void queryClient.invalidateQueries({ queryKey: ["search"] });
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
      void queryClient.invalidateQueries({ queryKey: ["search"] });
    },
  });
}
