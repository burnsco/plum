import { cn } from "@/lib/utils";

type RatingSource = "imdb" | "tmdb";
type RatingBadgeSize = "sm" | "md";

function normalizeRatingSource(label?: string): RatingSource | null {
  const normalized = label?.trim().toLowerCase();
  if (normalized === "imdb") return "imdb";
  if (normalized === "tmdb") return "tmdb";
  return null;
}

export function RatingBadge({
  label,
  value,
  size = "sm",
  className,
}: {
  label?: string;
  value?: number;
  size?: RatingBadgeSize;
  className?: string;
}) {
  const source = normalizeRatingSource(label);
  if (source == null || value == null || value <= 0) {
    return null;
  }

  const isMedium = size === "md";

  if (source === "imdb") {
    return (
      <span
        className={cn(
          "inline-flex items-center gap-1.5 rounded-full border border-[#f5c518]/30 bg-[#f5c518]/10 pr-2 text-[#f5c518]",
          isMedium ? "py-1 pl-1 text-sm" : "py-0.5 pl-0.5 text-[0.72rem]",
          className,
        )}
      >
        <span
          className={cn(
            "inline-flex items-center justify-center rounded-[0.45rem] bg-[#f5c518] font-black text-[#101010] shadow-[inset_0_-1px_0_rgba(0,0,0,0.18)]",
            isMedium ? "px-2.5 py-1 text-[0.8rem]" : "px-2 py-0.5 text-[0.64rem]",
          )}
        >
          IMDb
        </span>
        <span className={cn("font-semibold tabular-nums", isMedium ? "text-sm" : "text-[0.76rem]")}>
          {value.toFixed(1)}
        </span>
      </span>
    );
  }

  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border border-[#01d277]/30 bg-[#03261b]/85 pr-2 text-[#7fffd0]",
        isMedium ? "py-1 pl-1 text-sm" : "py-0.5 pl-0.5 text-[0.72rem]",
        className,
      )}
    >
      <span
        className={cn(
          "inline-flex items-center justify-center rounded-[0.45rem] bg-[linear-gradient(135deg,#90cea1_0%,#3cbec9_52%,#01b4e4_100%)] font-black lowercase text-[#032b25]",
          isMedium ? "px-2.5 py-1 text-[0.8rem]" : "px-2 py-0.5 text-[0.64rem]",
        )}
      >
        tmdb
      </span>
      <span className={cn("font-semibold tabular-nums", isMedium ? "text-sm" : "text-[0.76rem]")}>
        {value.toFixed(1)}
      </span>
    </span>
  );
}
