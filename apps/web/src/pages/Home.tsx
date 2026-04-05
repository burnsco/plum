import { useEffect, useMemo, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { Captions, ExternalLink, Image, RefreshCw, ScanSearch } from "lucide-react";
import { toast } from "sonner";
import type { Library, MediaItem } from "../api";
import { IdentifyMovieDialog } from "../components/IdentifyMovieDialog";
import { IdentifyShowDialog } from "../components/IdentifyShowDialog";
import type { PosterGridItem } from "../components/types";
import { LibraryPosterGrid } from "../components/LibraryPosterGrid";
import { LibraryViewControls } from "../components/LibraryViewControls";
import { MediaDetailView, MediaTableView } from "../components/MediaListView";
import { MusicLibraryView } from "../components/MusicLibraryView";
import { MusicNowPlayingBar } from "../components/MusicNowPlayingBar";
import { PosterPickerDialog } from "../components/PosterPickerDialog";
import { useIdentifyQueue, type IdentifyLibraryPhase } from "../contexts/IdentifyQueueContext";
import { usePlayer } from "../contexts/PlayerContext";
import { useScanQueue } from "../contexts/ScanQueueContext";
import { getEnrichmentPhase, getLibraryActivity } from "../lib/libraryActivity";
import { formatEpisodeLabel, formatRemainingTime, shouldShowProgress } from "../lib/progress";
import { getPreferredMovieRating } from "../lib/ratings";
import type { ShowGroup } from "../lib/showGrouping";
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
import {
  ContextMenuItem,
  ContextMenuSeparator,
} from "@/components/ui/context-menu";

const isTVOrAnime = (lib: Library) => lib.type === "tv" || lib.type === "anime";
const IDENTIFY_POLL_INTERVAL_MS = 5_000;
const SCAN_POLL_INTERVAL_MS = 2_000;
type ItemIdentifyState = "queued" | "identifying" | "failed" | undefined;

const hasProviderMatch = (tmdbId?: number, tvdbId?: string) =>
  Boolean(tmdbId && tmdbId > 0) || Boolean(tvdbId);
const isExplicitlyUnmatched = (matchStatus?: string) =>
  matchStatus === "local" || matchStatus === "unmatched";
const isActiveIdentifyState = (identifyState?: ItemIdentifyState) =>
  identifyState === "queued" || identifyState === "identifying";

/** Once provider poster art is present, drop the "Searching…" pill — it lags behind the visible poster for partial matches. */
function hasMetadataPoster(posterPath?: string, posterUrl?: string) {
  return Boolean(posterPath?.trim() || posterUrl?.trim());
}

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

function isMovieIncomplete(item: {
  match_status?: string;
  poster_path?: string;
  tmdb_id?: number;
  tvdb_id?: string;
}) {
  const hasIdentity = Boolean(item.tmdb_id && item.tmdb_id > 0) || Boolean(item.tvdb_id);
  const isIdentified = item.match_status === "identified" || hasIdentity;
  return !isIdentified && isExplicitlyUnmatched(item.match_status);
}

function isMovieTerminalFailure(
  item: {
    identify_state?: ItemIdentifyState;
    match_status?: string;
    poster_path?: string;
    tmdb_id?: number;
    tvdb_id?: string;
  },
  libraryCanShowFailure: boolean,
) {
  return (
    isMovieIncomplete(item) &&
    !isActiveIdentifyState(item.identify_state) &&
    (item.identify_state === "failed" || libraryCanShowFailure)
  );
}

function getGroupIdentifyState(group: ShowGroup): ItemIdentifyState {
  if (group.episodes.some((episode) => episode.identify_state === "identifying"))
    return "identifying";
  if (group.episodes.some((episode) => episode.identify_state === "queued")) return "queued";
  if (group.episodes.some((episode) => episode.identify_state === "failed")) return "failed";
  return undefined;
}

function getShowGroupRating(group: ShowGroup) {
  if ((group.showImdbRating ?? 0) > 0) {
    return { label: "IMDb", value: group.showImdbRating };
  }
  if ((group.showVoteAverage ?? 0) > 0) {
    return { label: "TMDb", value: group.showVoteAverage };
  }
  return { label: undefined, value: undefined };
}

export function Home() {
  const { libraryId: libraryIdParam } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const unidentifiedOnly = searchParams.get("unidentified") === "1";
  const navigate = useNavigate();
  const { playMovie, playMusicCollection, playShowGroup } = usePlayer();
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

  const [identifyGroup, setIdentifyGroup] = useState<ShowGroup | null>(null);
  const [identifyMovieItem, setIdentifyMovieItem] = useState<{ id: number; title: string } | null>(null);
  const [posterPicker, setPosterPicker] = useState<
    | { kind: "movie"; libraryId: number; mediaId: number; title: string }
    | { kind: "show"; libraryId: number; showKey: string; title: string }
    | null
  >(null);

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

  const showCardState = useMemo(() => {
    const visibleCards = showGroups.flatMap((group) => {
      const progressEpisode = [...group.episodes]
        .filter((episode) => shouldShowProgress(episode))
        .toSorted((a, b) => (b.last_watched_at ?? "").localeCompare(a.last_watched_at ?? ""))[0];
      const needsMetadataReview = group.episodes.some(
        (episode) => episode.metadata_review_needed === true,
      );
      const isConfirmingReview =
        confirmShowMutation.isPending &&
        confirmShowMutation.variables?.libraryId === selectedLibraryId &&
        confirmShowMutation.variables?.showKey === group.showKey;
      const identifyState = getGroupIdentifyState(group);
      const isIncomplete = group.unmatchedCount > 0 || group.localCount > 0;
      const groupHasPoster = hasMetadataPoster(group.posterPath, group.posterUrl);
      const showSearching =
        isIncomplete &&
        !groupHasPoster &&
        (isActiveIdentifyState(identifyState) ||
          (identifyState == null && shouldRevealSearchingCards && !selectedLibraryCanShowFailure));
      const showFailure =
        isIncomplete &&
        !showSearching &&
        !needsMetadataReview &&
        !isActiveIdentifyState(identifyState) &&
        (identifyState === "failed" || selectedLibraryCanShowFailure);
      const rating = getShowGroupRating(group);

      return [
        {
          key: group.showKey,
          title: group.showTitle,
          subtitle: `${group.episodes.length} episode${group.episodes.length === 1 ? "" : "s"}${group.unmatchedCount > 0 ? ` • ${group.unmatchedCount} unmatched` : group.localCount > 0 ? ` • ${group.localCount} local` : ""}`,
          metaLine: progressEpisode
            ? [
                formatEpisodeLabel(progressEpisode),
                formatRemainingTime(progressEpisode.remaining_seconds),
              ]
                .filter(Boolean)
                .join(" • ")
            : undefined,
          posterPath: group.posterPath,
          posterUrl: group.posterUrl,
          ratingLabel: rating.label,
          ratingValue: rating.value,
          progressPercent: progressEpisode?.progress_percent,
          cardState: needsMetadataReview
            ? "review-needed"
            : showSearching
              ? "identifying"
              : showFailure
                ? "identify-failed"
                : "default",
          statusLabel: needsMetadataReview
            ? "Is this correct?"
            : showSearching
              ? "Searching…"
              : showFailure
                ? "Couldn't match automatically"
                : undefined,
          statusActionLabel:
            needsMetadataReview && selectedLibraryId != null
              ? "Confirm"
              : showFailure && selectedLibraryId != null
                ? "Identify manually"
                : undefined,
          statusActionDisabled: isConfirmingReview,
          onStatusAction:
            needsMetadataReview && selectedLibraryId != null
              ? () =>
                  confirmShowMutation.mutate({
                    libraryId: selectedLibraryId,
                    showKey: group.showKey,
                  })
              : showFailure && selectedLibraryId != null
                ? () => setIdentifyGroup(group)
                : undefined,
          href: `/library/${selectedLibraryId}/show/${encodeURIComponent(group.showKey)}`,
          onPlay: () => playShowGroup(group.episodes, progressEpisode),
          contextMenuContent:
            selectedLibraryId == null ? undefined : (
              <>
                <ContextMenuItem
                  onSelect={() => {
                    setPosterPicker({
                      kind: "show",
                      libraryId: selectedLibraryId,
                      showKey: group.showKey,
                      title: group.showTitle,
                    });
                  }}
                >
                  <Image className="size-4 text-(--plum-muted)" />
                  Change poster…
                </ContextMenuItem>
                <ContextMenuSeparator />
                <ContextMenuItem
                  disabled={refreshShowMutation.isPending}
                  onSelect={() => {
                    refreshShowMutation.mutate({
                      libraryId: selectedLibraryId,
                      showKey: group.showKey,
                    });
                  }}
                >
                  <RefreshCw className="size-4 text-(--plum-muted)" />
                  Refresh metadata
                </ContextMenuItem>
                <ContextMenuItem
                  disabled={refreshPlaybackTrackMetadataMutation.isPending}
                  onSelect={() => {
                    const mediaIds = group.episodes.map((ep) => ep.id);
                    void refreshPlaybackTrackMetadataMutation
                      .mutateAsync({
                        libraryId: selectedLibraryId,
                        mediaIds,
                      })
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
                >
                  <Captions className="size-4 text-(--plum-muted)" />
                  Rescan tracks & subtitles (all episodes)
                </ContextMenuItem>
                <ContextMenuItem onSelect={() => setIdentifyGroup(group)}>
                  <ScanSearch className="size-4 text-(--plum-muted)" />
                  Identify…
                </ContextMenuItem>
                <ContextMenuSeparator />
                <ContextMenuItem
                  onSelect={() => {
                    navigate(
                      `/library/${selectedLibraryId}/show/${encodeURIComponent(group.showKey)}`,
                    );
                  }}
                >
                  <ExternalLink className="size-4 text-(--plum-muted)" />
                  Open details
                </ContextMenuItem>
              </>
            ),
        },
      ] satisfies PosterGridItem[];
    });
    return visibleCards;
  }, [
    confirmShowMutation,
    navigate,
    playShowGroup,
    refreshPlaybackTrackMetadataMutation,
    refreshShowMutation,
    shouldRevealSearchingCards,
    selectedLibraryCanShowFailure,
    selectedLibraryId,
    showGroups,
  ]);

  const movieCardState = useMemo(() => {
    const visibleCards = selectedItems.flatMap((item) => {
      const year =
        item.release_date?.split("-")[0] || item.title.match(/\((\d{4})\)$/)?.[1] || "Unknown year";
      const rating = getPreferredMovieRating(item);
      const status =
        item.match_status &&
        item.match_status !== "identified" &&
        !(item.match_status === "local" && hasProviderMatch(item.tmdb_id, item.tvdb_id))
          ? ` • ${item.match_status}`
          : "";
      const isIncomplete = isMovieIncomplete(item);
      const movieHasPoster = hasMetadataPoster(item.poster_path, item.poster_url);
      const showSearching =
        isIncomplete &&
        !movieHasPoster &&
        (isActiveIdentifyState(item.identify_state) ||
          (item.identify_state == null &&
            shouldRevealSearchingCards &&
            !selectedLibraryCanShowFailure));
      const showFailure = isMovieTerminalFailure(item, selectedLibraryCanShowFailure);

      return [
        {
          key: String(item.id),
          title: item.title,
          subtitle: `${year}${status}`,
          metaLine: formatRemainingTime(item.remaining_seconds),
          posterPath: item.poster_path,
          posterUrl: item.poster_url,
          ratingLabel: rating.label,
          ratingValue: rating.value,
          progressPercent: shouldShowProgress(item) ? item.progress_percent : undefined,
          cardState: showSearching ? "identifying" : showFailure ? "identify-failed" : "default",
          statusLabel: showSearching
            ? "Searching…"
            : showFailure
              ? "Couldn't match automatically"
              : undefined,
          statusActionLabel:
            showFailure && selectedLibraryId != null ? "Identify manually" : undefined,
          onStatusAction:
            showFailure && selectedLibraryId != null
              ? () => setIdentifyMovieItem({ id: item.id, title: item.title })
              : undefined,
          href: selectedLibraryId != null ? `/library/${selectedLibraryId}/movie/${item.id}` : undefined,
          onPlay: () => playMovie(item),
          contextMenuContent:
            selectedLibraryId == null ? undefined : (
              <>
                <ContextMenuItem
                  onSelect={() => {
                    setPosterPicker({
                      kind: "movie",
                      libraryId: selectedLibraryId,
                      mediaId: item.id,
                      title: item.title,
                    });
                  }}
                >
                  <Image className="size-4 text-(--plum-muted)" />
                  Change poster…
                </ContextMenuItem>
                <ContextMenuSeparator />
                <ContextMenuItem
                  disabled={refreshPlaybackTrackMetadataMutation.isPending}
                  onSelect={() => {
                    void refreshPlaybackTrackMetadataMutation
                      .mutateAsync({
                        libraryId: selectedLibraryId,
                        mediaIds: [item.id],
                      })
                      .then(() => {
                        toast.success(`Tracks and subtitles rescanned for “${item.title}”.`);
                      })
                      .catch((err: unknown) => {
                        toast.error(
                          err instanceof Error ? err.message : "Could not rescan tracks and subtitles.",
                        );
                      });
                  }}
                >
                  <Captions className="size-4 text-(--plum-muted)" />
                  Rescan tracks & subtitles
                </ContextMenuItem>
                <ContextMenuSeparator />
                <ContextMenuItem onSelect={() => setIdentifyMovieItem({ id: item.id, title: item.title })}>
                  <ScanSearch className="size-4 text-(--plum-muted)" />
                  Identify…
                </ContextMenuItem>
                <ContextMenuSeparator />
                <ContextMenuItem
                  onSelect={() => {
                    navigate(`/library/${selectedLibraryId}/movie/${item.id}`);
                  }}
                >
                  <ExternalLink className="size-4 text-(--plum-muted)" />
                  Open details
                </ContextMenuItem>
              </>
            ),
        },
      ] satisfies PosterGridItem[];
    });

    return visibleCards;
  }, [
    navigate,
    playMovie,
    refreshPlaybackTrackMetadataMutation,
    selectedItems,
    shouldRevealSearchingCards,
    selectedLibraryCanShowFailure,
    selectedLibraryId,
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
