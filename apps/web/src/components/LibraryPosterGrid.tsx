import { type CSSProperties } from "react";
import MediaCard from "./MediaCard";
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

export function LibraryPosterGrid({
  items,
  compact = false,
  aspectRatio = "poster",
  cardWidth: externalCardWidth,
}: Props) {
  const baseCardWidth = compact ? COMPACT_CARD_WIDTH : DEFAULT_CARD_WIDTH;
  const cardWidth = externalCardWidth ?? baseCardWidth;
  const gap = compact ? COMPACT_CARD_GAP : DEFAULT_CARD_GAP;
  const ratioValue = getAspectRatioValue(aspectRatio);

  return (
    <div
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
