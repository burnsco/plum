import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { resolveBackdropUrl, resolvePosterUrl } from "@plum/shared";
import { BASE_URL, type MediaItem } from "@/api";
import { CastGrid } from "@/components/CastGrid";
import { DetailViewSkeleton } from "@/components/loading/PlumLoadingSkeletons";
import { PosterPickerDialog } from "@/components/PosterPickerDialog";
import { RatingBadge } from "@/components/RatingBadge";
import { usePlayerQueue } from "@/contexts/PlayerContext";
import { useMovieDetails } from "@/queries";
import { fileNameFromPath } from "@/utils/fileNameFromPath";

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
  const { playMovie } = usePlayerQueue();
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
    return <DetailViewSkeleton />;
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
    path: details.source_path?.trim() ?? "",
    duration: details.runtime != null && details.runtime > 0 ? details.runtime * 60 : 0,
    type: "movie",
    tmdb_id: 0,
    overview: details.overview,
    poster_path: details.poster_path ?? "",
    poster_url: details.poster_url,
    backdrop_path: details.backdrop_path ?? "",
    backdrop_url: details.backdrop_url,
    release_date: details.release_date ?? "",
    vote_average: details.vote_average ?? 0,
    imdb_id: details.imdb_id,
    imdb_rating: details.imdb_rating,
    subtitles: details.subtitles,
    embeddedSubtitles: details.embeddedSubtitles,
    embeddedAudioTracks: details.embeddedAudioTracks,
    progress_seconds: details.progress_seconds,
    progress_percent: details.progress_percent,
    completed: details.completed,
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
              {details.source_path?.trim() ? (
                <div className="space-y-1 rounded-lg border border-white/15 bg-black/20 p-3 text-xs text-white/80">
                  <div className="font-medium text-white/90">
                    Source file: {fileNameFromPath(details.source_path)}
                  </div>
                  <div className="break-all font-mono text-white/65">{details.source_path}</div>
                </div>
              ) : null}
            </div>
          </div>
        </div>
      </div>

      <CastGrid members={details.cast} />

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
