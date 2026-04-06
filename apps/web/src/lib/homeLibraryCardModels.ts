import type { MediaItem } from "@/api";
import type { PosterGridItem } from "@/components/types";
import { formatEpisodeLabel, formatRemainingTime, shouldShowProgress } from "@/lib/progress";
import { getPreferredMovieRating } from "@/lib/ratings";
import type { ShowGroup } from "@/lib/showGrouping";

export type ItemIdentifyState = "queued" | "identifying" | "failed" | undefined;

export const hasProviderMatch = (tmdbId?: number, tvdbId?: string) =>
  Boolean(tmdbId && tmdbId > 0) || Boolean(tvdbId);

const isExplicitlyUnmatched = (matchStatus?: string) =>
  matchStatus === "local" || matchStatus === "unmatched";

export const isActiveIdentifyState = (identifyState?: ItemIdentifyState) =>
  identifyState === "queued" || identifyState === "identifying";

/** Once provider poster art is present, drop the "Searching…" pill — it lags behind the visible poster for partial matches. */
export function hasMetadataPoster(posterPath?: string, posterUrl?: string) {
  return Boolean(posterPath?.trim() || posterUrl?.trim());
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

export type ShowGroupCardBase = Omit<PosterGridItem, "contextMenuContent" | "onPlay" | "onStatusAction">;

export type ShowGroupCardModel = {
  base: ShowGroupCardBase;
  group: ShowGroup;
  progressEpisode?: MediaItem;
  statusAction: "confirm-show" | "identify-show" | null;
};

export type MovieCardBase = Omit<PosterGridItem, "contextMenuContent" | "onPlay" | "onStatusAction">;

export type MovieCardModel = {
  base: MovieCardBase;
  item: MediaItem;
  statusAction: "identify-movie" | null;
};

export function buildShowGroupCardModels(input: {
  showGroups: ShowGroup[];
  selectedLibraryId: number | null;
  shouldRevealSearchingCards: boolean;
  selectedLibraryCanShowFailure: boolean;
  confirmShowPending: boolean;
  confirmLibraryId?: number;
  confirmShowKey?: string;
}): ShowGroupCardModel[] {
  const {
    showGroups,
    selectedLibraryId,
    shouldRevealSearchingCards,
    selectedLibraryCanShowFailure,
    confirmShowPending,
    confirmLibraryId,
    confirmShowKey,
  } = input;

  return showGroups.flatMap((group) => {
    const progressEpisode = [...group.episodes]
      .filter((episode) => shouldShowProgress(episode))
      .toSorted((a, b) => (b.last_watched_at ?? "").localeCompare(a.last_watched_at ?? ""))[0];
    const needsMetadataReview = group.episodes.some(
      (episode) => episode.metadata_review_needed === true,
    );
    const isConfirmingReview =
      confirmShowPending &&
      confirmLibraryId === selectedLibraryId &&
      confirmShowKey === group.showKey;
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

    let statusAction: ShowGroupCardModel["statusAction"] = null;
    if (needsMetadataReview && selectedLibraryId != null) {
      statusAction = "confirm-show";
    } else if (showFailure && selectedLibraryId != null) {
      statusAction = "identify-show";
    }

    const base: ShowGroupCardBase = {
      key: group.showKey,
      title: group.showTitle,
      subtitle: `${group.episodes.length} episode${group.episodes.length === 1 ? "" : "s"}${group.unmatchedCount > 0 ? ` • ${group.unmatchedCount} unmatched` : group.localCount > 0 ? ` • ${group.localCount} local` : ""}`,
      metaLine: progressEpisode
        ? [formatEpisodeLabel(progressEpisode), formatRemainingTime(progressEpisode.remaining_seconds)]
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
      href: `/library/${selectedLibraryId}/show/${encodeURIComponent(group.showKey)}`,
    };

    return [{ base, group, progressEpisode, statusAction }];
  });
}

export function buildMovieCardModels(input: {
  items: MediaItem[];
  selectedLibraryId: number | null;
  shouldRevealSearchingCards: boolean;
  selectedLibraryCanShowFailure: boolean;
}): MovieCardModel[] {
  const { items, selectedLibraryId, shouldRevealSearchingCards, selectedLibraryCanShowFailure } = input;

  return items.flatMap((item) => {
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
        (item.identify_state == null && shouldRevealSearchingCards && !selectedLibraryCanShowFailure));
    const showFailure = isMovieTerminalFailure(item, selectedLibraryCanShowFailure);

    const statusAction: MovieCardModel["statusAction"] =
      showFailure && selectedLibraryId != null ? "identify-movie" : null;

    const base: MovieCardBase = {
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
      statusActionLabel: showFailure && selectedLibraryId != null ? "Identify manually" : undefined,
      href: selectedLibraryId != null ? `/library/${selectedLibraryId}/movie/${item.id}` : undefined,
    };

    return [{ base, item, statusAction }];
  });
}
