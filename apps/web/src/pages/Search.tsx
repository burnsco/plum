import { LibraryPosterGrid } from "@/components/LibraryPosterGrid";
import type { PosterGridItem } from "@/components/types";
import { Input } from "@/components/ui/input";
import { EmptyState, InfoBadge, PageHeader, Surface } from "@/components/ui/page";
import { useLibrarySearch } from "@/queries";
import { tmdbPosterUrl } from "@plum/shared";
import { Search as SearchIcon } from "lucide-react";
import { useMemo } from "react";
import { useSearchParams } from "react-router-dom";

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
      <Surface>
        <div className="flex flex-col gap-4">
          <PageHeader
            className="border-b-0 pb-0"
            title="Library Search"
            description="Search local movie, TV, and anime libraries by title, actor, or genre."
            meta={resultLabel || undefined}
          />

          <div className="relative max-w-2xl">
            <SearchIcon className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-[var(--nebula-muted)]" />
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
      </Surface>

      {query.length < 2 ? (
        <SearchMessage
          title="Start with at least 2 characters"
          copy="Plum will search your indexed local libraries once the query is long enough."
        />
      ) : error ? (
        <SearchMessage title="Search is unavailable" copy={error.message} />
      ) : isLoading && !data ? (
        <p className="text-sm text-[var(--nebula-muted)]">Searching library…</p>
      ) : data && data.results.length === 0 ? (
        <SearchMessage
          title={`No results for "${query}"`}
          copy="Try a shorter title, a different actor name, or clear one of the active filters."
        />
      ) : (
        <LibraryPosterGrid
          items={
            data?.results.map((result) => {
              const poster =
                resolveSearchPoster(result.poster_url, result.poster_path) || undefined;
              const reason =
                result.match_reason === "actor"
                  ? `Actor match${result.matched_actor ? `: ${result.matched_actor}` : ""}`
                  : "Title match";
              const genres = result.genres?.slice(0, 3).join(", ");
              const converted: PosterGridItem = {
                key: result.href,
                title: result.title,
                subtitle: result.subtitle ?? result.library_name,
                posterUrl: poster,
                ratingValue: result.imdb_rating ?? undefined,
                ratingLabel: result.imdb_rating ? "IMDb" : undefined,
                metaLine: [result.subtitle ? result.library_name : null, reason, genres]
                  .filter(Boolean)
                  .join(" • "),
                href: result.href,
                topBadge: (
                  <span className="rounded-full bg-[var(--nebula-accent-soft)] px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-[var(--nebula-accent)]">
                    {result.kind}
                  </span>
                ),
              };
              return converted;
            }) ?? []
          }
        />
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
    <button type="button" onClick={onClick}>
      <InfoBadge active={active} className="transition-colors hover:text-[var(--nebula-text)]">
        {label}
      </InfoBadge>
    </button>
  );
}

function SearchMessage({ title, copy }: { title: string; copy: string }) {
  return <EmptyState title={title} copy={copy} />;
}
