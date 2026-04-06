import { useEffect, useMemo } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { toast } from "sonner";
import type { Library, MediaItem } from "../api";
import { IdentifyMovieDialog } from "../components/IdentifyMovieDialog";
import { IdentifyShowDialog } from "../components/IdentifyShowDialog";
import { MovieLibraryCardContextMenu } from "../components/home/MovieLibraryCardContextMenu";
import { ShowLibraryCardContextMenu } from "../components/home/ShowLibraryCardContextMenu";
import { LibraryPosterGrid } from "../components/LibraryPosterGrid";
import { LibraryViewControls } from "../components/LibraryViewControls";
import { MediaDetailView, MediaTableView } from "../components/MediaListView";
import { MusicLibraryView } from "../components/MusicLibraryView";
import { MusicNowPlayingBar } from "../components/MusicNowPlayingBar";
import { PosterPickerDialog } from "../components/PosterPickerDialog";
import type { PosterGridItem } from "../components/types";
import { useIdentifyQueue, type IdentifyLibraryPhase } from "../contexts/IdentifyQueueContext";
import { usePlayerQueue } from "../contexts/PlayerContext";
import { useScanQueue } from "../contexts/ScanQueueContext";
import { useHomeLibraryDialogs } from "../hooks/useHomeLibraryDialogs";
import {
  buildMovieCardModels,
  buildShowGroupCardModels,
  hasProviderMatch,
  isActiveIdentifyState,
} from "../lib/homeLibraryCardModels";
import { getEnrichmentPhase, getLibraryActivity } from "../lib/libraryActivity";
import { groupMediaByShow } from "../lib/showGrouping";
import { useLibraryViewPrefs } from "../lib/useLibraryViewPrefs";
import { mediaItemNeedsIdentificationAttention } from "@/lib/unidentifiedMedia";
import {
  useConfirmShow,
  useLibraryMedia,
  useLibraries,
  useRefreshPlaybackTrackMetadata,
  useRefreshShow,
} from "../queries";

const isTVOrAnime = (lib: Library) => lib.type === "tv" || lib.type === "anime";
const IDENTIFY_POLL_INTERVAL_MS = 5_000;
const SCAN_POLL_INTERVAL_MS = 2_000;

function canShowFailureState(
  identifyPhase: IdentifyLibraryPhase | undefined,
  isProcessing: boolean,
  hasActiveIdentifyItems: boolean,
  identifyFailedCount: number,
) {
  const explicitFailure = identifyPhase === "identify-failed";
  // Do not gate on react-query isFetching: background refetches (e.g. identify poll) would
  // briefly hide failure and flip cards back to "Searching…", which looks like a glitch.
  return (
    !hasActiveIdentifyItems &&
    (explicitFailure || (!isProcessing && identifyPhase === "complete" && identifyFailedCount > 0))
  );
}

function mapBackendIdentifyPhase(phase?: string): IdentifyLibraryPhase | undefined {
  switch (phase) {
    case "queued":
      return "queued";
    case "identifying":
      return "identifying";
    case "completed":
      return "complete";
    case "failed":
      return "identify-failed";
    default:
      return undefined;
  }
}

function resolveLibraryIdentifyPhase(
  localPhase: IdentifyLibraryPhase | undefined,
  backendPhase: IdentifyLibraryPhase | undefined,
) {
  if (localPhase === "queued" || localPhase === "identifying" || localPhase === "soft-reveal") {
    return localPhase;
  }
  if (backendPhase === "queued" || backendPhase === "identifying") {
    return backendPhase;
  }
  return localPhase ?? backendPhase;
}

export function Home() {
  const { libraryId: libraryIdParam } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const unidentifiedOnly = searchParams.get("unidentified") === "1";
  const navigate = useNavigate();
  const { playMovie, playMusicCollection, playShowGroup } = usePlayerQueue();
  const { getLibraryPhase } = useIdentifyQueue();
  const { getLibraryScanStatus } = useScanQueue();
  const { cardWidth, setCardWidth, layoutMode, setLayoutMode } = useLibraryViewPrefs();
  const {
    data: libraries = [],
    isLoading: loadingLibs,
    error: loadLibsError,
    refetch: refetchLibraries,
  } = useLibraries();
  const selectedLibraryId = useMemo(() => {
    const id = libraryIdParam ? parseInt(libraryIdParam, 10) : null;
    if (id != null && libraries.some((library) => library.id === id)) return id;
    return libraries[0]?.id ?? null;
  }, [libraryIdParam, libraries]);
  const selectedLib = libraries.find((library) => library.id === selectedLibraryId);

  useEffect(() => {
    if (selectedLib?.type !== "music" || !unidentifiedOnly) return;
    const next = new URLSearchParams(searchParams);
    next.delete("unidentified");
    setSearchParams(next, { replace: true });
  }, [selectedLib?.type, unidentifiedOnly, searchParams, setSearchParams]);
  const selectedLibraryScanStatus = getLibraryScanStatus(selectedLibraryId);
  const selectedLibraryBackendIdentifyPhase = mapBackendIdentifyPhase(
    selectedLibraryScanStatus?.identifyPhase,
  );
  const selectedLibraryIdentifyPhase = resolveLibraryIdentifyPhase(
    getLibraryPhase(selectedLibraryId),
    selectedLibraryBackendIdentifyPhase,
  );
  const selectedLibraryActivity = getLibraryActivity({
    scanPhase: selectedLibraryScanStatus?.phase,
    enrichmentPhase: selectedLibraryScanStatus?.enrichmentPhase,
    enriching: selectedLibraryScanStatus?.enriching === true,
    identifyPhase: selectedLibraryScanStatus?.identifyPhase,
    localIdentifyPhase: selectedLibraryIdentifyPhase,
  });
  const selectedLibraryEnrichmentPhase = getEnrichmentPhase(selectedLibraryScanStatus ?? {});
  const isSelectedLibraryScanning =
    selectedLibraryScanStatus?.phase === "queued" ||
    selectedLibraryScanStatus?.phase === "scanning" ||
    selectedLibraryEnrichmentPhase === "queued" ||
    selectedLibraryEnrichmentPhase === "running" ||
    selectedLibraryScanStatus?.identifyPhase === "queued" ||
    selectedLibraryScanStatus?.identifyPhase === "identifying";
  const selectedLibraryPollInterval =
    selectedLibraryId == null
      ? false
      : isSelectedLibraryScanning
        ? SCAN_POLL_INTERVAL_MS
        : selectedLibraryIdentifyPhase === "identifying" ||
            selectedLibraryIdentifyPhase === "soft-reveal"
          ? IDENTIFY_POLL_INTERVAL_MS
          : false;
  const selectedLibraryPageSize =
    selectedLib?.type === "music" ? 100 : layoutMode === "grid" ? 60 : 75;
  const {
    data: selectedLibraryData,
    isLoading: selectedLoading,
    isFetchingNextPage: selectedFetchingNextPage,
    hasNextPage,
    fetchNextPage,
    error: selectedError,
    refetch: refetchLibraryMedia,
  } = useLibraryMedia(selectedLibraryId, {
    refetchInterval: selectedLibraryPollInterval,
    pageSize: selectedLibraryPageSize,
  });
  const selectedItems = useMemo<MediaItem[]>(
    () => selectedLibraryData?.pages.flatMap((page) => page.items) ?? [],
    [selectedLibraryData],
  );
  const selectedLibraryScanWarning =
    selectedLibraryScanStatus?.phase === "completed" && selectedItems.length === 0
      ? selectedLibraryScanStatus.error
      : undefined;
  const refreshShowMutation = useRefreshShow();
  const refreshPlaybackTrackMetadataMutation = useRefreshPlaybackTrackMetadata();
  const confirmShowMutation = useConfirmShow();
  const {
    identifyGroup,
    setIdentifyGroup,
    identifyMovieItem,
    setIdentifyMovieItem,
    posterPicker,
    setPosterPicker,
  } = useHomeLibraryDialogs();

  useEffect(() => {
    if (libraryIdParam != null || libraries.length === 0) return;
    navigate(`/library/${libraries[0].id}`, { replace: true });
  }, [libraryIdParam, libraries, navigate]);

  const loadMoreLibraryItems = () => {
    if (!hasNextPage || selectedFetchingNextPage) return;
    void fetchNextPage();
  };

  const hasActiveIdentifyItems = selectedItems.some((item) =>
    isActiveIdentifyState(item.identify_state),
  );
  const selectedLibraryCanShowFailure = canShowFailureState(
    selectedLibraryIdentifyPhase,
    isSelectedLibraryScanning,
    hasActiveIdentifyItems,
    selectedLibraryScanStatus?.identifyFailed ?? 0,
  );
  const hasIdentifyProgress = selectedItems.some((item) => {
    if (isActiveIdentifyState(item.identify_state)) return true;
    return item.match_status === "identified" || hasProviderMatch(item.tmdb_id, item.tvdb_id);
  });
  const shouldRevealSearchingCards =
    selectedLibraryIdentifyPhase === "soft-reveal" ||
    selectedLibraryIdentifyPhase === "identifying" ||
    selectedLibraryIdentifyPhase === "queued" ||
    hasIdentifyProgress;

  const showGroups = useMemo(
    () => (selectedLib && isTVOrAnime(selectedLib) ? groupMediaByShow(selectedItems) : []),
    [selectedItems, selectedLib],
  );

  const showCardModels = useMemo(
    () =>
      buildShowGroupCardModels({
        showGroups,
        selectedLibraryId,
        shouldRevealSearchingCards,
        selectedLibraryCanShowFailure,
        confirmShowPending: confirmShowMutation.isPending,
        confirmLibraryId: confirmShowMutation.variables?.libraryId,
        confirmShowKey: confirmShowMutation.variables?.showKey,
      }),
    [
      showGroups,
      selectedLibraryId,
      shouldRevealSearchingCards,
      selectedLibraryCanShowFailure,
      confirmShowMutation.isPending,
      confirmShowMutation.variables?.libraryId,
      confirmShowMutation.variables?.showKey,
    ],
  );

  const showCardState = useMemo((): PosterGridItem[] => {
    const lid = selectedLibraryId;
    return showCardModels.map(({ base, group, progressEpisode, statusAction }) => ({
      ...base,
      onPlay: () => playShowGroup(group.episodes, progressEpisode),
      onStatusAction:
        statusAction === "confirm-show" && lid != null
          ? () => confirmShowMutation.mutate({ libraryId: lid, showKey: group.showKey })
          : statusAction === "identify-show"
            ? () => setIdentifyGroup(group)
            : undefined,
      contextMenuContent:
        lid == null ? undefined : (
          <ShowLibraryCardContextMenu
            refreshShowDisabled={refreshShowMutation.isPending}
            refreshTracksDisabled={refreshPlaybackTrackMetadataMutation.isPending}
            onChangePoster={() =>
              setPosterPicker({
                kind: "show",
                libraryId: lid,
                showKey: group.showKey,
                title: group.showTitle,
              })
            }
            onRefreshShow={() =>
              refreshShowMutation.mutate({ libraryId: lid, showKey: group.showKey })
            }
            onRescanTracks={() => {
              const mediaIds = group.episodes.map((ep) => ep.id);
              void refreshPlaybackTrackMetadataMutation
                .mutateAsync({ libraryId: lid, mediaIds })
                .then(() => {
                  const n = mediaIds.length;
                  toast.success(
                    n === 1
                      ? `Tracks and subtitles rescanned for one episode of “${group.showTitle}”.`
                      : `Tracks and subtitles rescanned for ${n} episodes of “${group.showTitle}”.`,
                  );
                })
                .catch((err: unknown) => {
                  toast.error(
                    err instanceof Error ? err.message : "Could not rescan tracks and subtitles.",
                  );
                });
            }}
            onIdentify={() => setIdentifyGroup(group)}
            onOpenDetails={() =>
              navigate(`/library/${lid}/show/${encodeURIComponent(group.showKey)}`)
            }
          />
        ),
    }));
  }, [
    showCardModels,
    selectedLibraryId,
    playShowGroup,
    confirmShowMutation,
    setIdentifyGroup,
    refreshShowMutation,
    refreshPlaybackTrackMetadataMutation,
    navigate,
    setPosterPicker,
  ]);

  const movieCardModels = useMemo(
    () =>
      buildMovieCardModels({
        items: selectedItems,
        selectedLibraryId,
        shouldRevealSearchingCards,
        selectedLibraryCanShowFailure,
      }),
    [selectedItems, selectedLibraryId, shouldRevealSearchingCards, selectedLibraryCanShowFailure],
  );

  const movieCardState = useMemo((): PosterGridItem[] => {
    const lid = selectedLibraryId;
    return movieCardModels.map(({ base, item, statusAction }) => ({
      ...base,
      onPlay: () => playMovie(item),
      onStatusAction:
        statusAction === "identify-movie"
          ? () => setIdentifyMovieItem({ id: item.id, title: item.title })
          : undefined,
      contextMenuContent:
        lid == null ? undefined : (
          <MovieLibraryCardContextMenu
            refreshTracksDisabled={refreshPlaybackTrackMetadataMutation.isPending}
            onChangePoster={() =>
              setPosterPicker({
                kind: "movie",
                libraryId: lid,
                mediaId: item.id,
                title: item.title,
              })
            }
            onRescanTracks={() => {
              void refreshPlaybackTrackMetadataMutation
                .mutateAsync({ libraryId: lid, mediaIds: [item.id] })
                .then(() => {
                  toast.success(`Tracks and subtitles rescanned for “${item.title}”.`);
                })
                .catch((err: unknown) => {
                  toast.error(
                    err instanceof Error ? err.message : "Could not rescan tracks and subtitles.",
                  );
                });
            }}
            onIdentify={() => setIdentifyMovieItem({ id: item.id, title: item.title })}
            onOpenDetails={() => navigate(`/library/${lid}/movie/${item.id}`)}
          />
        ),
    }));
  }, [
    movieCardModels,
    selectedLibraryId,
    playMovie,
    refreshPlaybackTrackMetadataMutation,
    navigate,
    setPosterPicker,
    setIdentifyMovieItem,
  ]);

  const selectedLibraryCards = useMemo(() => {
    const raw =
      selectedLib == null || selectedLib.type === "music"
        ? []
        : isTVOrAnime(selectedLib)
          ? showCardState
          : movieCardState;
    if (!unidentifiedOnly || selectedLib == null || selectedLib.type === "music") {
      return raw;
    }
    if (isTVOrAnime(selectedLib)) {
      return raw.filter((card) => {
        const g = showGroups.find((gr) => gr.showKey === card.key);
        return g?.episodes.some((ep) => mediaItemNeedsIdentificationAttention(ep)) ?? false;
      });
    }
    return raw.filter((card) => {
      const mid = Number.parseInt(card.key, 10);
      if (!Number.isFinite(mid)) return false;
      const item = selectedItems.find((m) => m.id === mid);
      return item != null && mediaItemNeedsIdentificationAttention(item);
    });
  }, [
    unidentifiedOnly,
    selectedLib,
    showCardState,
    movieCardState,
    showGroups,
    selectedItems,
  ]);

  return (
    <>
      {loadingLibs ? (
        <p className="text-sm text-(--plum-muted)">Loading libraries…</p>
      ) : loadLibsError ? (
        <p className="text-sm text-(--plum-muted)">
          Failed to load libraries: {loadLibsError.message}{" "}
          <button
            type="button"
            className="text-(--plum-accent) hover:underline"
            onClick={() => void refetchLibraries()}
          >
            Retry
          </button>
        </p>
      ) : libraries.length === 0 ? (
        <p className="text-sm text-(--plum-muted)">
          No libraries yet. Add one in Settings or onboarding.
        </p>
      ) : (
        <>
          {selectedLib && (
            <div className="flex min-h-0 flex-1 flex-col">
              {selectedLoading ? (
                <p className="text-sm text-(--plum-muted)">Loading…</p>
              ) : selectedError ? (
                <p className="text-sm text-(--plum-muted)">
                  {selectedError.message}{" "}
                  <button
                    type="button"
                    className="text-(--plum-accent) hover:underline"
                    onClick={() => void refetchLibraryMedia()}
                  >
                    Retry
                  </button>
                </p>
              ) : selectedLibraryActivity != null && selectedItems.length === 0 ? (
                <p className="text-sm text-(--plum-muted)">
                  {selectedLibraryActivity === "importing"
                    ? "Importing library…"
                    : selectedLibraryActivity === "analyze-queued"
                      ? "Waiting for analyzer…"
                    : selectedLibraryActivity === "analyzing"
                      ? "Analyzing media…"
                      : selectedLibraryActivity === "identify-queued"
                        ? "Queued for identify…"
                        : "Identifying library…"}
                  {selectedLibraryActivity === "importing" && selectedLibraryScanStatus && (
                    <>
                      {" "}
                      {selectedLibraryScanStatus.processed} processed •{" "}
                      {selectedLibraryScanStatus.added} added
                    </>
                  )}
                </p>
              ) : selectedLibraryScanWarning ? (
                <p className="text-sm text-(--plum-muted)">{selectedLibraryScanWarning}</p>
              ) : selectedItems.length === 0 ? (
                <p className="text-sm text-(--plum-muted)">No media in this library yet.</p>
              ) : unidentifiedOnly && selectedLibraryCards.length === 0 ? (
                <div className="space-y-3">
                  <p className="text-sm text-(--plum-muted)">
                    No unidentified items in the loaded page yet.
                    {hasNextPage
                      ? " Load more pages to find additional titles, or clear the filter to browse everything."
                      : " If you already fixed matches, counts refresh after the next library scan or identify."}
                  </p>
                  <div className="flex flex-wrap items-center gap-3">
                    <button
                      type="button"
                      className="text-sm font-medium text-(--plum-accent) hover:underline"
                      onClick={() => {
                        const next = new URLSearchParams(searchParams);
                        next.delete("unidentified");
                        setSearchParams(next, { replace: true });
                      }}
                    >
                      Show entire library
                    </button>
                    {hasNextPage ? (
                      <button
                        type="button"
                        className="text-sm font-medium text-(--plum-text) underline decoration-(--plum-border) underline-offset-2 hover:text-(--plum-accent)"
                        disabled={selectedFetchingNextPage}
                        onClick={() => loadMoreLibraryItems()}
                      >
                        {selectedFetchingNextPage ? "Loading…" : "Load more"}
                      </button>
                    ) : null}
                  </div>
                </div>
              ) : selectedLib.type !== "music" ? (
                <>
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-4">
                    <div className="min-w-0">
                      <h2 className="text-base font-semibold text-(--plum-text) truncate">
                        {selectedLib.name}
                      </h2>
                      {unidentifiedOnly ? (
                        <p className="mt-1 text-xs text-(--plum-text-2)">
                          Showing titles that still need identification.
                          <button
                            type="button"
                            className="ml-2 text-(--plum-accent) hover:underline"
                            onClick={() => {
                              const next = new URLSearchParams(searchParams);
                              next.delete("unidentified");
                              setSearchParams(next, { replace: true });
                            }}
                          >
                            Show all
                          </button>
                        </p>
                      ) : null}
                    </div>
                    <LibraryViewControls
                      cardWidth={cardWidth}
                      onCardWidthChange={setCardWidth}
                      layoutMode={layoutMode}
                      onLayoutModeChange={setLayoutMode}
                    />
                  </div>
                  {layoutMode === "grid" ? (
                    <LibraryPosterGrid
                      items={selectedLibraryCards}
                      aspectRatio="cinema"
                      cardWidth={cardWidth}
                      hasMore={hasNextPage ?? false}
                      onLoadMore={loadMoreLibraryItems}
                    />
                  ) : layoutMode === "detail" ? (
                    <MediaDetailView
                      items={selectedLibraryCards}
                      hasMore={hasNextPage ?? false}
                      onLoadMore={loadMoreLibraryItems}
                    />
                  ) : (
                    <MediaTableView
                      items={selectedLibraryCards}
                      hasMore={hasNextPage ?? false}
                      onLoadMore={loadMoreLibraryItems}
                    />
                  )}
                </>
              ) : (
                <div className="flex min-h-0 flex-1 flex-col">
                  <div className="min-h-0 flex-1 overflow-auto">
                    <MusicLibraryView
                      items={selectedItems}
                      onPlayCollection={playMusicCollection}
                      hasMore={hasNextPage ?? false}
                      onLoadMore={loadMoreLibraryItems}
                    />
                  </div>
                  <MusicNowPlayingBar visible />
                </div>
              )}
              {identifyGroup && selectedLibraryId != null && (
                <IdentifyShowDialog
                  open={!!identifyGroup}
                  onOpenChange={(open) => !open && setIdentifyGroup(null)}
                  libraryId={selectedLibraryId}
                  showKey={identifyGroup.showKey}
                  showTitle={identifyGroup.showTitle}
                  onSuccess={() => void refetchLibraryMedia()}
                />
              )}
              {identifyMovieItem && selectedLibraryId != null ? (
                <IdentifyMovieDialog
                  open={identifyMovieItem != null}
                  onOpenChange={(open) => !open && setIdentifyMovieItem(null)}
                  libraryId={selectedLibraryId}
                  mediaId={identifyMovieItem.id}
                  movieTitle={identifyMovieItem.title}
                  onSuccess={() => void refetchLibraryMedia()}
                />
              ) : null}
              {posterPicker ? (
                <PosterPickerDialog
                  open={posterPicker != null}
                  onOpenChange={(open) => !open && setPosterPicker(null)}
                  {...posterPicker}
                />
              ) : null}
            </div>
          )}
        </>
      )}
    </>
  );
}
