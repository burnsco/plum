import { useVirtualContainerMetrics } from "@/lib/virtualization";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useRef, type CSSProperties } from "react";
import MediaCard from "./MediaCard";
import type { PosterAspectRatio, PosterGridItem } from "./types";

interface Props {
  items: PosterGridItem[];
  compact?: boolean;
  aspectRatio?: PosterAspectRatio;
}

const DEFAULT_CARD_WIDTH = 180;
const DEFAULT_CARD_GAP = 24;
const COMPACT_CARD_WIDTH = 150;
const COMPACT_CARD_GAP = 16;
const GRID_VIRTUALIZATION_THRESHOLD = 80;

function getAspectRatioValue(ratio: PosterAspectRatio = "poster"): number {
  switch (ratio) {
    case "cinema":
      return 3 / 4; // 0.75
    case "square":
      return 1;
    case "landscape":
      return 16 / 9;
    case "poster":
    default:
      return 2 / 3; // 0.66
  }
}

export function LibraryPosterGrid({ items, compact = false, aspectRatio = "poster" }: Props) {
  const rootRef = useRef<HTMLDivElement>(null);
  const { scrollElement, width, scrollMargin } = useVirtualContainerMetrics(rootRef);
  const cardWidth = compact ? COMPACT_CARD_WIDTH : DEFAULT_CARD_WIDTH;
  const gap = compact ? COMPACT_CARD_GAP : DEFAULT_CARD_GAP;
  const ratioValue = getAspectRatioValue(aspectRatio);
  const posterHeight = cardWidth / ratioValue;
  const infoHeight = compact ? 70 : 80;
  const rowHeight = posterHeight + infoHeight + gap;

  const columns = Math.max(1, Math.floor((Math.max(width, cardWidth) + gap) / (cardWidth + gap)));
  const shouldVirtualize =
    items.length >= GRID_VIRTUALIZATION_THRESHOLD &&
    scrollElement != null &&
    width > 0 &&
    typeof ResizeObserver !== "undefined";

  const rowCount = Math.ceil(items.length / columns);
  const rowVirtualizer = useVirtualizer({
    count: shouldVirtualize ? rowCount : 0,
    getScrollElement: () => scrollElement,
    estimateSize: () => rowHeight,
    overscan: compact ? 3 : 2,
    scrollMargin,
  });

  if (!shouldVirtualize) {
    return (
      <div
        ref={rootRef}
        className={`show-cards-grid${compact ? " show-cards-grid--compact" : ""}`}
        style={
          {
            "--poster-ratio": getAspectRatioValue(aspectRatio),
            "--poster-card-width": `${cardWidth}px`,
            "--poster-grid-gap": `${gap}px`,
          } as CSSProperties
        }
      >
        {items.map((item) => (
          <MediaCard key={item.key} item={item} />
        ))}
      </div>
    );
  }

  return (
    <div
      ref={rootRef}
      className={`show-cards-grid show-cards-grid--virtual${compact ? " show-cards-grid--compact" : ""}`}
      style={
        {
          "--poster-columns": String(columns),
          "--poster-ratio": getAspectRatioValue(aspectRatio),
          "--poster-card-width": `${cardWidth}px`,
          "--poster-grid-gap": `${gap}px`,
        } as CSSProperties
      }
    >
      <div
        className="show-cards-grid__spacer"
        style={{ height: `${rowVirtualizer.getTotalSize()}px` }}
      >
        {rowVirtualizer.getVirtualItems().map((virtualRow) => {
          const start = virtualRow.index * columns;
          const rowItems = items.slice(start, start + columns);
          return (
            <div
              key={virtualRow.key}
              className="show-cards-grid__row"
              style={{
                transform: `translateY(${virtualRow.start - scrollMargin}px)`,
                gap: `${gap}px`,
              }}
            >
              {rowItems.map((item) => (
                <MediaCard key={item.key} item={item} />
              ))}
            </div>
          );
        })}
      </div>
    </div>
  );
}
