import { useEffect, useMemo, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { BASE_URL, type MediaItem, type ShowSeasonEpisodes } from "../api";
import { PosterPickerDialog } from "../components/PosterPickerDialog";
import { RatingBadge } from "../components/RatingBadge";
import { usePlayer } from "../contexts/PlayerContext";
import { formatRemainingTime, shouldShowProgress } from "../lib/progress";
import { resolveBackdropUrl, resolveCastProfileUrl, resolvePosterUrl } from "@plum/shared";
import { useShowDetails, useShowEpisodes } from "../queries";

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
  const { playShowGroup } = usePlayer();
  const [expandedEpisodeId, setExpandedEpisodeId] = useState<number | null>(null);
  const [selectedSeason, setSelectedSeason] = useState<number | null>(null);
  const [posterPickerOpen, setPosterPickerOpen] = useState(false);

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

  const showTitle =
    details?.name ??
    (episodes.length > 0
      ? episodes[0].title.replace(/\s*-\s*S\d+.*$/i, "").trim()
      : (showKey ?? "Show"));
  const showImdbRating =
    details?.imdb_rating ??
    episodes.find((episode) => (episode.imdb_rating ?? 0) > 0)?.imdb_rating;
  const showTmdbRating =
    details?.vote_average ??
    episodes.find((episode) => (episode.show_vote_average ?? 0) > 0)?.show_vote_average ??
    episodes.find((episode) => (episode.vote_average ?? 0) > 0)?.vote_average;

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
    return <p className="auth-muted">Loading…</p>;
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
    ? resolvePosterUrl(details.poster_url, details.poster_path, "w200", BASE_URL)
    : episodes[0]
      ? resolvePosterUrl(
          episodes[0].show_poster_url ?? episodes[0].poster_url,
          episodes[0].show_poster_path ?? episodes[0].poster_path,
          "w200",
          BASE_URL,
        )
      : "";
  const backdropUrl = details?.backdrop_path
    ? resolveBackdropUrl(details.backdrop_url, details.backdrop_path, "w500", BASE_URL)
    : episodes[0]
      ? resolveBackdropUrl(episodes[0].backdrop_url, episodes[0].backdrop_path, "w500", BASE_URL)
      : "";

  return (
    <div className="show-detail">
      <nav className="show-detail-nav">
        <Link to={libraryId ? `/library/${libraryId}` : "/"} className="link-button">
          ← Back to library
        </Link>
      </nav>
      {backdropUrl && (
        <div className="show-detail-backdrop">
          <img src={backdropUrl} alt="" />
        </div>
      )}
      <div className="show-detail-header">
        {posterUrl && (
          <div
            className="show-detail-poster"
            onContextMenu={(event) => {
              event.preventDefault();
              setPosterPickerOpen(true);
            }}
          >
            <img src={posterUrl} alt="" />
          </div>
        )}
        <div className="show-detail-meta">
          <h1 className="show-detail-title">{showTitle}</h1>
          {showImdbRating || showTmdbRating ? (
            <div className="flex flex-wrap items-center gap-3">
              <RatingBadge label="IMDb" value={showImdbRating} size="md" />
              <RatingBadge label="TMDb" value={showTmdbRating} size="md" />
            </div>
          ) : null}
          {details?.first_air_date && <p className="show-detail-date">{details.first_air_date}</p>}
          {details?.overview && (
            <p className="show-detail-overview">{details.overview}</p>
          )}
          {details?.genres.length ? (
            <div className="flex flex-wrap gap-2">
              {details.genres.map((genre) => (
                <span
                  key={genre}
                  className="rounded-full border border-(--plum-border) px-3 py-1 text-xs uppercase tracking-[0.12em] text-(--plum-muted)"
                >
                  {genre}
                </span>
              ))}
            </div>
          ) : null}
          <div>
            <button
              type="button"
              className="rounded-md border border-(--plum-border) px-4 py-2 text-sm text-(--plum-text) transition-colors hover:bg-(--plum-panel)"
              onClick={() => setPosterPickerOpen(true)}
            >
              Change poster…
            </button>
          </div>
        </div>
      </div>
      {details?.cast.length ? (
        <section className="rounded-(--radius-xl) border border-(--plum-border) bg-(--plum-panel) p-5">
          <h2 className="text-lg font-semibold text-(--plum-text)">Cast</h2>
          <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {details.cast.map((member) => {
              const headshot = resolveCastProfileUrl(undefined, member.profile_path, "w185", BASE_URL);
              const initial = member.name.trim().charAt(0).toUpperCase() || "?";
              return (
                <div
                  key={`${member.name}-${member.character ?? ""}`}
                  className="flex gap-3 rounded-lg border border-(--plum-border) bg-(--plum-panel-alt) p-3"
                >
                  {headshot ? (
                    <img
                      src={headshot}
                      alt=""
                      className="h-[4.5rem] w-12 shrink-0 rounded-md object-cover object-top"
                    />
                  ) : (
                    <div
                      className="flex h-[4.5rem] w-12 shrink-0 items-center justify-center rounded-md bg-(--plum-border) text-sm font-semibold text-(--plum-muted)"
                      aria-hidden
                    >
                      {initial}
                    </div>
                  )}
                  <div className="min-w-0 flex-1">
                    <div className="text-sm font-semibold text-(--plum-text)">{member.name}</div>
                    {member.character ? (
                      <div className="text-xs text-(--plum-muted)">{member.character}</div>
                    ) : null}
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      ) : null}
      {episodes.length === 0 ? (
        <p className="auth-muted">No episodes found for this show.</p>
      ) : (
        <div className="show-detail-seasons">
          <div className="show-detail-season-picker" role="tablist" aria-label="Select season">
            {episodesBySeason.seasons.map((seasonNum: number) => {
              const label = episodesBySeason.labels.get(seasonNum) ?? "";
              const count = episodesBySeason.map.get(seasonNum)?.length ?? 0;
              const isActive = activeSeason === seasonNum;
              return (
                <button
                  key={seasonNum}
                  type="button"
                  role="tab"
                  aria-selected={isActive}
                  className={`show-detail-season-pill${isActive ? " is-active" : ""}`}
                  onClick={() => setSelectedSeason(seasonNum)}
                >
                  <span>{label}</span>
                  <span className="show-detail-season-pill__count">{count}</span>
                </button>
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
                    {ep.release_date && <span className="episode-release-date">{ep.release_date}</span>}
                    {ep.overview && (
                      <button
                        type="button"
                        className="episode-overview-toggle"
                        onClick={() => setExpandedEpisodeId((id) => (id === ep.id ? null : ep.id))}
                      >
                        {expandedEpisodeId === ep.id ? "Hide" : "Show"} summary
                      </button>
                    )}
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
                </li>
              ))}
            </ul>
          </section>
        </div>
      )}
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
