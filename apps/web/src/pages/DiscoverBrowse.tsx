import { useMemo } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import type { DiscoverBrowseCategory, DiscoverGenre, DiscoverMediaType } from "@/api";
import { Button } from "@/components/ui/button";
import { useAuthState } from "@/contexts/AuthContext";
import { DiscoverOriginProvider, useDiscoverOrigin } from "@/contexts/DiscoverOriginContext";
import {
  DISCOVER_CATEGORY_OPTIONS,
  DISCOVER_ORIGIN_PRESETS,
  discoverBrowseHref,
  discoverCategoryLabel,
  discoverMediaLabel,
} from "@/lib/discover";
import { useAddDiscoverTitle, useDiscoverBrowse, useDiscoverGenres } from "@/queries";
import { DiscoverMessage, mapDiscoverItemToPosterGridItem } from "./Discover";
import { LibraryPosterGrid } from "@/components/LibraryPosterGrid";

const VALID_CATEGORIES = new Set<DiscoverBrowseCategory>(
  DISCOVER_CATEGORY_OPTIONS.map((option) => option.id),
);

function parseCategory(value: string | null): DiscoverBrowseCategory | "" {
  if (value == null || value === "") {
    return "";
  }
  return VALID_CATEGORIES.has(value as DiscoverBrowseCategory) ? (value as DiscoverBrowseCategory) : "";
}

function parseMediaType(value: string | null): DiscoverMediaType | "" {
  return value === "movie" || value === "tv" ? value : "";
}

function parsePositiveInt(value: string | null): number | null {
  if (value == null || value === "") {
    return null;
  }
  const parsed = Number.parseInt(value, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

export function DiscoverBrowse() {
  return (
    <DiscoverOriginProvider>
      <DiscoverBrowseContent />
    </DiscoverOriginProvider>
  );
}

function DiscoverBrowseContent() {
  const { user } = useAuthState();
  const isAdmin = user?.is_admin ?? false;
  const navigate = useNavigate();
  const { originCountry } = useDiscoverOrigin();
  const [searchParams] = useSearchParams();
  const category = parseCategory(searchParams.get("category"));
  const mediaType = parseMediaType(searchParams.get("mediaType"));
  const genreId = parsePositiveInt(searchParams.get("genre"));
  const addTitle = useAddDiscoverTitle();
  const { data: genres } = useDiscoverGenres();
  const browse = useDiscoverBrowse({
    category,
    mediaType,
    genreId,
    originCountry: originCountry || undefined,
  });

  const items = useMemo(
    () => browse.data?.pages.flatMap((page) => page.items) ?? [],
    [browse.data],
  );
  const posterItems = useMemo(
    () =>
      items.map((item) =>
        mapDiscoverItemToPosterGridItem(item, isAdmin, addTitle, () => navigate("/settings")),
      ),
    [addTitle, isAdmin, items, navigate],
  );
  const firstPage = browse.data?.pages[0];
  const totalResults = firstPage?.total_results ?? 0;
  const activeMediaType = firstPage?.media_type ?? mediaType;
  const activeGenre = firstPage?.genre;
  const loadMoreItems = () => {
    if (!browse.hasNextPage || browse.isFetchingNextPage) {
      return;
    }
    void browse.fetchNextPage();
  };

  if (browse.error) {
    return (
      <DiscoverMessage
        title="Browse is unavailable right now"
        copy={browse.error.message}
        actionLabel="Retry"
        onAction={() => void browse.refetch()}
      />
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-8">
      <section className="rounded-(--radius-xl) border border-(--plum-border) bg-(--plum-panel) p-6 shadow-[0_20px_60px_rgba(8,12,24,0.1)]">
        <div className="flex flex-col gap-5">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div className="space-y-2">
              <div className="text-xs font-semibold uppercase tracking-[0.2em] text-(--plum-muted)">
                Discover Browse
              </div>
              <h1 className="text-3xl font-semibold tracking-tight text-(--plum-text)">
                {activeGenre?.name
                  ? `${activeGenre.name} ${activeMediaType === "tv" ? "TV" : "Movies"}`
                  : category
                    ? discoverCategoryLabel(category)
                    : "Browse Everything"}
              </h1>
              <p className="text-sm text-(--plum-muted)">
                {totalResults > 0
                  ? `${totalResults.toLocaleString()} titles available`
                  : "Open a category or genre to explore the full catalog."}
              </p>
              {originCountry ? (
                <p className="text-sm text-(--plum-muted)">
                  Production country:{" "}
                  <span className="font-medium text-(--plum-text)">
                    {DISCOVER_ORIGIN_PRESETS.find((p) => p.code === originCountry)?.label ??
                      originCountry}
                  </span>{" "}
                  (TMDB origin metadata).
                </p>
              ) : null}
            </div>

            <div className="flex flex-wrap gap-2">
              <Button asChild variant="outline" size="sm">
                <Link to={originCountry ? `/discover?origin=${originCountry}` : "/discover"}>
                  Back to Discover
                </Link>
              </Button>
              <Button asChild variant="ghost" size="sm">
                <Link to="/discover/browse">Clear filters</Link>
              </Button>
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            {DISCOVER_CATEGORY_OPTIONS.map((option) => {
              const selected = category === option.id;
              const browseMediaType = activeMediaType || option.defaultMediaType;
              return (
                <Link
                  key={option.id}
                  to={discoverBrowseHref({
                    category: option.id,
                    mediaType: browseMediaType,
                    originCountry,
                  })}
                  className={`rounded-full border px-3 py-1.5 text-sm transition-colors ${
                    selected
                      ? "border-(--plum-accent) bg-(--plum-accent) text-black"
                      : "border-(--plum-border) bg-(--plum-panel-alt) text-(--plum-text) hover:border-(--plum-accent-soft)"
                  }`}
                >
                  {option.label}
                </Link>
              );
            })}
          </div>

          <div className="flex flex-wrap gap-2">
            {(["movie", "tv"] as const).map((option) => {
              const selected = activeMediaType === option;
              return (
                <Link
                  key={option}
                  to={discoverBrowseHref({
                    category,
                    mediaType: option,
                    genreId: genreId ?? undefined,
                    originCountry,
                  })}
                  className={`rounded-full border px-3 py-1.5 text-sm transition-colors ${
                    selected
                      ? "border-(--plum-accent) bg-(--plum-accent) text-black"
                      : "border-(--plum-border) bg-(--plum-panel-alt) text-(--plum-text) hover:border-(--plum-accent-soft)"
                  }`}
                >
                  {discoverMediaLabel(option)}
                </Link>
              );
            })}
          </div>

          <div className="space-y-2">
            <div className="text-xs font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">
              Production country
            </div>
            <div className="flex flex-wrap gap-2">
              <Link
                to={discoverBrowseHref({
                  category,
                  mediaType: activeMediaType || undefined,
                  genreId: genreId ?? undefined,
                })}
                className={`rounded-full border px-3 py-1.5 text-sm transition-colors ${
                  originCountry === ""
                    ? "border-(--plum-accent) bg-(--plum-accent) text-black"
                    : "border-(--plum-border) bg-(--plum-panel-alt) text-(--plum-text) hover:border-(--plum-accent-soft)"
                }`}
              >
                Any
              </Link>
              {DISCOVER_ORIGIN_PRESETS.map((preset) => {
                const selected = originCountry === preset.code;
                return (
                  <Link
                    key={preset.code}
                    to={discoverBrowseHref({
                      category,
                      mediaType: activeMediaType || undefined,
                      genreId: genreId ?? undefined,
                      originCountry: preset.code,
                    })}
                    className={`rounded-full border px-3 py-1.5 text-sm transition-colors ${
                      selected
                        ? "border-(--plum-accent) bg-(--plum-accent) text-black"
                        : "border-(--plum-border) bg-(--plum-panel-alt) text-(--plum-text) hover:border-(--plum-accent-soft)"
                    }`}
                  >
                    {preset.label}
                  </Link>
                );
              })}
            </div>
          </div>

          <DiscoverBrowseGenreFilters
            movieGenres={genres?.movie_genres ?? []}
            tvGenres={genres?.tv_genres ?? []}
            category={category}
            activeMediaType={activeMediaType}
            activeGenreId={activeGenre?.id ?? genreId}
          />
        </div>
      </section>

      {browse.isLoading && items.length === 0 ? (
        <p className="text-sm text-(--plum-muted)">Loading browse results...</p>
      ) : items.length === 0 ? (
        <DiscoverMessage
          title="No titles found"
          copy="Try a different category, switch between movies and TV, or clear the active genre filter."
        />
      ) : (
        <>
          <LibraryPosterGrid
            items={posterItems}
            aspectRatio="poster"
            cardWidth={170}
            hasMore={browse.hasNextPage ?? false}
            onLoadMore={loadMoreItems}
          />
          {browse.isFetchingNextPage ? (
            <p className="text-center text-sm text-(--plum-muted)">Loading more titles...</p>
          ) : null}
        </>
      )}
    </div>
  );
}

function DiscoverBrowseGenreFilters({
  movieGenres,
  tvGenres,
  category,
  activeMediaType,
  activeGenreId,
}: {
  movieGenres: DiscoverGenre[];
  tvGenres: DiscoverGenre[];
  category: DiscoverBrowseCategory | "";
  activeMediaType: DiscoverMediaType | "";
  activeGenreId: number | null;
}) {
  if (activeMediaType === "movie") {
    return (
      <DiscoverBrowseGenreRow
        title="Movie Genres"
        genres={movieGenres}
        category={category}
        mediaType="movie"
        activeGenreId={activeGenreId}
      />
    );
  }
  if (activeMediaType === "tv") {
    return (
      <DiscoverBrowseGenreRow
        title="TV Genres"
        genres={tvGenres}
        category={category}
        mediaType="tv"
        activeGenreId={activeGenreId}
      />
    );
  }

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <DiscoverBrowseGenreRow
        title="Movie Genres"
        genres={movieGenres}
        category={category}
        mediaType="movie"
        activeGenreId={activeGenreId}
      />
      <DiscoverBrowseGenreRow
        title="TV Genres"
        genres={tvGenres}
        category={category}
        mediaType="tv"
        activeGenreId={activeGenreId}
      />
    </div>
  );
}

function DiscoverBrowseGenreRow({
  title,
  genres,
  category,
  mediaType,
  activeGenreId,
}: {
  title: string;
  genres: DiscoverGenre[];
  category: DiscoverBrowseCategory | "";
  mediaType: DiscoverMediaType;
  activeGenreId: number | null;
}) {
  const { originCountry } = useDiscoverOrigin();
  if (genres.length === 0) {
    return null;
  }
  return (
    <div className="space-y-2">
      <div className="text-xs font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">
        {title}
      </div>
      <div className="flex flex-wrap gap-2">
        <Link
          to={discoverBrowseHref({ category, mediaType, originCountry })}
          className={`rounded-full border px-3 py-1.5 text-sm transition-colors ${
            activeGenreId == null
              ? "border-(--plum-accent) bg-(--plum-accent) text-black"
              : "border-(--plum-border) bg-(--plum-panel-alt) text-(--plum-text)"
          }`}
        >
          All
        </Link>
        {genres.slice(0, 20).map((genre) => (
          <Link
            key={`${mediaType}-${genre.id}`}
            to={discoverBrowseHref({ category, mediaType, genreId: genre.id, originCountry })}
            className={`rounded-full border px-3 py-1.5 text-sm transition-colors ${
              activeGenreId === genre.id
                ? "border-(--plum-accent) bg-(--plum-accent) text-black"
                : "border-(--plum-border) bg-(--plum-panel-alt) text-(--plum-text) hover:border-(--plum-accent-soft)"
            }`}
          >
            {genre.name}
          </Link>
        ))}
      </div>
    </div>
  );
}
