import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import {
  getMetadataArtworkSettings,
  getMediaStackSettings,
  getServerEnvSettings,
  getTranscodingSettings,
  updateMetadataArtworkSettings,
  updateMediaStackSettings,
  updateServerEnvSettings,
  updateTranscodingSettings,
  validateMediaStackSettings,
  type MetadataArtworkSettings,
  type MetadataArtworkSettingsResponse,
  type MediaStackSettings,
  type MediaStackValidationResult,
  type ServerEnvSettingsResponse,
  type ServerEnvSettingsUpdate,
  type TranscodingSettings,
  type TranscodingSettingsResponse,
} from "@/api";
import {
  contractsView,
  invalidateDiscoverRelatedQueries,
  notifyMutationError,
  queryKeys,
  SERVER_SETTINGS_STALE_MS,
} from "./shared";

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
    onMutate: async (next) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.transcodingSettings });
      const previous = queryClient.getQueryData<TranscodingSettingsResponse>(queryKeys.transcodingSettings);
      if (previous != null) {
        queryClient.setQueryData<TranscodingSettingsResponse>(queryKeys.transcodingSettings, {
          ...previous,
          settings: next,
        });
      }
      return { previous };
    },
    onError: (err, _next, context) => {
      if (context?.previous !== undefined) {
        queryClient.setQueryData(queryKeys.transcodingSettings, context.previous);
      }
      notifyMutationError(err, "Could not save transcoding settings");
    },
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
    onMutate: async (next) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.metadataArtworkSettings });
      const previous = queryClient.getQueryData<MetadataArtworkSettingsResponse>(
        queryKeys.metadataArtworkSettings,
      );
      if (previous != null) {
        queryClient.setQueryData<MetadataArtworkSettingsResponse>(queryKeys.metadataArtworkSettings, {
          ...previous,
          settings: next,
        });
      }
      return { previous };
    },
    onError: (err, _next, context) => {
      if (context?.previous !== undefined) {
        queryClient.setQueryData(queryKeys.metadataArtworkSettings, context.previous);
      }
      notifyMutationError(err, "Could not save metadata artwork settings");
    },
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
    onMutate: async (next) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.mediaStackSettings });
      const previous = queryClient.getQueryData<MediaStackSettings>(queryKeys.mediaStackSettings);
      queryClient.setQueryData<MediaStackSettings>(queryKeys.mediaStackSettings, next);
      return { previous };
    },
    onError: (err, _next, context) => {
      if (context?.previous !== undefined) {
        queryClient.setQueryData(queryKeys.mediaStackSettings, context.previous);
      }
      notifyMutationError(err, "Could not save media stack settings");
    },
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
    onError: (err) => notifyMutationError(err, "Could not validate media stack settings"),
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
    onError: (err) => notifyMutationError(err, "Could not save server settings"),
  });
}
