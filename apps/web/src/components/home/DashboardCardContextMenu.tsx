import { memo } from "react";
import { CirclePlay, ExternalLink, EyeOff } from "lucide-react";
import { ContextMenuItem, ContextMenuSeparator } from "@/components/ui/context-menu";

type DashboardCardContextMenuProps = {
  canOpenDetails: boolean;
  canRemoveFromContinueWatching: boolean;
  removeDisabled: boolean;
  onPlay: () => void;
  onOpenDetails: () => void;
  onRemoveFromContinueWatching: () => void;
};

export const DashboardCardContextMenu = memo(function DashboardCardContextMenu({
  canOpenDetails,
  canRemoveFromContinueWatching,
  removeDisabled,
  onPlay,
  onOpenDetails,
  onRemoveFromContinueWatching,
}: DashboardCardContextMenuProps) {
  return (
    <>
      <ContextMenuItem onSelect={onPlay}>
        <CirclePlay className="size-4 text-(--plum-muted)" />
        Play
      </ContextMenuItem>
      <ContextMenuItem disabled={!canOpenDetails} onSelect={onOpenDetails}>
        <ExternalLink className="size-4 text-(--plum-muted)" />
        Open details
      </ContextMenuItem>
      {canRemoveFromContinueWatching ? (
        <>
          <ContextMenuSeparator />
          <ContextMenuItem disabled={removeDisabled} onSelect={onRemoveFromContinueWatching}>
            <EyeOff className="size-4 text-(--plum-muted)" />
            Remove from continue watching
          </ContextMenuItem>
        </>
      ) : null}
    </>
  );
});
