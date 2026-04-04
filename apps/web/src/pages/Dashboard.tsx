import type { HomeDashboard } from "@/api";
import { PosterScrollRail } from "@/components/PosterScrollRail";
import type { PosterGridItem } from "@/components/types";
import { usePlayer } from "@/contexts/PlayerContext";
import { formatRemainingTime } from "@/lib/progress";
import { getPreferredMovieRating } from "@/lib/ratings";
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
  if (entry.kind === "show") {
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
  playEpisode: (item: DashboardEntry["media"]) => void,
): PosterGridItem {
  const playItem =
    entry.kind === "movie" ? () => playMovie(entry.media) : () => playEpisode(entry.media);
  const href = dashboardDetailHref(entry);
  const rating = entry.kind === "movie" ? getPreferredMovieRating(entry.media) : {
    label: entry.media.imdb_rating ? "IMDb" : undefined,
    value: entry.media.imdb_rating,
  };
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
    href,
    onClick: href ? undefined : playItem,
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

  const recentlyAdded = data?.recentlyAdded ?? [];
  const recentlyAddedTv = recentlyAdded.filter((e) => e.kind === "show");
  const recentlyAddedMovies = recentlyAdded.filter((e) => e.kind === "movie");

  const recentlyAddedTvCards: PosterGridItem[] = recentlyAddedTv.map((entry) =>
    toPosterGridItem(entry, "recentlyAdded", playMovie, playEpisode),
  );
  const recentlyAddedMovieCards: PosterGridItem[] = recentlyAddedMovies.map((entry) =>
    toPosterGridItem(entry, "recentlyAdded", playMovie, playEpisode),
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
        <p className="text-sm text-(--plum-muted)">Loading your library…</p>
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

          <section className="flex flex-col gap-4" aria-labelledby="dash-recent-tv-heading">
            <div className="flex items-center justify-between gap-4">
              <h2
                id="dash-recent-tv-heading"
                className="text-lg font-semibold text-(--plum-text)"
                data-testid="dashboard-recent-tv-heading"
              >
                Recently added TV shows
              </h2>
              {recentlyAddedTv.length ? (
                <span className="text-sm text-(--plum-muted)">
                  {recentlyAddedTv.length} show{recentlyAddedTv.length === 1 ? "" : "s"}
                </span>
              ) : null}
            </div>

            {recentlyAddedTvCards.length === 0 ? (
              <div className="rounded-(--radius-xl) border border-dashed border-(--plum-border) bg-(--plum-panel)/45 p-8 text-sm text-(--plum-muted)">
                Scan a TV library and newly added episodes will appear here.
              </div>
            ) : (
              <PosterScrollRail label="Recently added TV shows" items={recentlyAddedTvCards} />
            )}
          </section>

          <section className="flex flex-col gap-4" aria-labelledby="dash-recent-movies-heading">
            <div className="flex items-center justify-between gap-4">
              <h2
                id="dash-recent-movies-heading"
                className="text-lg font-semibold text-(--plum-text)"
                data-testid="dashboard-recent-movies-heading"
              >
                Recently added movies
              </h2>
              {recentlyAddedMovies.length ? (
                <span className="text-sm text-(--plum-muted)">
                  {recentlyAddedMovies.length} film{recentlyAddedMovies.length === 1 ? "" : "s"}
                </span>
              ) : null}
            </div>

            {recentlyAddedMovieCards.length === 0 ? (
              <div className="rounded-(--radius-xl) border border-dashed border-(--plum-border) bg-(--plum-panel)/45 p-8 text-sm text-(--plum-muted)">
                Scan a movie library and the newest additions will show up in this row.
              </div>
            ) : (
              <PosterScrollRail label="Recently added movies" items={recentlyAddedMovieCards} />
            )}
          </section>
        </>
      ) : null}
    </div>
  );
}
