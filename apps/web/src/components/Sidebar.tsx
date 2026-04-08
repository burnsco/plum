import { Link, useLocation, useParams } from "react-router-dom";
import { useLibraries, useRefreshLibraryPlaybackTracks } from "@/queries";
import { getLibraryActivity } from "@/lib/libraryActivity";
import { getLibraryTabLabel } from "@/lib/showGrouping";
import { cn } from "@/lib/utils";
import {
  ArrowDownCircle,
  Captions,
  Compass,
  Film,
  Home,
  Music,
  RefreshCw,
  ScanLine,
  Tv,
} from "lucide-react";
import { toast } from "sonner";
import type { Library } from "@/api";
import { useIdentifyQueue } from "@/contexts/IdentifyQueueContext";
import { useScanQueue } from "@/contexts/ScanQueueContext";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";

function LibraryIcon({ lib }: { lib: Library }) {
  if (lib.type === "music") return <Music className="size-4 shrink-0" />;
  if (lib.type === "movie") return <Film className="size-4 shrink-0" />;
  return <Tv className="size-4 shrink-0" />;
}

export function Sidebar() {
  const { libraryId } = useParams();
  const { data: libraries = [], isLoading } = useLibraries();
  const refreshLibraryPlaybackTracksMutation = useRefreshLibraryPlaybackTracks();
  const { getLibraryPhase, queueLibraryIdentify } = useIdentifyQueue();
  const { getLibraryScanStatus, queueLibraryScan } = useScanQueue();
  const location = useLocation();
  const activeId = libraryId ? parseInt(libraryId, 10) : null;
  const isHomeRoute = location.pathname === "/";
  const isDiscoverRoute = location.pathname === "/discover" || location.pathname.startsWith("/discover/");
  const isDownloadsRoute =
    location.pathname === "/downloads" || location.pathname.startsWith("/downloads/");
  const navItemBase =
    "relative flex items-center gap-3 px-4 py-2.5 text-sm font-medium rounded-lg mx-2 transition-all cursor-pointer select-none";
  const navItemActive =
    "text-[var(--plum-text)] bg-[rgba(181,123,255,0.1)] before:absolute before:left-0 before:top-1/2 before:-translate-y-1/2 before:h-5 before:w-[3px] before:rounded-r-full before:bg-[var(--plum-accent)] before:content-[''] shadow-[0_0_20px_rgba(139,92,246,0.08)]";
  const navItemInactive =
    "text-[var(--plum-text-2)] hover:text-[var(--plum-text)] hover:bg-[var(--plum-accent-subtle)]";

  return (
    <aside
      className={
        __PLUM_VITEST_LAYOUT__
          ? "flex shrink-0 border-r border-(--plum-chrome-border) bg-(--plum-sidebar-bg) w-[var(--plum-sidebar-width)] flex-col"
          : "hidden shrink-0 border-r border-(--plum-chrome-border) bg-(--plum-sidebar-bg) md:flex md:w-[var(--plum-sidebar-width)] md:flex-col"
      }
      style={{ boxShadow: "inset -1px 0 0 var(--plum-chrome-border)" }}
    >
      <nav className="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto py-4" aria-label="Libraries">
        {/* Section: Browse */}
        <div className="px-4 pb-1 pt-2 text-[10px] font-semibold uppercase tracking-[0.18em] text-(--plum-text-2)">
          Browse
        </div>
        <Link
          to="/"
          className={cn(navItemBase, isHomeRoute ? navItemActive : navItemInactive)}
        >
          <Home className="size-4 shrink-0" />
          <span className="truncate">Home</span>
        </Link>
        <Link
          to="/discover"
          className={cn(navItemBase, isDiscoverRoute ? navItemActive : navItemInactive)}
        >
          <Compass className="size-4 shrink-0" />
          <span className="truncate">Discover</span>
        </Link>
        <Link
          to="/downloads"
          className={cn(navItemBase, isDownloadsRoute ? navItemActive : navItemInactive)}
        >
          <ArrowDownCircle className="size-4 shrink-0" />
          <span className="truncate">Downloads</span>
        </Link>

        {/* Divider */}
        <div className="mx-4 my-3 h-px bg-(--plum-chrome-border)" />

        {/* Section: Libraries */}
        <div className="px-4 pb-1 text-[10px] font-semibold uppercase tracking-[0.18em] text-(--plum-text-2)">
          Libraries
        </div>
        {isLoading ? (
          <div className="px-4 py-2 text-sm text-(--plum-text-2) italic">Loading…</div>
        ) : (
          libraries.map((lib) => {
            const isActive = activeId === lib.id;
            const identifyPhase = getLibraryPhase(lib.id);
            const scanStatus = getLibraryScanStatus(lib.id);
            const supportsMetadataRefresh = lib.type !== "music";
            const isBusy =
              getLibraryActivity({
                scanPhase: scanStatus?.phase,
                enrichmentPhase: scanStatus?.enrichmentPhase,
                enriching: scanStatus?.enriching === true,
                identifyPhase: scanStatus?.identifyPhase,
                localIdentifyPhase: identifyPhase,
              }) != null;
            const isIdentifying =
              identifyPhase === "queued" ||
              identifyPhase === "identifying" ||
              scanStatus?.identifyPhase === "queued" ||
              scanStatus?.identifyPhase === "identifying";
            const label = getLibraryTabLabel(lib);
            return (
              <ContextMenu key={lib.id}>
                <ContextMenuTrigger asChild>
                  <Link
                    to={`/library/${lib.id}`}
                    title={label}
                    className={cn(navItemBase, isActive ? navItemActive : navItemInactive)}
                  >
                    <LibraryIcon lib={lib} />
                    <span className="min-w-0 truncate">{label}</span>
                  </Link>
                </ContextMenuTrigger>
                <ContextMenuContent>
                  <ContextMenuItem
                    disabled={isBusy}
                    onSelect={() => void queueLibraryScan(lib.id, { identify: false })}
                  >
                    <ScanLine className="size-4 text-(--plum-muted)" />
                    Scan for changes
                  </ContextMenuItem>
                  {supportsMetadataRefresh ? (
                    <>
                      <ContextMenuSeparator />
                      <ContextMenuItem
                        disabled={isIdentifying}
                        onSelect={() => queueLibraryIdentify(lib.id)}
                      >
                        <RefreshCw className="size-4 text-(--plum-muted)" />
                        Refresh metadata
                      </ContextMenuItem>
                      <ContextMenuItem
                        disabled={isBusy || refreshLibraryPlaybackTracksMutation.isPending}
                        onSelect={() => {
                          void refreshLibraryPlaybackTracksMutation
                            .mutateAsync({ libraryId: lib.id })
                            .then(() => {
                              toast.success(
                                `Rescanning tracks and subtitles for “${label}” in the background. This can take several minutes on large libraries; watch the server log for progress.`,
                              );
                            })
                            .catch((err: unknown) => {
                              toast.error(
                                err instanceof Error
                                  ? err.message
                                  : "Could not start library tracks rescan.",
                              );
                            });
                        }}
                      >
                        <Captions className="size-4 text-(--plum-muted)" />
                        Rescan tracks & subtitles (entire library)
                      </ContextMenuItem>
                    </>
                  ) : null}
                </ContextMenuContent>
              </ContextMenu>
            );
          })
        )}
      </nav>
    </aside>
  );
}
