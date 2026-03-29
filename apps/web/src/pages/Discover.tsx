import type { DiscoverItem, DiscoverResponse } from "@/api";
import MediaCard from "@/components/MediaCard";
import type { PosterGridItem } from "@/components/types";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { EmptyState, HorizontalScrollRail, InfoBadge, PageHeader, Surface } from "@/components/ui/page";
import { discoverMediaLabel, discoverYear } from "@/lib/discover";
import { useDiscover, useDiscoverSearch } from "@/queries";
import { tmdbPosterUrl } from "@plum/shared";
import { Sparkles } from "lucide-react";
import { useEffect, useState } from "react";

function isDiscoverConfigError(error: Error | null): boolean {
  return error?.message.includes("TMDB_API_KEY") ?? false;
}

export function Discover() {
  const [searchInput, setSearchInput] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const {
    data: discover,
    error: discoverError,
    isLoading: discoverLoading,
    refetch: refetchDiscover,
  } = useDiscover();

  useEffect(() => {
    const trimmed = searchInput.trim();
    if (trimmed.length < 2) {
      setSearchQuery("");
      return;
    }
    const timeoutId = window.setTimeout(() => {
      setSearchQuery(trimmed);
    }, 300);
    return () => window.clearTimeout(timeoutId);
  }, [searchInput]);

  const searchActive = searchQuery.length >= 2;
  const {
    data: searchResults,
    error: searchError,
    isLoading: searchLoading,
    refetch: refetchSearch,
  } = useDiscoverSearch(searchQuery, { enabled: searchActive });
  const activeError = searchActive ? searchError : discoverError;
  const isConfigError = isDiscoverConfigError(activeError);

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-8">
      <Surface className="bg-[linear-gradient(180deg,rgba(12,21,36,0.96),rgba(12,21,36,0.92))]">
        <div className="flex flex-col gap-5">
          <InfoBadge className="w-fit">
            <Sparkles className="size-3.5" />
            Discover
          </InfoBadge>
          <PageHeader
            className="border-b-0 pb-0"
            title="Find something worth adding"
            description="Browse TMDB-powered shelves for movies and TV, then jump into title details with Plum-aware library status."
          />

          <div className="w-full max-w-xl">
            <Input
              type="search"
              value={searchInput}
              onChange={(event) => setSearchInput(event.target.value)}
              placeholder="Search movies and TV shows"
              className="h-11"
            />
            <p className="mt-2 text-xs text-[var(--plum-muted)]">
              Search kicks in after 2 characters with a 300ms delay.
            </p>
          </div>
        </div>
      </Surface>

      {isConfigError ? (
        <DiscoverMessage
          title="Discover needs TMDB configured"
          copy="Set `TMDB_API_KEY` on the server to enable external shelves, search, and title details."
          actionLabel="Retry"
          onAction={() => {
            if (searchActive) {
              void refetchSearch();
              return;
            }
            void refetchDiscover();
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
          }}
        />
      ) : searchActive ? (
        <DiscoverSearchResults
          query={searchQuery}
          loading={searchLoading}
          results={searchResults}
        />
      ) : discoverLoading ? (
        <p className="text-sm text-[var(--plum-muted)]">Loading discover shelves...</p>
      ) : (
        <DiscoverShelves discover={discover} />
      )}
    </div>
  );
}

function DiscoverShelves({ discover }: { discover: DiscoverResponse | undefined }) {
  if (!discover?.shelves.length) {
    return (
      <DiscoverMessage
        title="Nothing to surface yet"
        copy="Plum could not load any discover shelves from TMDB."
      />
    );
  }

  return (
    <div className="flex flex-col gap-8">
      {discover.shelves.map((shelf) => (
        <section key={shelf.id} className="flex flex-col gap-4">
          <div className="flex items-center justify-between gap-4">
            <h2 className="text-lg font-semibold text-[var(--plum-text)]">{shelf.title}</h2>
            <span className="text-sm text-[var(--plum-muted)]">
              {shelf.items.length} title{shelf.items.length === 1 ? "" : "s"}
            </span>
          </div>
          <DiscoverCardRail items={shelf.items} />
        </section>
      ))}
    </div>
  );
}

function DiscoverSearchResults({
  query,
  loading,
  results,
}: {
  query: string;
  loading: boolean;
  results: { movies: DiscoverItem[]; tv: DiscoverItem[] } | undefined;
}) {
  if (loading && !results) {
    return <p className="text-sm text-[var(--plum-muted)]">Searching TMDB...</p>;
  }

  const movies = results?.movies ?? [];
  const tv = results?.tv ?? [];

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
      <section className="flex flex-col gap-4">
        <div className="flex items-center justify-between gap-4">
          <h2 className="text-lg font-semibold text-[var(--plum-text)]">Movies</h2>
          <span className="text-sm text-[var(--plum-muted)]">{movies.length} matches</span>
        </div>
        <DiscoverGrid items={movies} emptyLabel="No movie matches." />
      </section>

      <section className="flex flex-col gap-4">
        <div className="flex items-center justify-between gap-4">
          <h2 className="text-lg font-semibold text-[var(--plum-text)]">TV Shows</h2>
          <span className="text-sm text-[var(--plum-muted)]">{tv.length} matches</span>
        </div>
        <DiscoverGrid items={tv} emptyLabel="No TV matches." />
      </section>
    </div>
  );
}

function DiscoverGrid({ items, emptyLabel }: { items: DiscoverItem[]; emptyLabel: string }) {
  if (items.length === 0) {
    return <EmptyState title="No matches here" copy={emptyLabel} className="p-6" />;
  }

  return (
    <div className="grid grid-cols-[repeat(auto-fill,minmax(170px,1fr))] gap-4">
      {items.map((item) => (
        <DiscoverCard key={`${item.media_type}-${item.tmdb_id}`} item={item} />
      ))}
    </div>
  );
}

function DiscoverCardRail({ items }: { items: DiscoverItem[] }) {
  return (
    <HorizontalScrollRail
      label="discover shelf"
      className="w-full"
      contentClassName="flex gap-4 overflow-x-auto px-12 pb-2"
    >
      {items.map((item) => (
        <DiscoverCard key={`${item.media_type}-${item.tmdb_id}`} item={item} rail />
      ))}
    </HorizontalScrollRail>
  );
}

function DiscoverCard({ item, rail = false }: { item: DiscoverItem; rail?: boolean }) {
  const posterUrl = tmdbPosterUrl(item.poster_path, "w500");
  const year = discoverYear(item);
  const inLibrary = (item.library_matches?.length ?? 0) > 0;

  const converted: PosterGridItem = {
    key: `${item.media_type}-${item.tmdb_id}`,
    title: item.title,
    subtitle: year || "Upcoming",
    posterUrl,
    ratingValue: item.vote_average ?? undefined,
    ratingLabel: "Rating",
    href: `/discover/${item.media_type}/${item.tmdb_id}`,
    topBadge: (
      <>
        <InfoBadge className="bg-black/40 text-white/82 backdrop-blur-sm">
          {discoverMediaLabel(item.media_type)}
        </InfoBadge>
        {inLibrary ? (
          <InfoBadge className="border-[rgba(255,255,255,0.18)] bg-[rgba(4,10,20,0.82)] text-white shadow-[0_12px_28px_rgba(0,0,0,0.28)]">
            In Library
          </InfoBadge>
        ) : null}
      </>
    ),
  };

  const wrapperClass = `flex ${rail ? "w-44 shrink-0" : "w-full"}`;
  const cardClass = `flex-col overflow-hidden rounded-[var(--radius-xl)] border border-[var(--plum-border)] bg-[var(--plum-panel)]/96 transition-colors duration-200`;

  return (
    <div className={`group ${wrapperClass}`}>
      <MediaCard item={converted} className={cardClass} />
    </div>
  );
}

function DiscoverMessage({
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
    <EmptyState
      title={title}
      copy={copy}
      action={
        actionLabel && onAction ? (
          <Button type="button" variant="outline" onClick={onAction}>
            {actionLabel}
          </Button>
        ) : undefined
      }
    />
  );
}
