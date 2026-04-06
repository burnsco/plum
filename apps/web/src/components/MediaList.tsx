import { memo } from "react";
import { BASE_URL, type MediaItem } from "../api";
import { resolvePosterUrl } from "@plum/shared";

interface Props {
  items: MediaItem[];
  onSelect: (item: MediaItem) => void;
  onTranscode: (item: MediaItem) => void;
}

const MediaListCard = memo(function MediaListCard({
  item: m,
  onSelect,
  onTranscode,
}: {
  item: MediaItem;
  onSelect: (item: MediaItem) => void;
  onTranscode: (item: MediaItem) => void;
}) {
  return (
    <div
      className="media-card"
      role="button"
      tabIndex={0}
      onClick={() => onSelect(m)}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect(m);
        }
      }}
    >
      <div className="media-poster">
        <img
          src={resolvePosterUrl(m.poster_url, m.poster_path, "w200", BASE_URL) || "/placeholder-poster.svg"}
          alt={m.title}
        />
        <div className="media-type-overlay">{m.type}</div>
      </div>
      <div className="media-info">
        <div className="media-title" title={m.title}>
          {m.title}
        </div>
        <div className="media-subtitle">
          {m.release_date && <span>{m.release_date.split("-")[0]}</span>}
          {m.type === "tv" || m.type === "anime" ? (
            (m.show_vote_average ?? 0) > 0 ? (
              <span>⭐ {m.show_vote_average!.toFixed(1)}</span>
            ) : null
          ) : m.vote_average ? (
            <span>⭐ {m.vote_average.toFixed(1)}</span>
          ) : null}
        </div>
      </div>
      <button
        className="play-button"
        onClick={(e) => {
          e.stopPropagation();
          onTranscode(m);
        }}
      >
        Play
      </button>
    </div>
  );
});

export function MediaList({ items, onSelect, onTranscode }: Props) {
  if (items.length === 0) {
    return <div className="empty-state">No media found in Plum.</div>;
  }

  return (
    <div className="media-grid">
      {items.map((m) => (
        <MediaListCard key={m.id} item={m} onSelect={onSelect} onTranscode={onTranscode} />
      ))}
    </div>
  );
}
