import { Slider } from "@/components/ui/slider";
import { cn } from "@/lib/utils";
import { LayoutGrid, LayoutList, Table2 } from "lucide-react";

export type LayoutMode = "grid" | "detail" | "table";

interface LibraryViewControlsProps {
  cardWidth: number;
  onCardWidthChange: (width: number) => void;
  layoutMode: LayoutMode;
  onLayoutModeChange: (mode: LayoutMode) => void;
  className?: string;
}

const CARD_WIDTH_MIN = 110;
const CARD_WIDTH_MAX = 260;

const LAYOUT_OPTIONS: { mode: LayoutMode; icon: React.ReactNode; label: string }[] = [
  { mode: "grid", icon: <LayoutGrid className="size-4" />, label: "Grid view" },
  { mode: "detail", icon: <LayoutList className="size-4" />, label: "Detail view" },
  { mode: "table", icon: <Table2 className="size-4" />, label: "Table view" },
];

export function LibraryViewControls({
  cardWidth,
  onCardWidthChange,
  layoutMode,
  onLayoutModeChange,
  className,
}: LibraryViewControlsProps) {
  return (
    <div className={cn("flex items-center gap-4", className)}>
      {/* Card size slider — only visible in grid mode */}
      {layoutMode === "grid" && (
        <div className="flex items-center gap-3">
          <Slider
            id="card-size-slider"
            min={CARD_WIDTH_MIN}
            max={CARD_WIDTH_MAX}
            step={10}
            value={[cardWidth]}
            onValueChange={([v]) => onCardWidthChange(v ?? cardWidth)}
            className="w-28"
            aria-label="Card size"
          />
        </div>
      )}

      {/* Layout mode toggle */}
      <div
        className="flex items-center rounded-[var(--radius-md)] border border-[var(--plum-border)] bg-[rgba(255,255,255,0.03)] p-0.5"
        role="group"
        aria-label="Layout mode"
      >
        {LAYOUT_OPTIONS.map(({ mode, icon, label }) => (
          <button
            key={mode}
            type="button"
            aria-label={label}
            aria-pressed={layoutMode === mode}
            onClick={() => onLayoutModeChange(mode)}
            className={cn(
              "flex items-center justify-center rounded-[calc(var(--radius-md)-2px)] p-1.5 transition-colors",
              layoutMode === mode
                ? "bg-[rgba(255,255,255,0.1)] text-[var(--plum-text)]"
                : "text-[var(--plum-muted)] hover:text-[var(--plum-text)]",
            )}
          >
            {icon}
          </button>
        ))}
      </div>
    </div>
  );
}
