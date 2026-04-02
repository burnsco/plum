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
  getDiscover,
  getDiscoverGenres,
  getDiscoverTitleDetails,
  getMovieDetails,
  getMoviePosterCandidates,
  getMetadataArtworkSettings,
  getMediaStackSettings,
  getShowDetails,
  getShowEpisodes,
  getShowPosterCandidates,
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
  refreshShow,
  scanLibraryById,
  type DiscoverAcquisition,
  type DiscoverBrowseCategory,
  type DiscoverBrowseResponse,
  type DiscoverGenre,
  type DiscoverGenresResponse,
  type DiscoverLibraryMatch,
  type DiscoverMediaType,
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
  resetMoviePosterSelection,
  resetShowPosterSelection,
  setMoviePosterSelection,
  setShowPosterSelection,
  type SeriesDetails,
  type ShowActionResult,
  type ShowDetails,
  type ShowEpisodesResponse,
  type TranscodingSettings,
  type TranscodingSettingsResponse,
  type UpdateLibraryPlaybackPreferencesPayload,
  updateLibraryPlaybackPreferences,
  updateMetadataArtworkSettings,
  updateMediaStackSettings,
  updateTranscodingSettings,
  validateMediaStackSettings,
} from "./api";

type LibrariesResult = Awaited<ReturnType<typeof listLibraries>>;
type DiscoverResult = Awaited<ReturnType<typeof getDiscover>>;
type DiscoverBrowseResult = Awaited<ReturnType<typeof browseDiscover>>;
type DiscoverGenresResult = Awaited<ReturnType<typeof getDiscoverGenres>>;
type DiscoverSearchResult = Awaited<ReturnType<typeof searchDiscover>>;
type DiscoverTitleDetailsResult = Awaited<ReturnType<typeof getDiscoverTitleDetails>>;
type DownloadsResult = Awaited<ReturnType<typeof getDownloads>>;
type LibraryMediaPageResult = Exclude<Awaited<ReturnType<typeof fetchLibraryMedia>>, MediaItem[]>;
type HomeDashboardResult = Awaited<ReturnType<typeof getHomeDashboard>>;
type MovieDetailsResult = Awaited<ReturnType<typeof getMovieDetails>>;
type MoviePosterCandidatesResult = Awaited<ReturnType<typeof getMoviePosterCandidates>>;
type MetadataArtworkSettingsResult = Awaited<ReturnType<typeof getMetadataArtworkSettings>>;
type MediaStackSettingsResult = Awaited<ReturnType<typeof getMediaStackSettings>>;
type ShowDetailsResult = Awaited<ReturnType<typeof getShowDetails>>;
type ShowEpisodesResult = Awaited<ReturnType<typeof getShowEpisodes>>;
type ShowPosterCandidatesResult = Awaited<ReturnType<typeof getShowPosterCandidates>>;
type SearchLibraryMediaResult = Awaited<ReturnType<typeof searchLibraryMedia>>;
type TranscodingSettingsResult = Awaited<ReturnType<typeof getTranscodingSettings>>;
type HomeDashboardMediaResult = HomeDashboardResult["continueWatching"][number]["media"];

function cloneLibrary(library: LibrariesResult[number]): Library {
  return { ...library };
}

function cloneDiscoverLibraryMatch(match: DiscoverLibraryMatch): DiscoverLibraryMatch {
  return { ...match };
}

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

function cloneMediaItem(item: HomeDashboardMediaResult): MediaItem {
  return {
    ...item,
    subtitles: item.subtitles?.map((subtitle) => ({ ...subtitle })),
    embeddedSubtitles: item.embeddedSubtitles?.map((subtitle) => ({ ...subtitle })),
    embeddedAudioTracks: item.embeddedAudioTracks?.map((track) => ({ ...track })),
  };
}

function cloneDiscoverAcquisition(acquisition: DiscoverAcquisition): DiscoverAcquisition {
  return { ...acquisition };
}

function cloneDiscoverItem(item: DiscoverResult["shelves"][number]["items"][number]) {
  return {
    ...item,
    library_matches: item.library_matches?.map(cloneDiscoverLibraryMatch),
    acquisition: item.acquisition ? cloneDiscoverAcquisition(item.acquisition) : undefined,
  };
}

function cloneDiscoverResponse(response: DiscoverResult): DiscoverResponse {
  return {
    shelves: response.shelves.map((shelf) => ({
      ...shelf,
      items: shelf.items.map(cloneDiscoverItem),
    })),
  };
}

function cloneDiscoverGenre(genre: DiscoverGenre): DiscoverGenre {
  return { ...genre };
}

function cloneDiscoverGenresResponse(response: DiscoverGenresResult): DiscoverGenresResponse {
  return {
    movie_genres: response.movie_genres.map(cloneDiscoverGenre),
    tv_genres: response.tv_genres.map(cloneDiscoverGenre),
  };
}

function cloneDiscoverBrowseResponse(response: DiscoverBrowseResult): DiscoverBrowseResponse {
  return {
    items: response.items.map(cloneDiscoverItem),
    page: response.page,
    total_pages: response.total_pages,
    total_results: response.total_results,
    media_type: response.media_type,
    genre: response.genre ? cloneDiscoverGenre(response.genre) : undefined,
    category: response.category,
  };
}

function cloneDiscoverSearchResponse(response: DiscoverSearchResult): DiscoverSearchResponse {
  return {
    movies: response.movies.map(cloneDiscoverItem),
    tv: response.tv.map(cloneDiscoverItem),
  };
}

function cloneDiscoverTitleDetails(
  details: DiscoverTitleDetailsResult,
): DiscoverTitleDetails | null {
  if (details == null) {
    return null;
  }
  return {
    ...details,
    genres: [...details.genres],
    videos: details.videos.map((video) => ({ ...video })),
    library_matches: details.library_matches?.map(cloneDiscoverLibraryMatch),
    acquisition: details.acquisition ? cloneDiscoverAcquisition(details.acquisition) : undefined,
  };
}

function cloneDownloadsResponse(response: DownloadsResult): DownloadsResponse {
  return {
    configured: response.configured,
    items: response.items.map((item) => ({ ...item })),
  };
}

function cloneMediaStackSettings(settings: MediaStackSettingsResult): MediaStackSettings {
  return {
    radarr: { ...settings.radarr },
    sonarrTv: { ...settings.sonarrTv },
  };
}

function cloneMediaStackValidationResult(
  result: Awaited<ReturnType<typeof validateMediaStackSettings>>,
): MediaStackValidationResult {
  return {
    radarr: {
      ...result.radarr,
      rootFolders: result.radarr.rootFolders.map((folder) => ({ ...folder })),
      qualityProfiles: result.radarr.qualityProfiles.map((profile) => ({ ...profile })),
    },
    sonarrTv: {
      ...result.sonarrTv,
      rootFolders: result.sonarrTv.rootFolders.map((folder) => ({ ...folder })),
      qualityProfiles: result.sonarrTv.qualityProfiles.map((profile) => ({ ...profile })),
    },
  };
}

function cloneMovieDetails(details: MovieDetailsResult): MovieDetails | null {
  if (details == null) {
    return null;
  }
  return {
    ...details,
    subtitles: details.subtitles?.map((subtitle) => ({ ...subtitle })),
    embeddedSubtitles: details.embeddedSubtitles?.map((subtitle) => ({ ...subtitle })),
    embeddedAudioTracks: details.embeddedAudioTracks?.map((track) => ({ ...track })),
    genres: [...details.genres],
    cast: details.cast.map((member) => ({ ...member })),
  };
}

function cloneShowDetails(details: ShowDetailsResult): ShowDetails | null {
  if (details == null) {
    return null;
  }
  return {
    ...details,
    genres: [...details.genres],
    cast: details.cast.map((member) => ({ ...member })),
  };
}

function cloneShowEpisodes(response: ShowEpisodesResult): ShowEpisodesResponse {
  return {
    seasons: response.seasons.map((s) => ({
      seasonNumber: s.seasonNumber,
      label: s.label,
      episodes: s.episodes.map((e) => ({ ...e })),
    })),
  };
}

function cloneSeriesDetails(details: Awaited<ReturnType<typeof fetchSeriesByTmdbId>>): SeriesDetails | null {
  if (details == null) {
    return null;
  }
  return {
    ...details,
    genres: [...details.genres],
    cast: details.cast.map((member) => ({ ...member })),
  };
}

function cloneSearchResponse(response: SearchLibraryMediaResult): SearchResponse {
  return {
    ...response,
    results: response.results.map((result) => ({
      ...result,
      genres: result.genres ? [...result.genres] : undefined,
    })),
    facets: {
      libraries: response.facets.libraries.map((facet) => ({ ...facet })),
      types: response.facets.types.map((facet) => ({ ...facet })),
      genres: response.facets.genres.map((facet) => ({ ...facet })),
    },
  };
}

function cloneHomeDashboard(dashboard: HomeDashboardResult): HomeDashboard {
  return {
    ...dashboard,
    continueWatching: dashboard.continueWatching.map((entry) => ({
      ...entry,
      media: cloneMediaItem(entry.media),
    })),
    recentlyAdded: (dashboard.recentlyAdded ?? []).map((entry) => ({
      ...entry,
      media: cloneMediaItem(entry.media),
    })),
  };
}

function cloneTranscodingSettingsResponse(
  response: TranscodingSettingsResult,
): TranscodingSettingsResponse {
  return {
    settings: {
      ...response.settings,
      decodeCodecs: { ...response.settings.decodeCodecs },
      encodeFormats: { ...response.settings.encodeFormats },
    },
    warnings: response.warnings.map((warning) => ({ ...warning })),
  };
}

function cloneMetadataArtworkSettingsResponse(
  response: MetadataArtworkSettingsResult,
): MetadataArtworkSettingsResponse {
  return {
    settings: {
      movies: { ...response.settings.movies },
      shows: { ...response.settings.shows },
      seasons: { ...response.settings.seasons },
      episodes: { ...response.settings.episodes },
    },
    provider_availability: response.provider_availability.map((provider) => ({ ...provider })),
  };
}

function clonePosterCandidatesResponse(
  response: MoviePosterCandidatesResult | ShowPosterCandidatesResult,
): PosterCandidatesResponse {
  return {
    candidates: response.candidates.map((candidate) => ({ ...candidate })),
    provider_availability: response.provider_availability.map((provider) => ({ ...provider })),
    has_custom_selection: response.has_custom_selection,
  };
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
  library: (id: number, pageSize?: number) =>
    pageSize == null ? (["library", id] as const) : (["library", id, pageSize] as const),
  movieDetails: (libraryId: number, mediaId: number) => ["movie-details", libraryId, mediaId] as const,
  moviePosterCandidates: (libraryId: number, mediaId: number) =>
    ["movie-poster-candidates", libraryId, mediaId] as const,
  metadataArtworkSettings: ["metadata-artwork-settings"] as const,
  mediaStackSettings: ["media-stack-settings"] as const,
  search: (query: string, libraryId: number | null, type: string, genre: string) =>
    ["search", query, libraryId ?? 0, type, genre] as const,
  series: (tmdbId: number) => ["series", tmdbId] as const,
  showPosterCandidates: (libraryId: number, showKey: string) =>
    ["show-poster-candidates", libraryId, showKey] as const,
  showDetails: (libraryId: number, showKey: string) => ["show-details", libraryId, showKey] as const,
  showEpisodes: (libraryId: number, showKey: string) => ["library", libraryId, "show-episodes", showKey] as const,
  transcodingSettings: ["transcoding-settings"] as const,
};

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
    queryFn: async () => (await listLibraries()).map(cloneLibrary),
    staleTime: LIBRARIES_STALE_MS,
  });
}

export function useDiscover(options?: {
  enabled?: boolean;
  refetchInterval?: number | false;
}): UseQueryResult<DiscoverResponse, Error> {
  return useQuery({
    queryKey: queryKeys.discover,
    queryFn: async () => cloneDiscoverResponse(await getDiscover()),
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
    queryFn: async () => cloneDiscoverGenresResponse(await getDiscoverGenres()),
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
      cloneDiscoverBrowseResponse(
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
    queryFn: async () => cloneDiscoverSearchResponse(await searchDiscover(normalizedQuery)),
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
    queryFn: async () => cloneDiscoverTitleDetails(await getDiscoverTitleDetails(mediaType!, tmdbId!)),
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
    queryFn: async () => cloneDownloadsResponse(await getDownloads()),
    enabled: options?.enabled ?? true,
    refetchInterval: options?.refetchInterval,
    staleTime: 5_000,
  });
}

export function useHomeDashboard(options?: {
  enabled?: boolean;
}): UseQueryResult<HomeDashboard, Error> {
  return useQuery({
    queryKey: queryKeys.home,
    queryFn: async () => cloneHomeDashboard(await getHomeDashboard()),
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
      void queryClient.invalidateQueries({ queryKey: queryKeys.library(libraryId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.libraries });
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
            cloneLibrary(library),
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
    queryFn: async () => cloneSeriesDetails(await fetchSeriesByTmdbId(tmdbId!)),
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
    queryFn: async () => cloneMovieDetails(await getMovieDetails(libraryId!, mediaId!)),
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
    queryFn: async () => cloneShowDetails(await getShowDetails(libraryId!, showKey!)),
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
    queryFn: async () => cloneShowEpisodes(await getShowEpisodes(libraryId!, showKey!)),
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
    queryFn: async () => clonePosterCandidatesResponse(await getMoviePosterCandidates(libraryId!, mediaId!)),
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
    queryFn: async () => clonePosterCandidatesResponse(await getShowPosterCandidates(libraryId!, showKey!)),
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
      cloneSearchResponse(
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
      void queryClient.invalidateQueries({ queryKey: ["search"] });
    },
  });
}

export function useTranscodingSettings(options?: {
  enabled?: boolean;
}): UseQueryResult<TranscodingSettingsResponse, Error> {
  return useQuery({
    queryKey: queryKeys.transcodingSettings,
    queryFn: async () => cloneTranscodingSettingsResponse(await getTranscodingSettings()),
    enabled: options?.enabled ?? true,
    staleTime: 30_000,
  });
}

export function useMetadataArtworkSettings(options?: {
  enabled?: boolean;
}): UseQueryResult<MetadataArtworkSettingsResponse, Error> {
  return useQuery({
    queryKey: queryKeys.metadataArtworkSettings,
    queryFn: async () => cloneMetadataArtworkSettingsResponse(await getMetadataArtworkSettings()),
    enabled: options?.enabled ?? true,
    staleTime: 30_000,
  });
}

export function useMediaStackSettings(options?: {
  enabled?: boolean;
}): UseQueryResult<MediaStackSettings, Error> {
  return useQuery({
    queryKey: queryKeys.mediaStackSettings,
    queryFn: async () => cloneMediaStackSettings(await getMediaStackSettings()),
    enabled: options?.enabled ?? true,
    staleTime: 30_000,
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
      cloneTranscodingSettingsResponse(await updateTranscodingSettings(settings)),
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
      cloneMetadataArtworkSettingsResponse(await updateMetadataArtworkSettings(settings)),
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
    mutationFn: async (settings) => cloneMediaStackSettings(await updateMediaStackSettings(settings)),
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
      cloneMediaStackValidationResult(await validateMediaStackSettings(settings)),
  });
}

export function useAddDiscoverTitle(): UseMutationResult<
  DiscoverAcquisition,
  Error,
  { mediaType: DiscoverMediaType; tmdbId: number }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ mediaType, tmdbId }) => addDiscoverTitle(mediaType, tmdbId),
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
