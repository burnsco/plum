import { useRef, type CSSProperties } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import MediaCard from "./MediaCard";
import { useVirtualContainerMetrics } from "@/lib/virtualization";
import type { PosterAspectRatio, PosterGridItem } from "./types";

interface Props {
  items: PosterGridItem[];
  compact?: boolean;
  aspectRatio?: PosterAspectRatio;
  /** Externally controlled card width in px (overrides compact prop widths) */
  cardWidth?: number;
}

const DEFAULT_CARD_WIDTH = 180;
const DEFAULT_CARD_GAP = 20;
const COMPACT_CARD_WIDTH = 150;
const COMPACT_CARD_GAP = 16;
const GRID_VIRTUALIZATION_THRESHOLD = 120;
const DEFAULT_INFO_HEIGHT = 110;
const COMPACT_INFO_HEIGHT = 96;

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

function getRowEstimate(cardWidth: number, ratioValue: number, gap: number, compact: boolean) {
  const infoHeight = compact ? COMPACT_INFO_HEIGHT : DEFAULT_INFO_HEIGHT;
  return Math.ceil(cardWidth / ratioValue + infoHeight + gap);
}

export function LibraryPosterGrid({
  items,
  compact = false,
  aspectRatio = "poster",
  cardWidth: externalCardWidth,
}: Props) {
  const rootRef = useRef<HTMLDivElement>(null);
  const { scrollElement, width, scrollMargin } = useVirtualContainerMetrics(rootRef);
  const baseCardWidth = compact ? COMPACT_CARD_WIDTH : DEFAULT_CARD_WIDTH;
  const cardWidth = externalCardWidth ?? baseCardWidth;
  const gap = compact ? COMPACT_CARD_GAP : DEFAULT_CARD_GAP;
  const ratioValue = getAspectRatioValue(aspectRatio);
  const columns = Math.max(1, Math.floor((Math.max(width, cardWidth) + gap) / (cardWidth + gap)));
  const rowCount = Math.ceil(items.length / columns);
  const shouldVirtualize =
    items.length >= GRID_VIRTUALIZATION_THRESHOLD &&
    scrollElement != null &&
    width > 0 &&
    typeof ResizeObserver !== "undefined";
  const estimatedCellWidth = Math.max(1, (width - gap * (columns - 1)) / columns);
  const rowVirtualizer = useVirtualizer({
    count: shouldVirtualize ? rowCount : 0,
    getScrollElement: () => scrollElement,
    estimateSize: () => getRowEstimate(estimatedCellWidth, ratioValue, gap, compact),
    overscan: compact ? 4 : 3,
    scrollMargin,
  });

  if (!shouldVirtualize) {
    return (
      <div
        ref={rootRef}
        className={`show-cards-grid${compact ? " show-cards-grid--compact" : ""}`}
        style={
          {
            "--poster-ratio": String(ratioValue),
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
          "--poster-ratio": String(ratioValue),
          "--poster-card-width": `${cardWidth}px`,
          "--poster-grid-gap": `${gap}px`,
          "--poster-columns": String(columns),
        } as CSSProperties
      }
    >
      <div className="show-cards-grid__spacer" style={{ height: `${rowVirtualizer.getTotalSize()}px` }}>
        {rowVirtualizer.getVirtualItems().map((virtualRow) => {
          const start = virtualRow.index * columns;
          const rowItems = items.slice(start, start + columns);
          return (
            <div
              key={virtualRow.key}
              className="show-cards-grid__row"
              style={{
                gap: `${gap}px`,
                transform: `translateY(${virtualRow.start - scrollMargin}px)`,
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
