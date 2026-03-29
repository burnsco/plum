import type { HomeDashboard } from "@/api";
import { LibraryPosterGrid } from "@/components/LibraryPosterGrid";
import type { PosterGridItem } from "@/components/types";
import { EmptyState, PageHeader } from "@/components/ui/page";
import { usePlayer } from "@/contexts/PlayerContext";
import { formatRemainingTime } from "@/lib/progress";
import { useHomeDashboard } from "@/queries";

type DashboardEntry =
  | HomeDashboard["continueWatching"][number]
  | NonNullable<HomeDashboard["recentlyAdded"]>[number];
type DashboardShelf = "continueWatching" | "recentlyAdded";

function getDashboardEntryTitle(entry: DashboardEntry): string {
  return entry.kind === "show" ? entry.show_title || entry.media.title : entry.media.title;
}

function getDashboardEntrySubtitle(entry: DashboardEntry, shelf: DashboardShelf): string {
  const remainingSeconds = "remaining_seconds" in entry ? entry.remaining_seconds : undefined;
  if (entry.kind === "show") {
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

function toPosterGridItem(
  entry: DashboardEntry,
  shelf: DashboardShelf,
  playMovie: (item: DashboardEntry["media"]) => void,
  playEpisode: (item: DashboardEntry["media"]) => void,
): PosterGridItem {
  const playItem =
    entry.kind === "movie" ? () => playMovie(entry.media) : () => playEpisode(entry.media);

  return {
    key: `${shelf}-${entry.kind}-${entry.media.id}`,
    title: getDashboardEntryTitle(entry),
    subtitle: getDashboardEntrySubtitle(entry, shelf),
    posterPath: entry.media.poster_path,
    posterUrl: entry.media.poster_url,
    ratingLabel: entry.media.imdb_rating ? "IMDb" : undefined,
    ratingValue: entry.media.imdb_rating,
    progressPercent: shelf === "continueWatching" ? entry.media.progress_percent : undefined,
    href: undefined,
    onClick: playItem,
    onPlay: playItem,
  };
}

export function Dashboard() {
  const { data, error, isLoading, refetch } = useHomeDashboard();
  const { playEpisode, playMovie } = usePlayer();

  const continueWatchingCards: PosterGridItem[] =
    data?.continueWatching.map((entry) =>
      toPosterGridItem(entry, "continueWatching", playMovie, playEpisode),
    ) ?? [];
  const recentlyAddedCards: PosterGridItem[] =
    data?.recentlyAdded?.map((entry) =>
      toPosterGridItem(entry, "recentlyAdded", playMovie, playEpisode),
    ) ?? [];

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-8">
      <PageHeader
        title="Home"
        description="Pick up where you left off and keep an eye on what was added most recently across your libraries."
      />

      <section className="flex min-h-0 flex-1 flex-col gap-4">
        <div className="flex items-center justify-between gap-4">
          <h2 className="text-lg font-semibold text-[var(--plum-text)]">Recent progress</h2>
          {data?.continueWatching.length ? (
            <span className="text-sm text-[var(--plum-muted)]">
              {data.continueWatching.length} active item
              {data.continueWatching.length === 1 ? "" : "s"}
            </span>
          ) : null}
        </div>

        {isLoading ? (
          <p className="text-sm text-[var(--plum-muted)]">Loading continue watching…</p>
        ) : error ? (
          <p className="text-sm text-[var(--plum-muted)]">
            Failed to load home: {error.message}{" "}
            <button
              type="button"
              className="text-[var(--plum-accent)] hover:underline"
              onClick={() => void refetch()}
            >
              Retry
            </button>
          </p>
        ) : continueWatchingCards.length === 0 ? (
          <EmptyState
            title="Nothing in progress yet"
            copy="Start a movie or episode and Plum will keep your place here."
          />
        ) : (
          <LibraryPosterGrid items={continueWatchingCards} aspectRatio="cinema" />
        )}
      </section>

      <section className="flex min-h-0 flex-col gap-4">
        <div className="flex items-center justify-between gap-4">
          <h2 className="text-lg font-semibold text-[var(--plum-text)]">Recently added</h2>
          {data?.recentlyAdded?.length ? (
            <span className="text-sm text-[var(--plum-muted)]">
              {data.recentlyAdded.length} new item{data.recentlyAdded.length === 1 ? "" : "s"}
            </span>
          ) : null}
        </div>

        {isLoading ? (
          <p className="text-sm text-[var(--plum-muted)]">Loading recently added…</p>
        ) : error ? (
          <p className="text-sm text-[var(--plum-muted)]">
            Failed to load home: {error.message}{" "}
            <button
              type="button"
              className="text-[var(--plum-accent)] hover:underline"
              onClick={() => void refetch()}
            >
              Retry
            </button>
          </p>
        ) : recentlyAddedCards.length === 0 ? (
          <EmptyState
            title="No recent additions yet"
            copy="Scan your libraries and Plum will surface the newest additions here."
          />
        ) : (
          <LibraryPosterGrid items={recentlyAddedCards} aspectRatio="cinema" />
        )}
      </section>
    </div>
  );
}
