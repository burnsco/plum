import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import {
  confirmShow,
  fetchLibraryMedia,
  fetchSeriesByTmdbId,
  getHomeDashboard,
  getIntroScanShowSummary,
  getIntroScanSummary,
  getIntroRefreshStatus,
  getMovieDetails,
  getMoviePosterCandidates,
  getShowDetails,
  getShowEpisodes,
  getShowPosterCandidates,
  getUnidentifiedLibrarySummaries,
  identifyLibrary,
  listLibraries,
  refreshLibraryPlaybackTracks,
  refreshLibraryIntroOnly,
  postLibraryIntroChromaprintScan,
  refreshPlaybackTracks,
  refreshShow,
  resetMoviePosterSelection,
  resetShowPosterSelection,
  scanLibraryById,
  searchLibraryMedia,
  setMoviePosterSelection,
  setShowPosterSelection,
  type HomeDashboard,
  type IdentifyResult,
  type IntroRefreshStatusResponse,
  type IntroScanShowSummaryResponse,
  type IntroScanSummaryResponse,
  type Library,
  type LibraryPlaybackTracksRefreshResult,
  type MovieDetails,
  type PosterCandidatesResponse,
  type ScanLibraryResult,
  type SearchResponse,
  type SeriesDetails,
  type ShowActionResult,
  type ShowDetails,
  type ShowEpisodesResponse,
  type UnidentifiedLibrariesResponse,
  type UpdateLibraryPlaybackPreferencesPayload,
  updateLibraryPlaybackPreferences,
} from "@/api";
import {
  buildHomeDashboard,
  contractsView,
  invalidateLibraryCatalogQueries,
  invalidateSearchAfterLibraryDataChange,
  METADATA_DETAIL_STALE_MS,
  normalizeLibraryMediaPage,
  notifyMutationError,
  queryKeys,
  SCAN_SENSITIVE_STALE_MS,
  selectMovieDetailsForPage,
  selectShowDetailsForPage,
  stripLibraryBrowseMediaItem,
  type LibraryMediaPageResult,
  type MovieDetailsPage,
  type ShowDetailsPage,
} from "./shared";

export function useLibraries(): UseQueryResult<Library[], Error> {
  return useQuery({
    queryKey: queryKeys.libraries,
    queryFn: async () => contractsView<Library[]>(await listLibraries()),
    staleTime: SCAN_SENSITIVE_STALE_MS,
    gcTime: 15 * 60 * 1000,
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
    gcTime: 10 * 60 * 1000,
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
    gcTime: 5 * 60 * 1000,
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
    gcTime: 3 * 60 * 1000,
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
    onError: (err) => notifyMutationError(err, "Library scan failed"),
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
    onError: (err) => notifyMutationError(err, "Library identify failed"),
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
    onMutate: async ({ libraryId, payload }) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.libraries });
      const previous = queryClient.getQueryData<Library[]>(queryKeys.libraries);
      if (previous == null) return { previous };
      queryClient.setQueryData<Library[]>(
        queryKeys.libraries,
        previous.map((item) => (item.id === libraryId ? { ...item, ...payload } : item)),
      );
      return { previous };
    },
    onError: (err, _vars, context) => {
      if (context?.previous != null) {
        queryClient.setQueryData(queryKeys.libraries, context.previous);
      }
      notifyMutationError(err, "Could not update playback preferences");
    },
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
    gcTime: 30 * 60 * 1000,
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
    gcTime: 20 * 60 * 1000,
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
    gcTime: 20 * 60 * 1000,
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
    gcTime: 15 * 60 * 1000,
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
    gcTime: 2 * 60 * 1000,
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
    onError: (err) => notifyMutationError(err, "Could not refresh show metadata"),
  });
}

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
    onError: (err) => notifyMutationError(err, "Could not refresh library playback tracks"),
  });
}

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
    onError: (err) => notifyMutationError(err, "Could not refresh playback track metadata"),
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
    onError: (err) => notifyMutationError(err, "Could not confirm show metadata"),
  });
}

type PosterMutationCtx = { previous: MovieDetails | null | undefined; detailKey: readonly unknown[] };

export function useSetMoviePosterSelection(): UseMutationResult<
  void,
  Error,
  { libraryId: number; mediaId: number; sourceUrl: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ libraryId, mediaId, sourceUrl }) =>
      setMoviePosterSelection(libraryId, mediaId, { source_url: sourceUrl }),
    onMutate: async ({ libraryId, mediaId, sourceUrl }) => {
      const detailKey = queryKeys.movieDetails(libraryId, mediaId);
      await queryClient.cancelQueries({ queryKey: detailKey });
      const previous = queryClient.getQueryData<MovieDetails | null>(detailKey);
      if (previous != null) {
        queryClient.setQueryData<MovieDetails>(detailKey, {
          ...previous,
          poster_url: sourceUrl,
          poster_path: undefined,
        });
      }
      return { previous, detailKey } satisfies PosterMutationCtx;
    },
    onError: (err, _v, context) => {
      const ctx = context as PosterMutationCtx | undefined;
      if (ctx?.previous !== undefined) {
        queryClient.setQueryData(ctx.detailKey, ctx.previous);
      }
      notifyMutationError(err, "Could not set movie poster");
    },
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
    onError: (err) => notifyMutationError(err, "Could not reset movie poster"),
  });
}

type ShowPosterMutationCtx = { previous: ShowDetails | null | undefined; detailKey: readonly unknown[] };

export function useSetShowPosterSelection(): UseMutationResult<
  void,
  Error,
  { libraryId: number; showKey: string; sourceUrl: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ libraryId, showKey, sourceUrl }) =>
      setShowPosterSelection(libraryId, showKey, { source_url: sourceUrl }),
    onMutate: async ({ libraryId, showKey, sourceUrl }) => {
      const detailKey = queryKeys.showDetails(libraryId, showKey);
      await queryClient.cancelQueries({ queryKey: detailKey });
      const previous = queryClient.getQueryData<ShowDetails | null>(detailKey);
      if (previous != null) {
        queryClient.setQueryData<ShowDetails>(detailKey, {
          ...previous,
          poster_url: sourceUrl,
          poster_path: undefined,
        });
      }
      return { previous, detailKey } satisfies ShowPosterMutationCtx;
    },
    onError: (err, _v, context) => {
      const ctx = context as ShowPosterMutationCtx | undefined;
      if (ctx?.previous !== undefined) {
        queryClient.setQueryData(ctx.detailKey, ctx.previous);
      }
      notifyMutationError(err, "Could not set show poster");
    },
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
    onError: (err) => notifyMutationError(err, "Could not reset show poster"),
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

export function useRefreshLibraryIntroOnly(): UseMutationResult<
  LibraryPlaybackTracksRefreshResult,
  Error,
  { libraryId: number }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ libraryId }) =>
      contractsView<LibraryPlaybackTracksRefreshResult>(
        await refreshLibraryIntroOnly(libraryId),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.introScanSummary });
    },
    onError: (err) => notifyMutationError(err, "Could not re-scan library intros"),
  });
}

export function usePostLibraryIntroChromaprintScan(): UseMutationResult<
  LibraryPlaybackTracksRefreshResult,
  Error,
  { libraryId: number; showKey?: string }
> {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async ({ libraryId, showKey }) =>
      contractsView<LibraryPlaybackTracksRefreshResult>(
        await postLibraryIntroChromaprintScan(libraryId, { showKey }),
      ),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.introScanSummary });
    },
    onError: (err) => notifyMutationError(err, "Chromaprint intro scan failed"),
  });
}
