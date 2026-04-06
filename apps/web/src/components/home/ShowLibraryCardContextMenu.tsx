import { memo } from "react";
import { Captions, ExternalLink, Image, RefreshCw, ScanSearch } from "lucide-react";
import { ContextMenuItem, ContextMenuSeparator } from "@/components/ui/context-menu";
export type ShowLibraryCardContextMenuProps = {
  refreshShowDisabled: boolean;
  refreshTracksDisabled: boolean;
  onChangePoster: () => void;
  onRefreshShow: () => void;
  onRescanTracks: () => void;
  onIdentify: () => void;
  onOpenDetails: () => void;
};

export const ShowLibraryCardContextMenu = memo(function ShowLibraryCardContextMenu({
  refreshShowDisabled,
  refreshTracksDisabled,
  onChangePoster,
  onRefreshShow,
  onRescanTracks,
  onIdentify,
  onOpenDetails,
}: ShowLibraryCardContextMenuProps) {
  return (
    <>
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
