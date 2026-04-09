import type { HomeDashboard, RecentlyAddedEntry } from "@/api";
import { DashboardCardContextMenu } from "@/components/home/DashboardCardContextMenu";
import { PageRouteSkeleton } from "@/components/loading/PlumLoadingSkeletons";
import { PosterScrollRail } from "@/components/PosterScrollRail";
import type { PosterGridItem } from "@/components/types";
import { usePlayerQueue } from "@/contexts/PlayerContext";
import { formatRemainingTime } from "@/lib/progress";
import { getPreferredMovieRating, getPreferredShowRatingFromBrowseEpisode } from "@/lib/ratings";
import { useClearMediaProgress, useClearShowProgress, useHomeDashboard } from "@/queries";
import { useCallback, useMemo } from "react";
import { useNavigate } from "react-router-dom";
import { toast } from "sonner";

type DashboardEntry =
  | HomeDashboard["continueWatching"][number]
  | RecentlyAddedEntry;

type DashboardShelf =
  | "continueWatching"
  | "recentlyAddedTvEpisodes"
  | "recentlyAddedTvShows"
  | "recentlyAddedMovies"
  | "recentlyAddedAnimeEpisodes"
  | "recentlyAddedAnimeShows";

function getDashboardEntryTitle(entry: DashboardEntry): string {
  if (entry.kind === "movie") {
    return entry.media.title;
  }
  return entry.show_title || entry.media.title;
}

function getDashboardEntrySubtitle(entry: DashboardEntry, shelf: DashboardShelf): string {
  const remainingSeconds = "remaining_seconds" in entry ? entry.remaining_seconds : undefined;
  if (entry.kind === "show" || entry.kind === "episode") {
    if (shelf === "continueWatching") {
      return [entry.episode_label, formatRemainingTime(remainingSeconds)]
        .filter(Boolean)
        .join(" • ");
    }
    return entry.episode_label || "New episode";
  }

  const year = entry.media.release_date?.split("-")[0] ?? "Movie";
  if (shelf === "continueWatching") {
    return [year, formatRemainingTime(remainingSeconds)].filter(Boolean).join(" • ");
  }
  return year;
}

function dashboardDetailHref(entry: DashboardEntry): string | undefined {
  const libraryId = entry.media.library_id;
  if (libraryId == null || libraryId <= 0) return undefined;
  if (entry.kind === "movie") {
    return `/library/${libraryId}/movie/${entry.media.id}`;
  }
  if (entry.show_key) {
    return `/library/${libraryId}/show/${encodeURIComponent(entry.show_key)}`;
  }
  return undefined;
}

/** TV/anime shelves should show series poster art, not episode stills or generated frame thumbnails. */
function dashboardPosterFields(entry: DashboardEntry): { posterPath?: string; posterUrl?: string } {
  if (entry.kind === "show" || entry.kind === "episode") {
    return {
      posterPath: entry.media.show_poster_path ?? entry.media.poster_path,
      posterUrl: entry.media.show_poster_url ?? entry.media.poster_url,
    };
  }
  return {
    posterPath: entry.media.poster_path,
    posterUrl: entry.media.poster_url,
  };
}

function toPosterGridItem(
  entry: DashboardEntry,
  shelf: DashboardShelf,
  playMovie: (item: DashboardEntry["media"]) => void,
  playEpisode: (item: DashboardEntry["media"], options?: { showKey?: string }) => void,
): PosterGridItem {
  const playItem =
    entry.kind === "movie"
      ? () => playMovie(entry.media)
      : () =>
          playEpisode(
            entry.media,
            entry.show_key?.trim() ? { showKey: entry.show_key } : undefined,
          );
  const rating =
    entry.kind === "movie"
      ? getPreferredMovieRating(entry.media)
      : getPreferredShowRatingFromBrowseEpisode(entry.media);
  const { posterPath, posterUrl } = dashboardPosterFields(entry);

  return {
    key: `${shelf}-${entry.kind}-${entry.media.id}`,
    title: getDashboardEntryTitle(entry),
    subtitle: getDashboardEntrySubtitle(entry, shelf),
    posterPath,
    posterUrl,
    ratingLabel: rating.label,
    ratingValue: rating.value,
    progressPercent: shelf === "continueWatching" ? entry.media.progress_percent : undefined,
    onClick: playItem,
    onPlay: playItem,
  };
}

type HomeRecentRailsKey =
  | "recentlyAddedTvEpisodes"
  | "recentlyAddedTvShows"
  | "recentlyAddedMovies"
  | "recentlyAddedAnimeEpisodes"
  | "recentlyAddedAnimeShows";

type RecentRailConfig = {
  shelf: Exclude<DashboardShelf, "continueWatching">;
  entriesKey: HomeRecentRailsKey;
  title: string;
  headingId: string;
  testId: string;
  countNoun: string;
  emptyMessage: string;
};

const RECENT_RAILS: RecentRailConfig[] = [
  {
    shelf: "recentlyAddedTvEpisodes",
    entriesKey: "recentlyAddedTvEpisodes",
    title: "Recently added TV episodes",
    headingId: "dash-recent-tv-episodes-heading",
    testId: "dashboard-recent-tv-episodes-heading",
    countNoun: "episode",
    emptyMessage: "Scan a TV library and newly added episodes will appear in this row.",
  },
  {
    shelf: "recentlyAddedTvShows",
    entriesKey: "recentlyAddedTvShows",
    title: "Recently added TV shows",
    headingId: "dash-recent-tv-shows-heading",
    testId: "dashboard-recent-tv-shows-heading",
    countNoun: "show",
    emptyMessage: "Grouped by series — newest episodes surface here once your TV library is scanned.",
  },
  {
    shelf: "recentlyAddedMovies",
    entriesKey: "recentlyAddedMovies",
    title: "Recently added movies",
    headingId: "dash-recent-movies-heading",
    testId: "dashboard-recent-movies-heading",
    countNoun: "film",
    emptyMessage: "Scan a movie library and the newest additions will show up in this row.",
  },
  {
    shelf: "recentlyAddedAnimeEpisodes",
    entriesKey: "recentlyAddedAnimeEpisodes",
    title: "Recently added anime episodes",
    headingId: "dash-recent-anime-episodes-heading",
    testId: "dashboard-recent-anime-episodes-heading",
    countNoun: "episode",
    emptyMessage: "Scan an anime library and new episodes will appear in this row.",
  },
  {
    shelf: "recentlyAddedAnimeShows",
    entriesKey: "recentlyAddedAnimeShows",
    title: "Recently added anime",
    headingId: "dash-recent-anime-shows-heading",
    testId: "dashboard-recent-anime-shows-heading",
    countNoun: "show",
    emptyMessage: "Grouped by series — newest anime episodes surface here once your library is scanned.",
  },
];

export function Dashboard() {
  const navigate = useNavigate();
  const { data, error, isLoading, refetch } = useHomeDashboard();
  const { playEpisode, playMovie } = usePlayerQueue();
  const clearMediaProgressMutation = useClearMediaProgress();
  const clearShowProgressMutation = useClearShowProgress();

  const buildDashboardCard = useCallback(
    (entry: DashboardEntry, shelf: DashboardShelf): PosterGridItem => {
      const card = toPosterGridItem(entry, shelf, playMovie, playEpisode);
      const detailHref = dashboardDetailHref(entry);
      return {
        ...card,
        playActionLabel: "Play",
        actionLabel: detailHref ? "Details" : undefined,
        actionTone: detailHref ? "muted" : undefined,
        onAction: detailHref
          ? () => {
              navigate(detailHref);
            }
          : undefined,
        contextMenuContent: (
          <DashboardCardContextMenu
            canOpenDetails={Boolean(detailHref)}
            canRemoveFromContinueWatching={shelf === "continueWatching"}
            removeDisabled={clearMediaProgressMutation.isPending || clearShowProgressMutation.isPending}
            onPlay={() => card.onPlay?.()}
            onOpenDetails={() => {
              if (detailHref) navigate(detailHref);
            }}
            onRemoveFromContinueWatching={() => {
              const isShowEntry = (entry.kind === "show" || entry.kind === "episode") && !!entry.show_key;
              const clearOp = isShowEntry && entry.media.library_id
                ? clearShowProgressMutation.mutateAsync({
                    libraryId: entry.media.library_id,
                    showKey: entry.show_key!,
                  })
                : clearMediaProgressMutation.mutateAsync({
                    mediaId: entry.media.id,
                    libraryId: entry.media.library_id ?? undefined,
                  });
              void clearOp
                .then(() => {
                  toast.success(`Removed “${getDashboardEntryTitle(entry)}” from continue watching.`);
                })
                .catch((err: unknown) => {
                  toast.error(
                    err instanceof Error ? err.message : "Could not remove item from continue watching.",
                  );
                });
            }}
          />
        ),
      };
    },
    [navigate, playMovie, playEpisode, clearMediaProgressMutation, clearShowProgressMutation],
  );

  const continueWatchingCards = useMemo(
    () => data?.continueWatching.map((entry) => buildDashboardCard(entry, "continueWatching")) ?? [],
    [buildDashboardCard, data?.continueWatching],
  );

  const railData = useMemo(
    () =>
      RECENT_RAILS.map((rail) => {
        const entries = data?.[rail.entriesKey] ?? [];
        return {
          ...rail,
          entries,
          cards: entries.map((entry) => buildDashboardCard(entry, rail.shelf)),
        };
      }),
    [buildDashboardCard, data],
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-8">
      <header className="space-y-1">
        <h1 className="text-2xl font-semibold tracking-tight text-(--plum-text)">Home</h1>
        <p className="text-sm text-(--plum-muted)">
          Pick up where you left off and browse what was added recently.
        </p>
      </header>

      {error ? (
        <p className="text-sm text-(--plum-muted)">
          Failed to load home: {error.message}{" "}
          <button
            type="button"
            className="text-(--plum-accent) hover:underline"
            onClick={() => void refetch()}
          >
            Retry
          </button>
        </p>
      ) : null}

      {isLoading ? (
        <PageRouteSkeleton />
      ) : null}

      {!isLoading && !error ? (
        <>
          <section className="flex min-h-0 flex-col gap-4" aria-labelledby="dash-continue-heading">
            <div className="flex items-center justify-between gap-4">
              <h2
                id="dash-continue-heading"
                className="text-lg font-semibold text-(--plum-text)"
                data-testid="dashboard-continue-heading"
              >
                Continue watching
              </h2>
              {data?.continueWatching.length ? (
                <span className="text-sm text-(--plum-muted)">
                  {data.continueWatching.length} active item
                  {data.continueWatching.length === 1 ? "" : "s"}
                </span>
              ) : null}
            </div>

            {continueWatchingCards.length === 0 ? (
              <div className="rounded-(--radius-xl) border border-dashed border-(--plum-border) bg-(--plum-panel)/45 p-8 text-sm text-(--plum-muted)">
                Start a movie or episode and Plum will keep your spot here.
              </div>
            ) : (
              <PosterScrollRail label="Continue watching" items={continueWatchingCards} />
            )}
          </section>

          {railData.map((rail) => {
            const n = rail.entries.length;
            const plural = n === 1 ? "" : "s";
            return (
              <section key={rail.shelf} className="flex flex-col gap-4" aria-labelledby={rail.headingId}>
                <div className="flex items-center justify-between gap-4">
                  <h2
                    id={rail.headingId}
                    className="text-lg font-semibold text-(--plum-text)"
                    data-testid={rail.testId}
                  >
                    {rail.title}
                  </h2>
                  {n > 0 ? (
                    <span className="text-sm text-(--plum-muted)">
                      {n} {rail.countNoun}
                      {plural}
                    </span>
                  ) : null}
                </div>

                {rail.cards.length === 0 ? (
                  <div className="rounded-(--radius-xl) border border-dashed border-(--plum-border) bg-(--plum-panel)/45 p-8 text-sm text-(--plum-muted)">
                    {rail.emptyMessage}
                  </div>
                ) : (
                  <PosterScrollRail label={rail.title} items={rail.cards} />
                )}
              </section>
            );
          })}
        </>
      ) : null}
    </div>
  );
}
