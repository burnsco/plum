import { memo } from "react";
import { Captions, ExternalLink, Image, ScanSearch } from "lucide-react";
import { ContextMenuItem, ContextMenuSeparator } from "@/components/ui/context-menu";

export type MovieLibraryCardContextMenuProps = {
  refreshTracksDisabled: boolean;
  onChangePoster: () => void;
  onRescanTracks: () => void;
  onIdentify: () => void;
  onOpenDetails: () => void;
};

export const MovieLibraryCardContextMenu = memo(function MovieLibraryCardContextMenu({
  refreshTracksDisabled,
  onChangePoster,
  onRescanTracks,
  onIdentify,
  onOpenDetails,
}: MovieLibraryCardContextMenuProps) {
  return (
    <>
      <ContextMenuItem onSelect={onChangePoster}>
        <Image className="size-4 text-(--plum-muted)" />
        Change poster…
      </ContextMenuItem>
      <ContextMenuSeparator />
      <ContextMenuItem disabled={refreshTracksDisabled} onSelect={onRescanTracks}>
        <Captions className="size-4 text-(--plum-muted)" />
        Rescan tracks & subtitles
      </ContextMenuItem>
      <ContextMenuSeparator />
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
