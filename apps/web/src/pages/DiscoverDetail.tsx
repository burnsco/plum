import { Link, useNavigate, useParams } from "react-router-dom";
import { ExternalLink, Film, Tv } from "lucide-react";
import { tmdbBackdropUrl, tmdbPosterUrl } from "@plum/shared";
import { RatingBadge } from "@/components/RatingBadge";
import { DetailViewSkeleton } from "@/components/loading/PlumLoadingSkeletons";
import { Button } from "@/components/ui/button";
import type { DiscoverMediaType } from "@/api";
import { useAuthState } from "@/contexts/AuthContext";
import {
  discoverAcquisitionLabel,
  discoverAcquisitionTone,
  discoverDetailMeta,
  discoverLibraryHref,
  discoverMediaLabel,
  discoverVideoUrl,
  firstDiscoverMatch,
} from "@/lib/discover";
import { useAddDiscoverTitle, useDiscoverTitleDetails } from "@/queries";

function isDiscoverMediaType(value: string | undefined): value is DiscoverMediaType {
  return value === "movie" || value === "tv";
}

function isDiscoverConfigError(error: Error | null): boolean {
  return error?.message.includes("TMDB_API_KEY") ?? false;
}

export function DiscoverDetail() {
  const { user } = useAuthState();
  const isAdmin = user?.is_admin ?? false;
  const navigate = useNavigate();
  const { mediaType: mediaTypeParam, tmdbId: tmdbIdParam } = useParams();
  const mediaType = isDiscoverMediaType(mediaTypeParam) ? mediaTypeParam : null;
  const tmdbId = tmdbIdParam ? Number.parseInt(tmdbIdParam, 10) : null;
  const addTitle = useAddDiscoverTitle();
  const {
    data: details,
    error,
    isLoading,
    refetch,
  } = useDiscoverTitleDetails(mediaType, tmdbId, { refetchInterval: 15_000 });

  if (mediaType == null || tmdbId == null || Number.isNaN(tmdbId) || tmdbId <= 0) {
    return (
      <DiscoverDetailMessage
        title="Invalid title"
        copy="The discover title you opened is missing a valid TMDB media type or id."
      />
    );
  }

  if (isLoading) {
    return <DetailViewSkeleton />;
  }

  if (isDiscoverConfigError(error)) {
    return (
      <DiscoverDetailMessage
        title="Discover needs TMDB configured"
        copy="Set `TMDB_API_KEY` on the server to enable external title details."
        actionLabel="Retry"
        onAction={() => void refetch()}
      />
    );
  }

  if (error) {
    return (
      <DiscoverDetailMessage
        title="Could not load this title"
        copy={error.message}
        actionLabel="Retry"
        onAction={() => void refetch()}
      />
    );
  }

  if (!details) {
    return (
      <DiscoverDetailMessage
        title="Title not found"
        copy="This discover title is no longer available from TMDB."
      />
    );
  }

  const primaryMatch = firstDiscoverMatch(details.library_matches);
  const backdropUrl = tmdbBackdropUrl(details.backdrop_path, "w780");
  const posterUrl = tmdbPosterUrl(details.poster_path, "w500");
  const meta = discoverDetailMeta(details);
  const videos = details.videos
    .map((video) => ({ ...video, href: discoverVideoUrl(video) }))
    .filter((video) => video.href !== "")
    .slice(0, 6);
  const addPending =
    addTitle.isPending &&
    addTitle.variables?.mediaType === details.media_type &&
    addTitle.variables?.tmdbId === details.tmdb_id;
  const needsSetup =
    details.acquisition?.is_configured === false &&
    details.acquisition?.state === "not_added" &&
    isAdmin;
  const actionLabel = needsSetup
    ? "Open Media Stack Settings"
    : discoverAcquisitionLabel(details.acquisition, addPending);
  const actionTone = needsSetup
    ? "default"
    : discoverAcquisitionTone(details.acquisition, addPending);
  const canAdd = details.acquisition?.can_add === true;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-8">
      <section className="relative overflow-hidden rounded-(--radius-xl) border border-(--plum-border) bg-(--plum-panel) shadow-[0_24px_80px_rgba(8,12,24,0.16)]">
        <div className="absolute inset-0">
          {backdropUrl ? (
            <img src={backdropUrl} alt="" className="h-full w-full object-cover opacity-35" />
          ) : null}
          <div className="absolute inset-0 bg-[linear-gradient(120deg,rgba(12,17,30,0.96),rgba(12,17,30,0.72),rgba(12,17,30,0.86))]" />
        </div>

        <div className="relative grid gap-6 p-6 lg:grid-cols-[220px_minmax(0,1fr)] lg:p-8">
          <div className="overflow-hidden rounded-lg border border-white/10 bg-black/20 shadow-[0_20px_55px_rgba(0,0,0,0.28)]">
            {posterUrl ? (
              <img src={posterUrl} alt="" className="h-full w-full object-cover" />
            ) : (
              <img src="/placeholder-poster.svg" alt="" className="h-full w-full object-cover" />
            )}
          </div>

          <div className="flex flex-col gap-5 text-white">
            <div className="flex flex-wrap items-center gap-3 text-xs font-semibold uppercase tracking-[0.2em] text-white/65">
              <Link to="/discover" className="transition-colors hover:text-white">
                Discover
              </Link>
              <span>/</span>
              <span>{discoverMediaLabel(details.media_type)}</span>
            </div>

            <div className="space-y-3">
              <div className="flex flex-wrap items-center gap-3">
                <span className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/10 px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] text-white/75">
                  {details.media_type === "movie" ? (
                    <Film className="size-3.5" />
                  ) : (
                    <Tv className="size-3.5" />
                  )}
                  {discoverMediaLabel(details.media_type)}
                </span>
                {(details.library_matches?.length ?? 0) > 0 ? (
                  <span className="rounded-full bg-(--plum-accent) px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] text-white shadow-[0_0_18px_rgba(244,90,160,0.35)]">
                    In Library
                  </span>
                ) : (
                  <span className="rounded-full border border-white/10 bg-white/8 px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] text-white/70">
                    Not In Server Yet
                  </span>
                )}
              </div>

              <div className="space-y-2">
                <h1 className="text-3xl font-semibold tracking-tight">{details.title}</h1>
                <div className="flex flex-wrap items-center gap-3 text-sm text-white/70">
                  {meta.map((entry) => (
                    <span key={entry}>{entry}</span>
                  ))}
                </div>
              </div>
            </div>

            <div className="flex flex-wrap items-center gap-4 text-sm text-white/80">
              <RatingBadge label="TMDb" value={details.vote_average} size="md" />
              <RatingBadge label="IMDb" value={details.imdb_rating} size="md" />
            </div>

            <p className="max-w-3xl text-sm leading-7 text-white/78">{details.overview}</p>

            <div className="flex flex-wrap gap-3">
              {primaryMatch ? (
                <Button asChild>
                  <Link to={discoverLibraryHref(primaryMatch)}>Open in Library</Link>
                </Button>
              ) : (
                <Button
                  variant={actionTone === "success" ? "secondary" : "default"}
                  disabled={!needsSetup && (!canAdd || addPending)}
                  onClick={() => {
                    if (needsSetup) {
                      navigate("/settings");
                      return;
                    }
                    if (canAdd) {
                      addTitle.mutate({ mediaType: details.media_type, tmdbId: details.tmdb_id });
                    }
                  }}
                >
                  {actionLabel}
                </Button>
              )}
              <Button asChild variant="outline">
                <Link to="/discover">Back to Discover</Link>
              </Button>
            </div>

            {!primaryMatch && details.acquisition?.is_configured === false ? (
              <p className="text-sm text-white/68">
                {isAdmin
                  ? "Configure Radarr and Sonarr TV in Settings to enable direct adds from Discover."
                  : "Direct add is unavailable until an admin configures the media stack."}
              </p>
            ) : null}

            {(details.library_matches?.length ?? 0) > 0 ? (
              <div className="rounded-lg border border-white/10 bg-black/20 p-4">
                <div className="text-sm font-medium text-white">Available in Plum</div>
                <div className="mt-3 flex flex-wrap gap-2">
                  {details.library_matches?.map((match) => (
                    <Link
                      key={`${match.library_id}-${match.kind}-${match.show_key ?? "root"}`}
                      to={discoverLibraryHref(match)}
                      className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/8 px-3 py-1.5 text-xs font-medium text-white/80 transition-colors hover:bg-white/12"
                    >
                      <span>{match.library_name}</span>
                      <span className="text-white/45">•</span>
                      <span className="uppercase tracking-[0.14em] text-white/55">
                        {match.kind}
                      </span>
                    </Link>
                  ))}
                </div>
              </div>
            ) : null}
          </div>
        </div>
      </section>

      <section className="grid gap-6 xl:grid-cols-[minmax(0,1fr)_320px]">
        <div className="rounded-(--radius-xl) border border-(--plum-border) bg-(--plum-panel) p-6">
          <h2 className="text-lg font-semibold text-(--plum-text)">Videos</h2>
          {videos.length === 0 ? (
            <p className="mt-3 text-sm text-(--plum-muted)">
              TMDB did not return any trailer or featurette links for this title.
            </p>
          ) : (
            <div className="mt-4 grid gap-3">
              {videos.map((video) => (
                <a
                  key={`${video.site}-${video.key}-${video.type}`}
                  href={video.href}
                  target="_blank"
                  rel="noreferrer"
                  className="flex items-center justify-between gap-4 rounded-lg border border-(--plum-border) bg-(--plum-panel-alt) px-4 py-3 text-sm text-(--plum-text) transition-colors hover:border-(--plum-accent-soft)"
                >
                  <div className="min-w-0">
                    <div className="truncate font-medium">{video.name}</div>
                    <div className="text-xs uppercase tracking-[0.14em] text-(--plum-muted)">
                      {video.site} • {video.type}
                    </div>
                  </div>
                  <ExternalLink className="size-4 shrink-0 text-(--plum-muted)" />
                </a>
              ))}
            </div>
          )}
        </div>

        <aside className="rounded-(--radius-xl) border border-(--plum-border) bg-(--plum-panel) p-6">
          <h2 className="text-lg font-semibold text-(--plum-text)">At a glance</h2>
          <div className="mt-4 flex flex-wrap gap-2">
            {details.genres.map((genre) => (
              <span
                key={genre}
                className="rounded-full border border-(--plum-border) bg-(--plum-panel-alt) px-3 py-1.5 text-xs font-medium text-(--plum-text)"
              >
                {genre}
              </span>
            ))}
          </div>
          <dl className="mt-6 space-y-4 text-sm">
            {details.release_date ? <DetailRow label="Release date" value={details.release_date} /> : null}
            {details.first_air_date ? (
              <DetailRow label="First air date" value={details.first_air_date} />
            ) : null}
            {details.status ? <DetailRow label="Status" value={details.status} /> : null}
            {details.number_of_seasons ? (
              <DetailRow label="Seasons" value={String(details.number_of_seasons)} />
            ) : null}
            {details.number_of_episodes ? (
              <DetailRow label="Episodes" value={String(details.number_of_episodes)} />
            ) : null}
            {details.runtime ? (
              <DetailRow
                label={details.media_type === "movie" ? "Runtime" : "Episode runtime"}
                value={`${details.runtime} min`}
              />
            ) : null}
          </dl>
        </aside>
      </section>
    </div>
  );
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-start justify-between gap-4">
      <dt className="text-(--plum-muted)">{label}</dt>
      <dd className="text-right font-medium text-(--plum-text)">{value}</dd>
    </div>
  );
}

function DiscoverDetailMessage({
  title,
  copy,
  actionLabel,
  onAction,
}: {
  title: string;
  copy: string;
  actionLabel?: string;
  onAction?: () => void;
}) {
  return (
    <div className="rounded-(--radius-xl) border border-dashed border-(--plum-border) bg-(--plum-panel)/45 p-8">
      <div className="max-w-xl space-y-2">
        <h1 className="text-lg font-semibold text-(--plum-text)">{title}</h1>
        <p className="text-sm leading-6 text-(--plum-muted)">{copy}</p>
        <div className="pt-2">
          {actionLabel && onAction ? (
            <button
              type="button"
              className="text-sm font-medium text-(--plum-accent) hover:underline"
              onClick={onAction}
            >
              {actionLabel}
            </button>
          ) : (
            <Button asChild variant="outline">
              <Link to="/discover">Back to Discover</Link>
            </Button>
          )}
        </div>
      </div>
    </div>
  );
}
