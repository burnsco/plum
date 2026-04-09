import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Image, ListChecks } from "lucide-react";
import { Link, useParams } from "react-router-dom";
import { toast } from "sonner";
import { BASE_URL, type MediaItem, type ShowSeasonEpisodes } from "../api";
import { CastGrid } from "@/components/CastGrid";
import { LibraryMediaContextMenu } from "@/components/LibraryMediaContextMenu";
import { DetailViewSkeleton } from "@/components/loading/PlumLoadingSkeletons";
import { PosterPickerDialog } from "../components/PosterPickerDialog";
import { RatingBadge } from "../components/RatingBadge";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import { usePlayerQueue } from "../contexts/PlayerContext";
import { formatEpisodeLabel, formatRemainingTime, shouldShowProgress } from "../lib/progress";
import { resolveBackdropUrl, resolvePosterUrl } from "@plum/shared";
import { fileNameFromPath } from "@/utils/fileNameFromPath";
import { useMarkShowWatched, useShowDetails, useShowEpisodes, useUpdateMediaProgress } from "../queries";

function formatDuration(seconds: number): string {
  if (seconds <= 0) return "";
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return s > 0 ? `${m}:${s.toString().padStart(2, "0")}` : `${m} min`;
}

function seasonEpisodeLabel(item: MediaItem): string {
  const s = item.season ?? 0;
  const e = item.episode ?? 0;
  if (s > 0 || e > 0) return `S${String(s).padStart(2, "0")}E${String(e).padStart(2, "0")}`;
  return "";
}

export function ShowDetail() {
  const { libraryId: libraryIdParam, showKey: showKeyEncoded } = useParams();
  const libraryId = libraryIdParam ? parseInt(libraryIdParam, 10) : null;
  const showKey = showKeyEncoded ? decodeURIComponent(showKeyEncoded) : null;

  const { data: episodesData, isLoading: loading, error } = useShowEpisodes(libraryId, showKey);
  const { data: details } = useShowDetails(libraryId, showKey);
  const { playShowGroup } = usePlayerQueue();
  const markShowWatchedMutation = useMarkShowWatched();
  const updateProgressMutation = useUpdateMediaProgress();
  const [expandedEpisodeId, setExpandedEpisodeId] = useState<number | null>(null);
  const [selectedSeason, setSelectedSeason] = useState<number | null>(null);
  const [posterPickerOpen, setPosterPickerOpen] = useState(false);
  const activeSeasonPillRef = useRef<HTMLButtonElement | null>(null);

  const episodes = useMemo<MediaItem[]>(() => {
    if (!episodesData?.seasons) return [];
    const out: MediaItem[] = [];
    for (const s of episodesData.seasons) {
      for (const e of s.episodes) {
        out.push(e as MediaItem);
      }
    }
    return out;
  }, [episodesData]);

  /** Episodes sorted ascending by season then episode number. */
  const sortedEpisodes = useMemo(
    () =>
      episodes.toSorted((a, b) => {
        const s = (a.season ?? 0) - (b.season ?? 0);
        return s !== 0 ? s : (a.episode ?? 0) - (b.episode ?? 0);
      }),
    [episodes],
  );

  /**
   * The episode to resume:
   * 1. In-progress episode (has progress, not completed) – most recently watched first.
   * 2. The episode right after the most-recently-watched completed episode.
   */
  const resumeEpisode = useMemo<MediaItem | null>(() => {
    if (sortedEpisodes.length === 0) return null;
    const inProgress = sortedEpisodes
      .filter((ep) => shouldShowProgress(ep))
      .toSorted((a, b) => (b.last_watched_at ?? "").localeCompare(a.last_watched_at ?? ""))[0];
    if (inProgress) return inProgress;
    const lastWatched = sortedEpisodes
      .filter((ep) => ep.last_watched_at)
      .toSorted((a, b) => (b.last_watched_at ?? "").localeCompare(a.last_watched_at ?? ""))[0];
    if (lastWatched) {
      const idx = sortedEpisodes.findIndex((ep) => ep.id === lastWatched.id);
      if (idx >= 0 && idx + 1 < sortedEpisodes.length) return sortedEpisodes[idx + 1];
    }
    return null;
  }, [sortedEpisodes]);

  /** Episodes grouped by season number, with seasons sorted ascending. */
  const episodesBySeason = useMemo(() => {
    if (!episodesData?.seasons) {
      return { map: new Map<number, MediaItem[]>(), labels: new Map<number, string>(), seasons: [] as number[] };
    }
    const map = new Map<number, MediaItem[]>();
    const labels = new Map<number, string>();
    for (const s of episodesData.seasons) {
      map.set(s.seasonNumber, s.episodes as MediaItem[]);
      labels.set(s.seasonNumber, s.label);
    }
    const seasons = episodesData.seasons.map((s: ShowSeasonEpisodes) => s.seasonNumber).toSorted((a: number, b: number) => a - b);
    return { map, labels, seasons };
  }, [episodesData]);
  const activeSeason = selectedSeason ?? episodesBySeason.seasons[0] ?? null;
  const activeSeasonEpisodes =
    activeSeason == null ? [] : (episodesBySeason.map.get(activeSeason) ?? []);
  const activeSeasonLabel = activeSeason == null ? "" : (episodesBySeason.labels.get(activeSeason) ?? "");

  const markActionsDisabled =
    libraryId == null ||
    showKey == null ||
    markShowWatchedMutation.isPending ||
    sortedEpisodes.length === 0;

  const showTitle =
    details?.name ??
    (episodes.length > 0
      ? episodes[0].title.replace(/\s*-\s*S\d+.*$/i, "").trim()
      : (showKey ?? "Show"));
  const showImdbRating =
    details?.imdb_rating ??
    episodes.find((episode) => (episode.show_imdb_rating ?? 0) > 0)?.show_imdb_rating;
  const showTmdbRating =
    details?.vote_average ??
    episodes.find((episode) => (episode.show_vote_average ?? 0) > 0)?.show_vote_average;

  useEffect(() => {
    if (episodesBySeason.seasons.length === 0) {
      setSelectedSeason(null);
      return;
    }
    setSelectedSeason((current) =>
      current != null && episodesBySeason.seasons.includes(current)
        ? current
        : episodesBySeason.seasons[0],
    );
  }, [episodesBySeason.seasons]);

  useLayoutEffect(() => {
    activeSeasonPillRef.current?.scrollIntoView({
      behavior: "smooth",
      block: "nearest",
      inline: "nearest",
    });
  }, [activeSeason]);

  if (libraryId == null || showKey == null) {
    return (
      <p className="auth-muted">
        <Link to="/" className="link-button">
          Back to library
        </Link>
      </p>
    );
  }

  if (loading) {
    return <DetailViewSkeleton />;
  }

  if (error) {
    return (
      <p className="auth-muted">
        {error.message} ·{" "}
        <Link to={`/library/${libraryId}`} className="link-button">
          Back
        </Link>
      </p>
    );
  }

  const posterUrl = details?.poster_path
    ? resolvePosterUrl(details.poster_url, details.poster_path, "w342", BASE_URL)
    : episodes[0]
      ? resolvePosterUrl(
          episodes[0].show_poster_url ?? episodes[0].poster_url,
          episodes[0].show_poster_path ?? episodes[0].poster_path,
          "w342",
          BASE_URL,
        )
      : "";
  const backdropUrl = details?.backdrop_path
    ? resolveBackdropUrl(details.backdrop_url, details.backdrop_path, "w1280", BASE_URL)
    : episodes[0]
      ? resolveBackdropUrl(episodes[0].backdrop_url, episodes[0].backdrop_path, "w1280", BASE_URL)
      : "";

  return (
    <div className="show-detail">
      <div className="detail-hero">
        {backdropUrl ? <img className="detail-hero-bg" src={backdropUrl} alt="" /> : null}
        <div className="detail-hero-scrim" />
        <div className="detail-hero-inner">
          <nav className="show-detail-nav">
            <Link to={libraryId ? `/library/${libraryId}` : "/"} className="link-button">
              ← Back to library
            </Link>
          </nav>

          <div className="detail-hero-body">
            {posterUrl ? (
              <LibraryMediaContextMenu
                menu={
                  <>
                    <ContextMenuItem
                      disabled={markActionsDisabled}
                      onSelect={() => {
                        if (libraryId == null || showKey == null) return;
                        const n = sortedEpisodes.length;
                        void markShowWatchedMutation
                          .mutateAsync({
                            libraryId,
                            showKey,
                            payload: { mode: "all" },
                          })
                          .then(() => {
                            toast.success(
                              n === 1
                                ? "Marked one episode as watched."
                                : `Marked all ${n} episodes as watched.`,
                            );
                          })
                          .catch(() => {
                            /* mutation onError */
                          });
                      }}
                    >
                      <ListChecks className="size-4 text-(--plum-muted)" />
                      Mark show as watched
                    </ContextMenuItem>
                    <ContextMenuItem onSelect={() => setPosterPickerOpen(true)}>
                      <Image className="size-4 text-(--plum-muted)" />
                      Change poster…
                    </ContextMenuItem>
                  </>
                }
              >
                <div className="detail-hero-poster">
                  <img src={posterUrl} alt="" />
                </div>
              </LibraryMediaContextMenu>
            ) : null}

            <div className="detail-hero-meta">
              <h1 className="detail-hero-title">{showTitle}</h1>

              {details?.first_air_date ? (
                <p className="detail-hero-chips">{details.first_air_date.split("-")[0]}</p>
              ) : null}

              {showImdbRating || showTmdbRating ? (
                <div className="flex flex-wrap items-center gap-3">
                  <RatingBadge label="IMDb" value={showImdbRating} size="md" />
                  <RatingBadge label="TMDb" value={showTmdbRating} size="md" />
                </div>
              ) : null}

              {details?.overview ? (
                <p className="detail-hero-overview">{details.overview}</p>
              ) : null}

              {details?.genres.length ? (
                <div className="flex flex-wrap gap-2">
                  {details.genres.map((genre) => (
                    <span
                      key={genre}
                      className="rounded-full border border-white/20 px-3 py-1 text-xs uppercase tracking-[0.12em] text-white/60"
                    >
                      {genre}
                    </span>
                  ))}
                </div>
              ) : null}

              <div className="detail-hero-actions">
                <button
                  type="button"
                  className="play-button detail-hero-play"
                  onClick={() => playShowGroup(sortedEpisodes, sortedEpisodes[0])}
                >
                  ▶ Play
                </button>
                {resumeEpisode && (
                  <button
                    type="button"
                    className="detail-hero-resume"
                    onClick={() => playShowGroup(sortedEpisodes, resumeEpisode)}
                  >
                    Resume
                    <span className="detail-hero-resume-label">
                      {formatEpisodeLabel(resumeEpisode)}
                      {shouldShowProgress(resumeEpisode) && resumeEpisode.remaining_seconds
                        ? ` · ${formatRemainingTime(resumeEpisode.remaining_seconds)}`
                        : ""}
                    </span>
                  </button>
                )}
                <button
                  type="button"
                  className="detail-hero-ghost-button"
                  onClick={() => setPosterPickerOpen(true)}
                >
                  Change poster…
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
      {episodes.length === 0 ? (
        <p className="auth-muted">No episodes found for this show.</p>
      ) : (
        <div className="show-detail-seasons">
          <div className="show-detail-season-picker" role="group" aria-label="Select season">
            {episodesBySeason.seasons.map((seasonNum: number) => {
              const label = episodesBySeason.labels.get(seasonNum) ?? "";
              const count = episodesBySeason.map.get(seasonNum)?.length ?? 0;
              const isActive = activeSeason === seasonNum;
              return (
                <ContextMenu key={seasonNum}>
                  <ContextMenuTrigger asChild>
                    <button
                      ref={isActive ? activeSeasonPillRef : undefined}
                      type="button"
                      aria-current={isActive ? "true" : undefined}
                      className={`show-detail-season-pill${isActive ? " is-active" : ""}`}
                      onClick={() => setSelectedSeason(seasonNum)}
                    >
                      <span>{label}</span>
                      <span className="show-detail-season-pill__count">{count}</span>
                    </button>
                  </ContextMenuTrigger>
                  <ContextMenuContent>
                    <ContextMenuItem
                      disabled={markActionsDisabled}
                      onSelect={() => {
                        if (libraryId == null || showKey == null) return;
                        void markShowWatchedMutation
                          .mutateAsync({
                            libraryId,
                            showKey,
                            payload: { mode: "season", season: seasonNum },
                          })
                          .then(() => {
                            toast.success(`Marked “${label || `Season ${seasonNum}`}” as watched.`);
                          })
                          .catch(() => {
                            /* mutation onError */
                          });
                      }}
                    >
                      <ListChecks className="size-4 text-(--plum-muted)" />
                      Mark season as watched
                    </ContextMenuItem>
                  </ContextMenuContent>
                </ContextMenu>
              );
            })}
          </div>

          <section className="show-detail-season">
            <h2 className="show-detail-season-title">
              {activeSeasonLabel}
              <span className="show-detail-season-count">
                {activeSeasonEpisodes.length} episode{activeSeasonEpisodes.length !== 1 ? "s" : ""}
              </span>
            </h2>
            <ul className="episodes-list show-detail-episodes">
              {activeSeasonEpisodes.map((ep) => (
                <li key={ep.id} className="episode-row episode-row-detail">
                  <ContextMenu>
                    <ContextMenuTrigger asChild>
                      <div className="contents">
                        <div className="episode-thumbnail-wrap">
                          <img
                            src={
                              resolvePosterUrl(ep.poster_url, ep.poster_path, "w200", BASE_URL) ||
                              ep.thumbnail_url ||
                              `${BASE_URL}/api/media/${ep.id}/thumbnail`
                            }
                            alt=""
                            className="episode-thumbnail"
                          />
                        </div>
                        <span className="episode-season-ep" title={ep.title}>
                          {seasonEpisodeLabel(ep)}
                        </span>
                        <div className="episode-info">
                          <span className="episode-title" title={ep.title}>
                            {ep.title}
                          </span>
                          {ep.match_status && ep.match_status !== "identified" && (
                            <span className="episode-release-date">{ep.match_status}</span>
                          )}
                          {ep.release_date && (
                            <span className="episode-release-date">{ep.release_date}</span>
                          )}
                          {ep.overview && (
                            <button
                              type="button"
                              className="episode-overview-toggle"
                              onClick={() =>
                                setExpandedEpisodeId((id) => (id === ep.id ? null : ep.id))
                              }
                            >
                              {expandedEpisodeId === ep.id ? "Hide" : "Show"} summary
                            </button>
                          )}
                          {ep.path.trim() ? (
                            <div className="mt-2 rounded-md border border-(--plum-border) bg-(--plum-panel-alt) p-2">
                              <div className="text-[11px] font-medium text-(--plum-text)">
                                Source file: {fileNameFromPath(ep.path)}
                              </div>
                              <div className="break-all font-mono text-[11px] text-(--plum-muted)">
                                {ep.path}
                              </div>
                            </div>
                          ) : null}
                          {expandedEpisodeId === ep.id && ep.overview && (
                            <p className="episode-overview">{ep.overview}</p>
                          )}
                          {shouldShowProgress(ep) && (
                            <div className="mt-2 flex flex-col gap-1">
                              <div className="h-1.5 overflow-hidden rounded-full bg-white/10">
                                <div
                                  className="h-full rounded-full bg-[#f7c44f]"
                                  style={{ width: `${ep.progress_percent ?? 0}%` }}
                                />
                              </div>
                              <span className="text-xs text-(--plum-muted)">
                                {formatRemainingTime(ep.remaining_seconds)}
                              </span>
                            </div>
                          )}
                        </div>
                        {ep.duration > 0 && (
                          <span className="episode-duration">{formatDuration(ep.duration)}</span>
                        )}
                        <button
                          type="button"
                          className="play-button small"
                          onClick={() => playShowGroup(episodes, ep)}
                        >
                          Play
                        </button>
                      </div>
                    </ContextMenuTrigger>
                    <ContextMenuContent>
                      <ContextMenuItem
                        disabled={updateProgressMutation.isPending}
                        onSelect={() => {
                          if (libraryId == null) return;
                          void updateProgressMutation
                            .mutateAsync({
                              mediaId: ep.id,
                              payload: {
                                position_seconds: 0,
                                duration_seconds: ep.duration > 0 ? ep.duration : 1,
                                completed: true,
                              },
                              libraryId,
                            })
                            .then(() => {
                              toast.success(`Marked ${formatEpisodeLabel(ep)} as watched.`);
                            })
                            .catch(() => {
                              /* mutation onError */
                            });
                        }}
                      >
                        <ListChecks className="size-4 text-(--plum-muted)" />
                        Mark as watched
                      </ContextMenuItem>
                      <ContextMenuItem
                        disabled={markActionsDisabled}
                        onSelect={() => {
                          if (libraryId == null || showKey == null) return;
                          const s = ep.season ?? 0;
                          const e = ep.episode ?? 0;
                          void markShowWatchedMutation
                            .mutateAsync({
                              libraryId,
                              showKey,
                              payload: { mode: "up_to", season: s, episode: e },
                            })
                            .then(() => {
                              const label = formatEpisodeLabel(ep);
                              toast.success(
                                label
                                  ? `Marked every episode before ${label} as watched.`
                                  : "Marked earlier episodes as watched.",
                              );
                            })
                            .catch(() => {
                              /* mutation onError */
                            });
                        }}
                      >
                        <ListChecks className="size-4 text-(--plum-muted)" />
                        Mark earlier episodes as watched
                      </ContextMenuItem>
                    </ContextMenuContent>
                  </ContextMenu>
                </li>
              ))}
            </ul>
          </section>
        </div>
      )}
      <CastGrid members={details?.cast ?? []} hideWhenEmpty />
      <PosterPickerDialog
        open={posterPickerOpen}
        onOpenChange={setPosterPickerOpen}
        kind="show"
        libraryId={libraryId}
        showKey={showKey}
        title={showTitle}
      />
    </div>
  );
}
