import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { resolveBackdropUrl, resolveCastProfileUrl, resolvePosterUrl } from "@plum/shared";
import { BASE_URL, type MediaItem } from "@/api";
import { PosterPickerDialog } from "@/components/PosterPickerDialog";
import { RatingBadge } from "@/components/RatingBadge";
import { usePlayer } from "@/contexts/PlayerContext";
import { useMovieDetails } from "@/queries";

function formatRuntime(minutes?: number): string {
  if (!minutes || minutes <= 0) {
    return "";
  }
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  if (hours <= 0) {
    return `${mins} min`;
  }
  return `${hours}h ${mins}m`;
}

export function MovieDetail() {
  const { libraryId: libraryIdParam, mediaId: mediaIdParam } = useParams();
  const libraryId = libraryIdParam ? Number(libraryIdParam) : null;
  const mediaId = mediaIdParam ? Number(mediaIdParam) : null;
  const { data: details, isLoading, error } = useMovieDetails(libraryId, mediaId);
  const { playMovie } = usePlayer();
  const [posterPickerOpen, setPosterPickerOpen] = useState(false);

  if (libraryId == null || mediaId == null) {
    return (
      <p className="auth-muted">
        <Link to="/" className="link-button">
          Back
        </Link>
      </p>
    );
  }

  if (isLoading) {
    return <p className="text-sm text-(--plum-muted)">Loading movie…</p>;
  }

  if (error) {
    return <p className="text-sm text-(--plum-muted)">{error.message}</p>;
  }

  if (!details) {
    return <p className="text-sm text-(--plum-muted)">Movie not found.</p>;
  }

  const movie: MediaItem = {
    id: mediaId,
    library_id: libraryId,
    title: details.title,
    path: "",
    duration: details.runtime != null && details.runtime > 0 ? details.runtime * 60 : 0,
    type: "movie",
    overview: details.overview,
    poster_path: details.poster_path,
    poster_url: details.poster_url,
    backdrop_path: details.backdrop_path,
    backdrop_url: details.backdrop_url,
    release_date: details.release_date,
    vote_average: details.vote_average,
    imdb_id: details.imdb_id,
    imdb_rating: details.imdb_rating,
    subtitles: details.subtitles,
    embeddedSubtitles: details.embeddedSubtitles,
    embeddedAudioTracks: details.embeddedAudioTracks,
  };
  const posterUrl = resolvePosterUrl(details.poster_url, details.poster_path, "w342", BASE_URL);
  const backdropUrl = resolveBackdropUrl(details.backdrop_url, details.backdrop_path, "w1280", BASE_URL);
  const runtime = formatRuntime(details.runtime);
  const year = details.release_date?.split("-")[0] ?? "";

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-6">
      <div className="detail-hero">
        {backdropUrl ? <img className="detail-hero-bg" src={backdropUrl} alt="" /> : null}
        <div className="detail-hero-scrim" />
        <div className="detail-hero-inner">
          <nav className="show-detail-nav">
            <Link to={`/library/${libraryId}`} className="link-button">
              ← Back to library
            </Link>
          </nav>

          <div className="detail-hero-body">
            {posterUrl ? (
              <div
                className="detail-hero-poster"
                onContextMenu={(event) => {
                  event.preventDefault();
                  setPosterPickerOpen(true);
                }}
              >
                <img src={posterUrl} alt="" />
              </div>
            ) : null}

            <div className="detail-hero-meta">
              <h1 className="detail-hero-title">{details.title}</h1>

              {(year || runtime) ? (
                <p className="detail-hero-chips">
                  {[year, runtime].filter(Boolean).join(" · ")}
                </p>
              ) : null}

              {(details.imdb_rating ?? 0) > 0 || (details.vote_average ?? 0) > 0 ? (
                <div className="flex flex-wrap items-center gap-3">
                  <RatingBadge label="IMDb" value={details.imdb_rating} size="md" />
                  <RatingBadge label="TMDb" value={details.vote_average} size="md" />
                </div>
              ) : null}

              {details.overview ? (
                <p className="detail-hero-overview">{details.overview}</p>
              ) : null}

              {details.genres.length ? (
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

              <div className="flex flex-wrap gap-3">
                <button type="button" className="play-button" onClick={() => playMovie(movie)}>
                  Play
                </button>
                <button
                  type="button"
                  className="rounded-md border border-white/20 px-4 py-2 text-sm text-white/75 transition-colors hover:bg-white/10"
                  onClick={() => setPosterPickerOpen(true)}
                >
                  Change poster…
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <section className="rounded-(--radius-xl) border border-(--plum-border) bg-(--plum-panel) p-5">
        <h2 className="text-lg font-semibold text-(--plum-text)">Cast</h2>
        {details.cast.length === 0 ? (
          <p className="mt-3 text-sm text-(--plum-muted)">No cast metadata yet.</p>
        ) : (
          <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {details.cast.map((member) => {
              const headshot = resolveCastProfileUrl(undefined, member.profile_path, "w185", BASE_URL);
              const initial = member.name.trim().charAt(0).toUpperCase() || "?";
              return (
                <Link
                  key={`${member.name}-${member.character ?? ""}`}
                  to={`/search?q=${encodeURIComponent(member.name)}`}
                  className="flex gap-3 rounded-lg border border-(--plum-border) bg-(--plum-panel-alt) p-3 transition-colors hover:border-(--plum-accent)/50 hover:bg-(--plum-panel)"
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
                </Link>
              );
            })}
          </div>
        )}
      </section>

      <PosterPickerDialog
        open={posterPickerOpen}
        onOpenChange={setPosterPickerOpen}
        kind="movie"
        libraryId={libraryId}
        mediaId={mediaId}
        title={details.title}
      />
    </div>
  );
}
