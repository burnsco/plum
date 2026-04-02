import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Sparkles } from "lucide-react";
import { tmdbPosterUrl } from "@plum/shared";
import type { DiscoverBrowseCategory, DiscoverGenre, DiscoverItem, DiscoverResponse } from "@/api";
import { LibraryPosterGrid } from "@/components/LibraryPosterGrid";
import MediaCard from "@/components/MediaCard";
import type { PosterGridItem } from "@/components/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { HorizontalScrollRail } from "@/components/ui/page";
import { useAuthState } from "@/contexts/AuthContext";
import {
  DISCOVER_CATEGORY_OPTIONS,
  type DiscoverMediaFilter,
  discoverAcquisitionLabel,
  discoverAcquisitionTone,
  discoverBrowseHref,
  discoverGenresForFilter,
  discoverMediaFilterLabel,
  discoverMediaLabel,
  discoverVisibleItems,
  discoverYear,
} from "@/lib/discover";
import { useAddDiscoverTitle, useDiscover, useDiscoverGenres, useDiscoverSearch } from "@/queries";

function isDiscoverConfigError(error: Error | null): boolean {
  return error?.message.includes("TMDB_API_KEY") ?? false
}

export function Discover() {
  const { user } = useAuthState();
  const isAdmin = user?.is_admin ?? false;
  const navigate = useNavigate();
  const [mediaFilter, setMediaFilter] = useState<DiscoverMediaFilter>("all");
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const addTitle = useAddDiscoverTitle();
  const {
    data: discover,
    error: discoverError,
    isLoading: discoverLoading,
    refetch: refetchDiscover,
  } = useDiscover({ refetchInterval: 15_000 });
  const {
    data: genres,
    error: genresError,
    refetch: refetchGenres,
  } = useDiscoverGenres({ refetchInterval: 15_000 });

  useEffect(() => {
    const trimmed = searchInput.trim();
    if (trimmed.length < 2) {
      setSearchQuery((current) => (current === "" ? current : ""));
      return;
    }
    const timeoutId = window.setTimeout(() => {
      setSearchQuery((current) => (current === trimmed ? current : trimmed));
    }, 300);
    return () => window.clearTimeout(timeoutId);
  }, [searchInput]);

  const searchActive = searchQuery.length >= 2;
  const {
    data: searchResults,
    error: searchError,
    isLoading: searchLoading,
    refetch: refetchSearch,
  } = useDiscoverSearch(searchQuery, { enabled: searchActive, refetchInterval: 15_000 });
  const activeError = searchActive ? searchError : (discoverError ?? genresError);
  const isConfigError = isDiscoverConfigError(activeError);

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-8">
      <section className="rounded-(--radius-xl) border border-(--plum-border) bg-[radial-gradient(circle_at_top_left,rgba(244,90,160,0.22),transparent_45%),linear-gradient(135deg,rgba(15,23,42,0.96),rgba(18,24,38,0.88))] p-6 text-white shadow-[0_20px_70px_rgba(9,12,20,0.28)]">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div className="max-w-2xl space-y-4">
            <div className="inline-flex items-center gap-2 rounded-full border border-white/15 bg-white/10 px-3 py-1 text-xs font-semibold uppercase tracking-[0.24em] text-white/75">
              <Sparkles className="size-3.5" />
              Discover
            </div>
            <div className="space-y-2">
              <h1 className="text-3xl font-semibold tracking-tight">Find something worth adding.</h1>
              <p className="max-w-xl text-sm leading-6 text-white/75">
                Browse wide TMDB-powered shelves, open full category pages, and filter by movies,
                TV, and genres without leaving Plum.
              </p>
            </div>
            <DiscoverMediaToggle value={mediaFilter} onChange={setMediaFilter} />
          </div>

          <div className="w-full max-w-xl">
            <Input
              type="search"
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              placeholder="Search movies and TV shows"
              className="h-11 border-white/10 bg-black/20 text-white placeholder:text-white/45"
            />
            <p className="mt-2 text-xs text-white/55">
              Search kicks in after 2 characters with a 300ms delay.
            </p>
          </div>
        </div>
      </section>

      {!searchActive ? (
        <DiscoverGenreSection
          movieGenres={genres?.movie_genres ?? []}
          tvGenres={genres?.tv_genres ?? []}
          mediaFilter={mediaFilter}
        />
      ) : null}

      {isConfigError ? (
        <DiscoverMessage
          title="Discover needs TMDB configured"
          copy="Set `TMDB_API_KEY` on the server to enable external shelves, browse pages, genres, search, and title details."
          actionLabel="Retry"
          onAction={() => {
            if (searchActive) {
              void refetchSearch();
              return;
            }
            void refetchDiscover();
            void refetchGenres();
          }}
        />
      ) : activeError ? (
        <DiscoverMessage
          title="Discover is unavailable right now"
          copy={activeError.message}
          actionLabel="Retry"
          onAction={() => {
            if (searchActive) {
              void refetchSearch();
              return;
            }
            void refetchDiscover();
            void refetchGenres();
          }}
        />
      ) : searchActive ? (
        <DiscoverSearchResults
          query={searchQuery}
          loading={searchLoading}
          results={searchResults}
          mediaFilter={mediaFilter}
          isAdmin={isAdmin}
          addTitle={addTitle}
          onOpenSettings={() => navigate("/settings")}
        />
      ) : discoverLoading ? (
        <p className="text-sm text-(--plum-muted)">Loading discover shelves...</p>
      ) : (
        <DiscoverShelves
          discover={discover}
          mediaFilter={mediaFilter}
          isAdmin={isAdmin}
          addTitle={addTitle}
          onOpenSettings={() => navigate("/settings")}
        />
      )}
    </div>
  );
}

function DiscoverMediaToggle({
  value,
  onChange,
}: {
  value: DiscoverMediaFilter;
  onChange: (value: DiscoverMediaFilter) => void;
}) {
  return (
    <div className="inline-flex w-fit rounded-full border border-white/10 bg-black/20 p-1">
      {(["all", "movie", "tv"] as const).map((option) => {
        const selected = value === option;
        return (
          <button
            key={option}
            type="button"
            onClick={() => onChange(option)}
            className={`rounded-full px-4 py-2 text-sm font-medium transition ${
              selected ? "bg-white text-slate-950" : "text-white/75 hover:text-white"
            }`}
          >
            {discoverMediaFilterLabel(option)}
          </button>
        );
      })}
    </div>
  );
}

function DiscoverGenreSection({
  movieGenres,
  tvGenres,
  mediaFilter,
}: {
  movieGenres: DiscoverGenre[];
  tvGenres: DiscoverGenre[];
  mediaFilter: DiscoverMediaFilter;
}) {
  if (mediaFilter === "all") {
    if (movieGenres.length === 0 && tvGenres.length === 0) {
      return null;
    }
    return (
      <section className="rounded-(--radius-xl) border border-(--plum-border) bg-(--plum-panel) p-5">
        <div className="mb-4 flex items-center justify-between gap-4">
          <div>
            <h2 className="text-lg font-semibold text-(--plum-text)">Browse by Genre</h2>
            <p className="text-sm text-(--plum-muted)">
              Jump straight into a bigger movie or TV catalog.
            </p>
          </div>
        </div>
        <div className="space-y-4">
          <DiscoverGenreRow title="Movie Genres" genres={movieGenres} mediaType="movie" />
          <DiscoverGenreRow title="TV Genres" genres={tvGenres} mediaType="tv" />
        </div>
      </section>
    );
  }

  const genres = discoverGenresForFilter(movieGenres, tvGenres, mediaFilter);
  if (genres.length === 0) {
    return null;
  }
  return (
    <section className="rounded-(--radius-xl) border border-(--plum-border) bg-(--plum-panel) p-5">
      <div className="mb-4 flex items-center justify-between gap-4">
        <div>
          <h2 className="text-lg font-semibold text-(--plum-text)">Browse by Genre</h2>
          <p className="text-sm text-(--plum-muted)">
            Explore {mediaFilter === "movie" ? "movies" : "TV"} by genre.
          </p>
        </div>
      </div>
      <DiscoverGenreChips genres={genres} mediaType={mediaFilter} />
    </section>
  );
}

function DiscoverGenreRow({
  title,
  genres,
  mediaType,
}: {
  title: string;
  genres: DiscoverGenre[];
  mediaType: "movie" | "tv";
}) {
  if (genres.length === 0) {
    return null;
  }
  return (
    <div className="space-y-2">
      <div className="text-xs font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">
        {title}
      </div>
      <DiscoverGenreChips genres={genres} mediaType={mediaType} />
    </div>
  );
}

function DiscoverGenreChips({
  genres,
  mediaType,
}: {
  genres: DiscoverGenre[];
  mediaType: "movie" | "tv";
}) {
  return (
    <div className="flex flex-wrap gap-2">
      {genres.slice(0, 18).map((genre) => (
        <Link
          key={`${mediaType}-${genre.id}`}
          to={discoverBrowseHref({ mediaType, genreId: genre.id })}
          className="rounded-full border border-(--plum-border) bg-(--plum-panel-alt) px-3 py-1.5 text-sm text-(--plum-text) transition-colors hover:border-(--plum-accent-soft) hover:text-(--plum-accent)"
        >
          {genre.name}
        </Link>
      ))}
    </div>
  );
}

function DiscoverShelves({
  discover,
  mediaFilter,
  isAdmin,
  addTitle,
  onOpenSettings,
}: {
  discover: DiscoverResponse | undefined;
  mediaFilter: DiscoverMediaFilter;
  isAdmin: boolean;
  addTitle: ReturnType<typeof useAddDiscoverTitle>;
  onOpenSettings: () => void;
}) {
  const shelves = useMemo(
    () =>
      (discover?.shelves ?? [])
        .map((shelf) => ({
          ...shelf,
          items: discoverVisibleItems(shelf.items, mediaFilter),
        }))
        .filter((shelf) => shelf.items.length > 0),
    [discover, mediaFilter],
  );

  if (!shelves.length) {
    return (
      <DiscoverMessage
        title="Nothing to surface yet"
        copy="Plum could not load any discover shelves for the current filter."
      />
    );
  }

  return (
    <div className="flex flex-col gap-8">
      {shelves.map((shelf) => {
        const categoryOption = DISCOVER_CATEGORY_OPTIONS.find((option) => option.id === shelf.id);
        const browseMediaType =
          mediaFilter === "all" ? (categoryOption?.defaultMediaType ?? "") : mediaFilter;
        return (
          <section key={shelf.id} className="flex flex-col gap-4">
            <div className="flex items-center justify-between gap-4">
              <div>
                <h2 className="text-lg font-semibold text-(--plum-text)">{shelf.title}</h2>
                <p className="text-sm text-(--plum-muted)">
                  {shelf.items.length} visible title{shelf.items.length === 1 ? "" : "s"}
                </p>
              </div>
              <Button asChild variant="outline" size="sm">
                <Link
                  to={discoverBrowseHref({
                    category: shelf.id as DiscoverBrowseCategory,
                    mediaType: browseMediaType,
                  })}
                >
                  View all
                </Link>
              </Button>
            </div>
            <DiscoverCardRail
              title={shelf.title}
              items={shelf.items}
              isAdmin={isAdmin}
              addTitle={addTitle}
              onOpenSettings={onOpenSettings}
            />
          </section>
        );
      })}
    </div>
  );
}

function DiscoverSearchResults({
  query,
  loading,
  results,
  mediaFilter,
  isAdmin,
  addTitle,
  onOpenSettings,
}: {
  query: string;
  loading: boolean;
  results: { movies: DiscoverItem[]; tv: DiscoverItem[] } | undefined;
  mediaFilter: DiscoverMediaFilter;
  isAdmin: boolean;
  addTitle: ReturnType<typeof useAddDiscoverTitle>;
  onOpenSettings: () => void;
}) {
  if (loading && !results) {
    return <p className="text-sm text-(--plum-muted)">Searching TMDB...</p>;
  }

  const movies = mediaFilter === "tv" ? [] : results?.movies ?? [];
  const tv = mediaFilter === "movie" ? [] : results?.tv ?? [];

  if (movies.length === 0 && tv.length === 0) {
    return (
      <DiscoverMessage
        title={`No results for "${query}"`}
        copy="Try another title, a shorter query, or browse one of the shelves instead."
      />
    );
  }

  return (
    <div className="flex flex-col gap-8">
      {movies.length > 0 ? (
        <section className="flex flex-col gap-4">
          <div className="flex items-center justify-between gap-4">
            <h2 className="text-lg font-semibold text-(--plum-text)">Movies</h2>
            <span className="text-sm text-(--plum-muted)">{movies.length} matches</span>
          </div>
          <DiscoverGrid
            items={movies}
            emptyLabel="No movie matches."
            isAdmin={isAdmin}
            addTitle={addTitle}
            onOpenSettings={onOpenSettings}
          />
        </section>
      ) : null}

      {tv.length > 0 ? (
        <section className="flex flex-col gap-4">
          <div className="flex items-center justify-between gap-4">
            <h2 className="text-lg font-semibold text-(--plum-text)">TV Shows</h2>
            <span className="text-sm text-(--plum-muted)">{tv.length} matches</span>
          </div>
          <DiscoverGrid
            items={tv}
            emptyLabel="No TV matches."
            isAdmin={isAdmin}
            addTitle={addTitle}
            onOpenSettings={onOpenSettings}
          />
        </section>
      ) : null}
    </div>
  );
}

function DiscoverGrid({
  items,
  emptyLabel,
  isAdmin,
  addTitle,
  onOpenSettings,
}: {
  items: DiscoverItem[];
  emptyLabel: string;
  isAdmin: boolean;
  addTitle: ReturnType<typeof useAddDiscoverTitle>;
  onOpenSettings: () => void;
}) {
  if (items.length === 0) {
    return (
      <div className="rounded-(--radius-xl) border border-dashed border-(--plum-border) bg-(--plum-panel)/45 p-6 text-sm text-(--plum-muted)">
        {emptyLabel}
      </div>
    );
  }

  return (
    <LibraryPosterGrid
      items={items.map((item) =>
        mapDiscoverItemToPosterGridItem(item, isAdmin, addTitle, onOpenSettings),
      )}
      aspectRatio="poster"
      cardWidth={170}
    />
  );
}

function DiscoverCardRail({
  title,
  items,
  isAdmin,
  addTitle,
  onOpenSettings,
}: {
  title: string;
  items: DiscoverItem[];
  isAdmin: boolean;
  addTitle: ReturnType<typeof useAddDiscoverTitle>;
  onOpenSettings: () => void;
}) {
  const posterItems = useMemo(
    () =>
      items.map((item) =>
        mapDiscoverItemToPosterGridItem(item, isAdmin, addTitle, onOpenSettings),
      ),
    [addTitle, isAdmin, items, onOpenSettings],
  );

  return (
    <HorizontalScrollRail
      label={title}
      contentClassName="gap-4 overflow-x-auto pb-2 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
    >
      {posterItems.map((item, index) => (
        <div key={item.key} className="w-44 shrink-0">
          <MediaCard item={item} index={index} />
        </div>
      ))}
    </HorizontalScrollRail>
  );
}

export function mapDiscoverItemToPosterGridItem(
  item: DiscoverItem,
  isAdmin: boolean,
  addTitle: ReturnType<typeof useAddDiscoverTitle>,
  onOpenSettings: () => void,
): PosterGridItem {
  const year = discoverYear(item);
  const posterUrl = tmdbPosterUrl(item.poster_path, "w500");
  const addPending =
    addTitle.isPending &&
    addTitle.variables?.mediaType === item.media_type &&
    addTitle.variables?.tmdbId === item.tmdb_id;
  const acquisition = item.acquisition;
  const needsSetup =
    acquisition?.is_configured === false && acquisition?.state === "not_added" && isAdmin;
  const actionLabel = needsSetup ? "Set Up" : discoverAcquisitionLabel(acquisition, addPending);
  const actionTone = needsSetup ? "default" : discoverAcquisitionTone(acquisition, addPending);
  const actionDisabled = addPending || (!needsSetup && acquisition?.can_add !== true);
  const onAction = needsSetup
    ? onOpenSettings
    : acquisition?.can_add
      ? () => addTitle.mutate({ mediaType: item.media_type, tmdbId: item.tmdb_id })
      : undefined;

  return {
    key: `${item.media_type}-${item.tmdb_id}`,
    title: item.title,
    subtitle: year || "Upcoming",
    metaLine: item.overview ? item.overview : undefined,
    posterUrl,
    ratingLabel: "TMDB",
    ratingValue: item.vote_average,
    href: `/discover/${item.media_type}/${item.tmdb_id}`,
    topBadge: (
      <span className="rounded-full bg-black/60 px-2.5 py-1 text-[11px] font-medium uppercase tracking-[0.18em] text-white/75 backdrop-blur-sm">
        {discoverMediaLabel(item.media_type)}
      </span>
    ),
    actionLabel,
    actionDisabled,
    actionTone,
    onAction,
  } satisfies PosterGridItem;
}

export function DiscoverMessage({
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
        <h2 className="text-lg font-semibold text-(--plum-text)">{title}</h2>
        <p className="text-sm leading-6 text-(--plum-muted)">{copy}</p>
        {actionLabel && onAction ? (
          <button
            type="button"
            className="mt-2 text-sm font-medium text-(--plum-accent) hover:underline"
            onClick={onAction}
          >
            {actionLabel}
          </button>
        ) : null}
      </div>
    </div>
  );
}
