import { useRef, useMemo, type ReactNode } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import type { MediaItem } from "../api";
import {
  buildMusicLibraryGroups,
  formatRuntime,
} from "../lib/musicGrouping";
import { useLoadMoreTrigger, useVirtualContainerMetrics } from "../lib/virtualization";
import { LibraryPosterGrid } from "./LibraryPosterGrid";

interface Props {
  items: MediaItem[];
  onPlayCollection: (items: MediaItem[], startItem?: MediaItem) => void;
  hasMore?: boolean;
  onLoadMore?: () => void;
}

const TRACK_ROW_ESTIMATE = 74;
const TRACK_VIRTUALIZATION_THRESHOLD = 150;

export function MusicLibraryView({
  items,
  onPlayCollection,
  hasMore = false,
  onLoadMore,
}: Props) {
  const { tracks, albums, artists } = useMemo(() => buildMusicLibraryGroups(items), [items]);

  return (
    <div className="music-library">
      <MusicSection
        title="Albums"
        count={`${albums.length} album${albums.length === 1 ? "" : "s"}`}
      >
        <LibraryPosterGrid
          compact
          hasMore={hasMore}
          onLoadMore={onLoadMore}
          items={albums.map((album) => ({
            key: album.key,
            title: album.title,
            subtitle: `${album.artist} • ${album.trackCount} tracks${album.year ? ` • ${album.year}` : ""}`,
            posterPath: album.posterPath,
            posterUrl: album.posterUrl,
            onClick: () => onPlayCollection(album.tracks, album.tracks[0]),
            onPlay: () => onPlayCollection(album.tracks, album.tracks[0]),
          }))}
        />
      </MusicSection>

      <MusicSection
        title="Artists"
        count={`${artists.length} artist${artists.length === 1 ? "" : "s"}`}
      >
        <LibraryPosterGrid
          compact
          hasMore={hasMore}
          onLoadMore={onLoadMore}
          items={artists.map((artist) => ({
            key: artist.key,
            title: artist.name,
            subtitle: `${artist.albumCount} albums • ${artist.trackCount} tracks`,
            posterPath: artist.posterPath,
            posterUrl: artist.posterUrl,
            onClick: () => onPlayCollection(artist.tracks, artist.tracks[0]),
            onPlay: () => onPlayCollection(artist.tracks, artist.tracks[0]),
          }))}
        />
      </MusicSection>

      <MusicSection
        title="Tracks"
        count={`${tracks.length} track${tracks.length === 1 ? "" : "s"}`}
      >
        <VirtualTrackList
          items={tracks}
          onPlayCollection={onPlayCollection}
          hasMore={hasMore}
          onLoadMore={onLoadMore}
        />
      </MusicSection>

      <div className="music-section-grid">
        <MusicPlaceholderSection
          title="Genres"
          description="Genres will appear here once Plum stores music genre metadata."
        />
        <MusicPlaceholderSection
          title="Playlists"
          description="Playlists are not persisted yet. This section is ready for future queue saving."
        />
      </div>
    </div>
  );
}

function VirtualTrackList({
  items,
  onPlayCollection,
  hasMore = false,
  onLoadMore,
}: {
  items: MediaItem[];
  onPlayCollection: (items: MediaItem[], startItem?: MediaItem) => void;
  hasMore?: boolean;
  onLoadMore?: () => void;
}) {
  const rootRef = useRef<HTMLDivElement>(null);
  const { scrollElement, scrollMargin } = useVirtualContainerMetrics(rootRef);
  const loadMoreRef = useLoadMoreTrigger({
    root: scrollElement,
    enabled: hasMore ?? false,
    onLoadMore,
  });
  const shouldVirtualize =
    items.length >= TRACK_VIRTUALIZATION_THRESHOLD &&
    scrollElement != null &&
    typeof ResizeObserver !== "undefined";
  const rowVirtualizer = useVirtualizer({
    count: shouldVirtualize ? items.length : 0,
    getScrollElement: () => scrollElement,
    estimateSize: () => TRACK_ROW_ESTIMATE,
    overscan: 6,
    scrollMargin,
  });

  if (!shouldVirtualize) {
    return (
      <div ref={rootRef} className="music-track-list" role="list">
        {items.map((track) => (
          <TrackRow
            key={track.id}
            track={track}
            items={items}
            onPlayCollection={onPlayCollection}
          />
        ))}
        {hasMore ? <div ref={loadMoreRef} className="h-px w-full" aria-hidden="true" /> : null}
      </div>
    );
  }

  return (
    <div ref={rootRef} className="music-track-list music-track-list--virtual" role="list">
      <div style={{ height: `${rowVirtualizer.getTotalSize()}px`, position: "relative" }}>
        {rowVirtualizer.getVirtualItems().map((virtualRow) => {
          const track = items[virtualRow.index];
          return (
            <div
              key={track.id}
              className="music-track-list__row"
              style={{
                transform: `translateY(${virtualRow.start - scrollMargin}px)`,
              }}
            >
              <TrackRow track={track} items={items} onPlayCollection={onPlayCollection} />
            </div>
          );
        })}
        {hasMore ? (
          <div
            ref={loadMoreRef}
            className="w-full"
            style={{ position: "absolute", top: `${Math.max(rowVirtualizer.getTotalSize() - 1, 0)}px`, height: "1px" }}
            aria-hidden="true"
          />
        ) : null}
      </div>
    </div>
  );
}

function TrackRow({
  track,
  items,
  onPlayCollection,
}: {
  track: MediaItem;
  items: MediaItem[];
  onPlayCollection: (items: MediaItem[], startItem?: MediaItem) => void;
}) {
  return (
    <button
      type="button"
      className="music-track-row"
      onClick={() => onPlayCollection(items, track)}
    >
      <span className="music-track-index">
        {(track.track_number ?? 0) > 0 ? String(track.track_number).padStart(2, "0") : "•"}
      </span>
      <span className="music-track-main">
        <span className="music-track-title">{track.title}</span>
        <span className="music-track-meta">
          {track.artist || "Unknown Artist"}
          {track.album ? ` • ${track.album}` : ""}
        </span>
      </span>
      <span className="music-track-duration">{formatRuntime(track.duration)}</span>
    </button>
  );
}

function MusicSection({
  title,
  count,
  children,
}: {
  title: string;
  count: string;
  children: ReactNode;
}) {
  return (
    <section className="music-section">
      <div className="music-section-header">
        <h2 className="music-section-title">{title}</h2>
        <span className="music-section-count">{count}</span>
      </div>
      {children}
    </section>
  );
}

function MusicPlaceholderSection({ title, description }: { title: string; description: string }) {
  return (
    <section className="music-placeholder">
      <div className="music-section-header">
        <h2 className="music-section-title">{title}</h2>
      </div>
      <p className="music-placeholder-copy">{description}</p>
    </section>
  );
}
