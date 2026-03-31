import { Link, useLocation, useParams } from "react-router-dom";
import { useLibraries } from "@/queries";
import { getLibraryActivity } from "@/lib/libraryActivity";
import { getLibraryTabLabel } from "@/lib/showGrouping";
import { cn } from "@/lib/utils";
import { Compass, Film, Home, Music, Tv } from "lucide-react";
import type { Library } from "@/api";
import { useIdentifyQueue } from "@/contexts/IdentifyQueueContext";
import { useScanQueue } from "@/contexts/ScanQueueContext";

function LibraryIcon({ lib }: { lib: Library }) {
  if (lib.type === "music") return <Music className="size-4 shrink-0" />;
  if (lib.type === "movie") return <Film className="size-4 shrink-0" />;
  return <Tv className="size-4 shrink-0" />;
}

export function Sidebar() {
  const { libraryId } = useParams();
  const { data: libraries = [], isLoading } = useLibraries();
  const { getLibraryPhase } = useIdentifyQueue();
  const { getLibraryScanStatus } = useScanQueue();
  const location = useLocation();
  const activeId = libraryId ? parseInt(libraryId, 10) : null;
  const isHomeRoute = location.pathname === "/";
  const isDiscoverRoute = location.pathname === "/discover" || location.pathname.startsWith("/discover/");

  const navItemBase =
    "relative flex items-center gap-3 px-4 py-2.5 text-sm font-medium rounded-lg mx-2 transition-all cursor-pointer select-none";
  const navItemActive =
    "text-[var(--plum-text)] bg-[rgba(181,123,255,0.1)] before:absolute before:left-0 before:top-1/2 before:-translate-y-1/2 before:h-5 before:w-[3px] before:rounded-r-full before:bg-[var(--plum-accent)] before:content-[''] shadow-[0_0_20px_rgba(139,92,246,0.08)]";
  const navItemInactive =
    "text-[var(--plum-muted)] hover:text-[var(--plum-text)] hover:bg-[rgba(181,123,255,0.06)]";

  return (
    <aside className="hidden w-60 shrink-0 border-r border-[rgba(181,123,255,0.1)] bg-[rgba(10,8,18,0.98)] md:flex md:flex-col" style={{boxShadow: "inset -1px 0 0 rgba(181,123,255,0.06)", backdropFilter: "blur(16px)"}}>
      <nav className="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto py-4" aria-label="Libraries">
        {/* Section: Browse */}
        <div className="px-4 pb-1 pt-2 text-[10px] font-semibold uppercase tracking-[0.18em] text-[rgba(181,123,255,0.45)]">
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

        {/* Divider */}
        <div className="mx-4 my-3 h-px bg-[rgba(181,123,255,0.1)]" />

        {/* Section: Libraries */}
        <div className="px-4 pb-1 text-[10px] font-semibold uppercase tracking-[0.18em] text-[rgba(181,123,255,0.45)]">
          Libraries
        </div>
        {isLoading ? (
          <div className="px-4 py-2 text-sm text-[var(--plum-muted)] italic">Loading…</div>
        ) : (
          libraries.map((lib) => {
            const isActive = activeId === lib.id;
            const identifyPhase = getLibraryPhase(lib.id);
            const scanStatus = getLibraryScanStatus(lib.id);
            const activity = getLibraryActivity({
              scanPhase: scanStatus?.phase,
              enrichmentPhase: scanStatus?.enrichmentPhase,
              enriching: scanStatus?.enriching === true,
              identifyPhase: scanStatus?.identifyPhase,
              localIdentifyPhase: identifyPhase,
            });
            const isFailed =
              scanStatus?.phase === "failed" || scanStatus?.identifyPhase === "failed";
            const isBusy = activity != null;
            const showActivePulse = activity != null && activity !== "identify-queued";
            return (
              <Link
                key={lib.id}
                to={`/library/${lib.id}`}
                className={cn(navItemBase, isActive ? navItemActive : navItemInactive)}
              >
                <LibraryIcon lib={lib} />
                <span className="min-w-0 truncate">{getLibraryTabLabel(lib)}</span>
                {(isBusy || isFailed) && (
                  <span
                    className="ml-auto flex shrink-0 items-center"
                    data-testid={`library-identifying-${lib.id}`}
                    aria-hidden="true"
                  >
                    <span className="relative flex size-2.5 items-center justify-center">
                      {showActivePulse && !isFailed && (
                        <span className="absolute inline-flex size-full animate-ping rounded-full bg-[var(--plum-accent)] opacity-45" />
                      )}
                      <span
                        className={cn(
                          "relative size-2 rounded-full",
                          isFailed
                            ? "bg-rose-400"
                            : "bg-[var(--plum-accent)] shadow-[0_0_10px_var(--plum-accent)]",
                        )}
                      />
                    </span>
                  </span>
                )}
              </Link>
            );
          })
        )}
      </nav>
    </aside>
  );
}
