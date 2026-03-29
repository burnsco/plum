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
  const navItemClass =
    "flex items-center gap-3 rounded-[var(--radius-md)] border px-3 py-2.5 text-sm font-medium transition-colors";

  return (
    <aside className="hidden w-72 shrink-0 border-r border-[var(--nebula-border)] bg-[color-mix(in_srgb,var(--nebula-panel)_72%,black_28%)]/88 backdrop-blur-xl md:flex md:flex-col">
      <nav className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto px-4 py-5" aria-label="Libraries">
        <div className="space-y-1">
          <div className="px-2 text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--nebula-muted)]">
            Browse
          </div>
        <Link
          to="/"
          className={cn(
            navItemClass,
            isHomeRoute
              ? "border-[color-mix(in_srgb,var(--nebula-accent)_28%,var(--nebula-border))] bg-[var(--nebula-accent-soft)] text-[var(--nebula-accent)]"
              : "border-transparent text-[var(--nebula-muted)] hover:border-[var(--nebula-border)] hover:bg-[var(--nebula-panel-alt)] hover:text-[var(--nebula-text)]",
          )}
        >
          <Home className="size-4 shrink-0" />
          <span className="truncate">Home</span>
        </Link>
        <Link
          to="/discover"
          className={cn(
            navItemClass,
            isDiscoverRoute
              ? "border-[color-mix(in_srgb,var(--nebula-accent)_28%,var(--nebula-border))] bg-[var(--nebula-accent-soft)] text-[var(--nebula-accent)]"
              : "border-transparent text-[var(--nebula-muted)] hover:border-[var(--nebula-border)] hover:bg-[var(--nebula-panel-alt)] hover:text-[var(--nebula-text)]",
          )}
        >
          <Compass className="size-4 shrink-0" />
          <span className="truncate">Discover</span>
        </Link>
        </div>
        <div className="space-y-1">
        <div className="px-2 text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--nebula-muted)]">
          Libraries
        </div>
        {isLoading ? (
          <div className="px-2 py-2 text-sm text-[var(--nebula-muted)] italic">Loading libraries…</div>
        ) : (
          libraries.map((lib) => {
            const isActive = activeId === lib.id;
            const identifyPhase = getLibraryPhase(lib.id);
            const scanStatus = getLibraryScanStatus(lib.id);
            const activity = getLibraryActivity({
              scanPhase: scanStatus?.phase,
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
                className={cn(
                  navItemClass,
                  isActive
                    ? "border-[color-mix(in_srgb,var(--nebula-accent)_28%,var(--nebula-border))] bg-[var(--nebula-accent-soft)] text-[var(--nebula-accent)]"
                    : "border-transparent text-[var(--nebula-muted)] hover:border-[var(--nebula-border)] hover:bg-[var(--nebula-panel-alt)] hover:text-[var(--nebula-text)]",
                  isBusy &&
                    "border-[color-mix(in_srgb,var(--nebula-accent)_20%,var(--nebula-border))]",
                )}
              >
                <LibraryIcon lib={lib} />
                <span className="min-w-0 truncate">{getLibraryTabLabel(lib)}</span>
                {(isBusy || isFailed) && (
                  <span
                    className="ml-auto flex shrink-0 items-center"
                    data-testid={`library-identifying-${lib.id}`}
                    aria-hidden="true"
                  >
                    <span
                      className="relative flex size-2.5 items-center justify-center"
                    >
                      {showActivePulse && !isFailed && (
                        <span className="absolute inline-flex size-full animate-ping rounded-full bg-[var(--nebula-accent)] opacity-45" />
                      )}
                      <span
                        className={cn(
                          "relative size-2 rounded-full",
                          isFailed
                            ? "bg-rose-400"
                            : "bg-[var(--nebula-accent)] shadow-[0_0_10px_var(--nebula-accent)]",
                        )}
                      />
                    </span>
                  </span>
                )}
              </Link>
            );
          })
        )}
        </div>
      </nav>
    </aside>
  );
}
