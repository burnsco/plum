import { memo } from "react";
import { Captions, ExternalLink, Image, ListChecks, RefreshCw, ScanSearch } from "lucide-react";
import { ContextMenuItem, ContextMenuSeparator } from "@/components/ui/context-menu";
export type ShowLibraryCardContextMenuProps = {
  refreshShowDisabled: boolean;
  refreshTracksDisabled: boolean;
  markShowWatchedDisabled: boolean;
  onMarkShowWatchedAll: () => void;
  onChangePoster: () => void;
  onRefreshShow: () => void;
  onRescanTracks: () => void;
  onIdentify: () => void;
  onOpenDetails: () => void;
};

export const ShowLibraryCardContextMenu = memo(function ShowLibraryCardContextMenu({
  refreshShowDisabled,
  refreshTracksDisabled,
  markShowWatchedDisabled,
  onMarkShowWatchedAll,
  onChangePoster,
  onRefreshShow,
  onRescanTracks,
  onIdentify,
  onOpenDetails,
}: ShowLibraryCardContextMenuProps) {
  return (
    <>
      <ContextMenuItem disabled={markShowWatchedDisabled} onSelect={onMarkShowWatchedAll}>
        <ListChecks className="size-4 text-(--plum-muted)" />
        Mark show as watched
      </ContextMenuItem>
      <ContextMenuItem onSelect={onChangePoster}>
        <Image className="size-4 text-(--plum-muted)" />
        Change poster…
      </ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem disabled={refreshShowDisabled} onSelect={onRefreshShow}>
        <RefreshCw className="size-4 text-(--plum-muted)" />
        Refresh metadata
      </ContextMenuItem>
      <ContextMenuItem disabled={refreshTracksDisabled} onSelect={onRescanTracks}>
        <Captions className="size-4 text-(--plum-muted)" />
        Rescan tracks & subtitles (all episodes)
      </ContextMenuItem>
      <ContextMenuItem onSelect={onIdentify}>
        <ScanSearch className="size-4 text-(--plum-muted)" />
        Identify…
      </ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem onSelect={onOpenDetails}>
        <ExternalLink className="size-4 text-(--plum-muted)" />
        Open details
      </ContextMenuItem>
    </>
  );
});
