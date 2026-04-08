import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import {
  addDiscoverTitle,
  browseDiscover,
  getDownloads,
  removeDownload,
  getDiscover,
  getDiscoverGenres,
  getDiscoverTitleDetails,
  searchDiscover,
  type DiscoverBrowseCategory,
  type DiscoverMediaType,
  type DiscoverAcquisition,
  type DiscoverBrowseResponse,
  type DiscoverGenresResponse,
  type DiscoverResponse,
  type DiscoverSearchResponse,
  type DiscoverTitleDetails,
  type DownloadsResponse,
} from "@/api";
import { normalizeDiscoverOriginKey } from "@plum/shared";
import {
  contractsView,
  DISCOVER_STALE_MS,
  invalidateDiscoverRelatedQueries,
  notifyMutationError,
  queryKeys,
} from "./shared";

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

export function useDiscoverBrowse(options: {
  category?: DiscoverBrowseCategory | "";
  mediaType?: DiscoverMediaType | "";
  genreId?: number | null;
  originCountry?: string;
  enabled?: boolean;
  refetchInterval?: number | false;
}) {
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
    onError: (err) => notifyMutationError(err, "Could not remove download"),
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
    onError: (err) => notifyMutationError(err, "Could not add title from Discover"),
  });
}
