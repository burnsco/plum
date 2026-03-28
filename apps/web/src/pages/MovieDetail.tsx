import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { resolveBackdropUrl, resolvePosterUrl } from "@plum/shared";
import type { MediaItem } from "@/api";
import { PosterPickerDialog } from "@/components/PosterPickerDialog";
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
    return <p className="text-sm text-[var(--plum-muted)]">Loading movie…</p>;
  }

  if (error) {
    return <p className="text-sm text-[var(--plum-muted)]">{error.message}</p>;
  }

  if (!details) {
    return <p className="text-sm text-[var(--plum-muted)]">Movie not found.</p>;
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
    imdb_id: details.imdb_id,
    imdb_rating: details.imdb_rating,
  };
  const posterUrl = resolvePosterUrl(details.poster_url, details.poster_path);
  const backdropUrl = resolveBackdropUrl(details.backdrop_url, details.backdrop_path);
  const runtime = formatRuntime(details.runtime);
  const year = details.release_date?.split("-")[0] ?? "";

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-6">
      <nav className="show-detail-nav">
        <Link to={`/library/${libraryId}`} className="link-button">
          ← Back to library
        </Link>
      </nav>

      {backdropUrl ? (
        <div className="show-detail-backdrop">
          <img src={backdropUrl} alt="" />
        </div>
      ) : null}

      <section className="show-detail-header">
        {posterUrl ? (
          <div
            className="show-detail-poster"
            onContextMenu={(event) => {
              event.preventDefault();
              setPosterPickerOpen(true);
            }}
          >
            <img src={posterUrl} alt="" />
          </div>
        ) : null}

        <div className="show-detail-meta space-y-4">
          <div className="space-y-2">
            <h1 className="show-detail-title">{details.title}</h1>
            <div className="flex flex-wrap gap-2 text-sm text-[var(--plum-muted)]">
              {year ? <span>{year}</span> : null}
              {runtime ? <span>{runtime}</span> : null}
              {details.imdb_rating ? <span>IMDb {details.imdb_rating.toFixed(1)}</span> : null}
            </div>
          </div>

          {details.overview ? (
            <p className="show-detail-overview">{details.overview}</p>
          ) : null}

          {details.genres.length ? (
            <div className="flex flex-wrap gap-2">
              {details.genres.map((genre) => (
                <span
                  key={genre}
                  className="rounded-full border border-[var(--plum-border)] px-3 py-1 text-xs uppercase tracking-[0.12em] text-[var(--plum-muted)]"
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
              className="rounded-[var(--radius-md)] border border-[var(--plum-border)] px-4 py-2 text-sm text-[var(--plum-text)] transition-colors hover:bg-[var(--plum-panel)]"
              onClick={() => setPosterPickerOpen(true)}
            >
              Change poster…
            </button>
          </div>
        </div>
      </section>

      <section className="rounded-[var(--radius-xl)] border border-[var(--plum-border)] bg-[var(--plum-panel)] p-5">
        <h2 className="text-lg font-semibold text-[var(--plum-text)]">Cast</h2>
        {details.cast.length === 0 ? (
          <p className="mt-3 text-sm text-[var(--plum-muted)]">No cast metadata yet.</p>
        ) : (
          <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {details.cast.map((member) => (
              <div
                key={`${member.name}-${member.character ?? ""}`}
                className="rounded-[var(--radius-lg)] border border-[var(--plum-border)] bg-[var(--plum-panel-alt)] p-3"
              >
                <div className="text-sm font-semibold text-[var(--plum-text)]">{member.name}</div>
                {member.character ? (
                  <div className="text-xs text-[var(--plum-muted)]">{member.character}</div>
                ) : null}
              </div>
            ))}
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
