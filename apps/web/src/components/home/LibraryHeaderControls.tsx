import { LibraryViewControls } from "@/components/LibraryViewControls";
import type { LayoutMode } from "@/components/LibraryViewControls";

type LibraryHeaderControlsProps = {
  cardWidth: number;
  layoutMode: LayoutMode;
  onCardWidthChange: (width: number) => void;
  onClearUnidentifiedFilter: () => void;
  onLayoutModeChange: (mode: LayoutMode) => void;
  title: string;
  unidentifiedOnly: boolean;
};

export function LibraryHeaderControls({
  cardWidth,
  layoutMode,
  onCardWidthChange,
  onClearUnidentifiedFilter,
  onLayoutModeChange,
  title,
  unidentifiedOnly,
}: LibraryHeaderControlsProps) {
  return (
    <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between mb-4">
      <div className="min-w-0">
        <h2 className="text-base font-semibold text-(--plum-text) truncate">{title}</h2>
        {unidentifiedOnly ? (
          <p className="mt-1 text-xs text-(--plum-text-2)">
            Showing titles that still need identification.
            <button
              type="button"
              className="ml-2 text-(--plum-accent) hover:underline"
              onClick={onClearUnidentifiedFilter}
            >
              Show all
            </button>
          </p>
        ) : null}
      </div>
      <LibraryViewControls
        cardWidth={cardWidth}
        onCardWidthChange={onCardWidthChange}
        layoutMode={layoutMode}
        onLayoutModeChange={onLayoutModeChange}
      />
    </div>
  );
}
