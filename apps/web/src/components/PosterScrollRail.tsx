import MediaCard from "@/components/MediaCard";
import type { PosterGridItem } from "@/components/types";
import { HorizontalScrollRail } from "@/components/ui/page";
import { cn } from "@/lib/utils";

/** Matches Discover horizontal shelves: flex row, hidden scrollbar, chevron paging. */
const RAIL_CONTENT_CLASS =
  "gap-4 overflow-x-auto pb-2 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden";

export function PosterScrollRail({
  label,
  items,
  cardClassName,
}: {
  label: string;
  items: PosterGridItem[];
  /** Poster column width; default matches Discover (`w-44`). */
  cardClassName?: string;
}) {
  return (
    <HorizontalScrollRail label={label} contentClassName={RAIL_CONTENT_CLASS}>
      {items.map((item, index) => (
        <div key={item.key} className={cn("w-44 shrink-0", cardClassName)}>
          <MediaCard item={item} index={index} />
        </div>
      ))}
    </HorizontalScrollRail>
  );
}
