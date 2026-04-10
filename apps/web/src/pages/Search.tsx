import { useMemo } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { Search as SearchIcon, User } from "lucide-react";
import { tmdbPosterUrl } from "@plum/shared";
import { MediaGridSkeleton } from "@/components/loading/PlumLoadingSkeletons";
import { Input } from "@/components/ui/input";
import { useLibrarySearch } from "@/queries";

function resolveSearchPoster(posterUrl?: string, posterPath?: string): string {
  if (posterUrl) {
    return posterUrl;
  }
  if (posterPath?.startsWith("http")) {
    return posterPath;
  }
  return tmdbPosterUrl(posterPath, "w500");
}

export function SearchPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const query = searchParams.get("q")?.trim() ?? "";
  const selectedType = (searchParams.get("type") as "movie" | "show" | null) ?? "";
  const selectedGenre = searchParams.get("genre") ?? "";
  const selectedLibraryId = searchParams.get("library_id");
  const libraryId = selectedLibraryId ? Number(selectedLibraryId) : null;
  const { data, error, isLoading } = useLibrarySearch(query, {
    enabled: query.length >= 2,
    libraryId,
    type: selectedType === "movie" || selectedType === "show" ? selectedType : "",
    genre: selectedGenre,
    limit: 36,
  });

  const resultLabel = useMemo(() => {
    if (!data) {
      return "";
    }
    return `${data.total} result${data.total === 1 ? "" : "s"}`;
  }, [data]);

  const updateParam = (key: string, value: string) => {
    const next = new URLSearchParams(searchParams);
    if (value) {
      next.set(key, value);
    } else {
      next.delete(key);
    }
    setSearchParams(next);
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-6">
      <section className="rounded-(--radius-xl) border border-(--plum-border) bg-(--plum-panel) p-5">
        <div className="flex flex-col gap-4">
          <div className="flex items-center justify-between gap-4">
            <div>
              <h1 className="text-2xl font-semibold text-(--plum-text)">Library Search</h1>
              <p className="text-sm text-(--plum-muted)">
                Search local movie, TV, and anime libraries by title, actor, or genre.
              </p>
            </div>
            {resultLabel ? (
              <span className="text-sm text-(--plum-muted)">{resultLabel}</span>
            ) : null}
          </div>

          <div className="relative max-w-2xl">
            <SearchIcon className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-(--plum-muted)" />
            <Input
              type="search"
              value={query}
              onChange={(event) => updateParam("q", event.target.value)}
              placeholder="Search your library"
              className="h-11 pl-9"
            />
          </div>

          {data ? (
            <div className="flex flex-wrap gap-2">
              <FilterChip
                active={selectedType === ""}
                label="All Types"
                onClick={() => updateParam("type", "")}
              />
              {data.facets.types.map((facet) => (
                <FilterChip
                  key={facet.value}
                  active={selectedType === facet.value}
                  label={`${facet.label} (${facet.count})`}
                  onClick={() => updateParam("type", facet.value)}
                />
              ))}
            </div>
          ) : null}

          {data?.facets.genres.length ? (
            <div className="flex flex-wrap gap-2">
              <FilterChip
                active={selectedGenre === ""}
                label="All Genres"
                onClick={() => updateParam("genre", "")}
              />
              {data.facets.genres.slice(0, 12).map((facet) => (
                <FilterChip
                  key={facet.value}
                  active={selectedGenre === facet.value}
                  label={`${facet.label} (${facet.count})`}
                  onClick={() => updateParam("genre", facet.value)}
                />
              ))}
            </div>
          ) : null}
        </div>
      </section>

      {query.length < 2 ? (
        <SearchMessage
          title="Start with at least 2 characters"
          copy="Plum will search your indexed local libraries once the query is long enough."
        />
      ) : error ? (
        <SearchMessage title="Search is unavailable" copy={error.message} />
      ) : isLoading && !data ? (
        <MediaGridSkeleton count={8} />
      ) : data && data.results.length === 0 ? (
        <SearchMessage
          title={`No results for "${query}"`}
          copy="Try a shorter title, a different actor name, or clear one of the active filters."
        />
      ) : (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-4">
          {data?.results.map((result) => (
            <Link
              key={result.href}
              to={result.href}
              className="group flex overflow-hidden rounded-(--radius-xl) border border-(--plum-border) bg-(--plum-panel) transition-transform duration-200 hover:-translate-y-1 hover:border-(--plum-accent-soft)"
            >
              <div className="w-24 shrink-0 bg-(--plum-panel-alt)">
                <img
                  src={resolveSearchPoster(result.poster_url, result.poster_path) || "/placeholder-poster.svg"}
                  alt=""
                  className="h-full w-full object-contain"
                />
              </div>
              <div className="flex min-w-0 flex-1 flex-col gap-2 p-3">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-semibold text-(--plum-text)">
                      {result.title}
                    </div>
                    {result.subtitle ? (
                      <div className="text-xs text-(--plum-muted)">{result.subtitle}</div>
                    ) : null}
                  </div>
                  <span className="rounded-full bg-(--plum-accent-soft) px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-(--plum-accent)">
                    {result.kind}
                  </span>
                </div>

                <div className="text-xs text-(--plum-muted)">
                  {result.library_name}
                  {result.imdb_rating ? ` • IMDb ${result.imdb_rating.toFixed(1)}` : ""}
                </div>

                {result.match_reason === "actor" ? (
                  <div className="inline-flex items-center gap-1 text-xs text-(--plum-muted)">
                    <User className="size-3.5" />
                    <span>Actor match: {result.matched_actor}</span>
                  </div>
                ) : (
                  <div className="text-xs text-(--plum-muted)">Title match</div>
                )}

                {result.genres?.length ? (
                  <div className="flex flex-wrap gap-1.5">
                    {result.genres.slice(0, 3).map((genre) => (
                      <span
                        key={genre}
                        className="rounded-full border border-(--plum-border) px-2 py-1 text-[10px] uppercase tracking-[0.12em] text-(--plum-muted)"
                      >
                        {genre}
                      </span>
                    ))}
                  </div>
                ) : null}
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

function FilterChip({
  active,
  label,
  onClick,
}: {
  active: boolean;
  label: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`rounded-full border px-3 py-1.5 text-xs font-semibold uppercase tracking-[0.12em] transition-colors ${
        active
          ? "border-(--plum-accent) bg-(--plum-accent-soft) text-(--plum-accent)"
          : "border-(--plum-border) text-(--plum-muted) hover:border-(--plum-accent-soft) hover:text-(--plum-text)"
      }`}
    >
      {label}
    </button>
  );
}

function SearchMessage({ title, copy }: { title: string; copy: string }) {
  return (
    <div className="rounded-(--radius-xl) border border-dashed border-(--plum-border) bg-(--plum-panel)/45 p-8">
      <div className="max-w-xl space-y-2">
        <h2 className="text-lg font-semibold text-(--plum-text)">{title}</h2>
        <p className="text-sm leading-6 text-(--plum-muted)">{copy}</p>
      </div>
    </div>
  );
}
