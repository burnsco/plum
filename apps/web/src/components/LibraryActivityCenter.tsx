import { useMemo } from "react";
import { Activity } from "lucide-react";
import { type Library, type LibraryScanActivityEntry, type LibraryScanStatus } from "@/api";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { useScanQueue } from "@/contexts/ScanQueueContext";
import { getLibraryActivity, getLibraryActivityLabel } from "@/lib/libraryActivity";
import { getLibraryTabLabel } from "@/lib/showGrouping";
import { cn } from "@/lib/utils";
import { useLibraries } from "@/queries";

type ActivityLibraryStatus = {
  library: Library;
  status: LibraryScanStatus;
};

function getStatusLabel(status: LibraryScanStatus) {
  if (status.phase === "failed" || status.identifyPhase === "failed") {
    return "Failed";
  }
  const activity = getLibraryActivity({
    scanPhase: status.phase,
    enriching: status.enriching,
    identifyPhase: status.identifyPhase,
  });
  return getLibraryActivityLabel(activity) ?? (status.phase === "queued" ? "Queued" : "Idle");
}

function getStatusSortOrder(status: LibraryScanStatus) {
  if (status.phase === "failed" || status.identifyPhase === "failed") return 4;
  if (status.identifyPhase === "queued" || status.identifyPhase === "identifying") return 3;
  if (status.enriching) return 2;
  if (status.phase === "queued") return 1;
  if (status.phase === "scanning") return 0;

  switch (status.activity?.stage) {
    case "discovery":
      return 0;
    case "queued":
      return 1;
    case "enrichment":
      return 2;
    case "identify":
      return 3;
    case "failed":
      return 4;
    default:
      return 5;
  }
}

function formatActivityPath(entry: LibraryScanActivityEntry) {
  const basePath = entry.relativePath.trim();
  if (entry.target === "directory") {
    return basePath ? `${basePath}/` : "Library root/";
  }
  return basePath || "Library root";
}

function formatActivityPhase(phase: LibraryScanActivityEntry["phase"]) {
  switch (phase) {
    case "discovery":
      return "Import";
    case "enrichment":
      return "Finish";
    case "identify":
      return "Identify";
    default:
      return phase;
  }
}

function getRecentEntries(status: LibraryScanStatus) {
  const recent = status.activity?.recent ?? [];
  const current = status.activity?.current;
  return recent
    .filter((entry) => {
      if (current == null) return true;
      return !(
        entry.phase === current.phase &&
        entry.target === current.target &&
        entry.relativePath === current.relativePath &&
        entry.at === current.at
      );
    })
    .slice(0, 6);
}

function StatusCounters({ status }: { status: LibraryScanStatus }) {
  const counters = [
    `Processed ${status.processed}`,
    `Added ${status.added}`,
    `Updated ${status.updated}`,
    `Removed ${status.removed}`,
    `Unmatched ${status.unmatched}`,
    `Skipped ${status.skipped}`,
  ];
  if (status.identified > 0 || status.identifyFailed > 0) {
    counters.push(`Identified ${status.identified}`);
    counters.push(`Failed ${status.identifyFailed}`);
  }

  return (
    <div className="flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-[var(--plum-muted)]">
      {counters.map((counter) => (
        <span key={counter}>{counter}</span>
      ))}
    </div>
  );
}

function ActivityList({ status }: { status: LibraryScanStatus }) {
  const current = status.activity?.current;
  const recentEntries = getRecentEntries(status);

  if (current == null && recentEntries.length === 0) {
    return null;
  }

  return (
    <div className="space-y-2">
      {current && (
        <div className="space-y-1">
          <div className="text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--plum-muted)]">
            Current
          </div>
          <div className="rounded-[var(--radius-sm)] border border-[var(--plum-border)] bg-[var(--plum-bg)]/70 px-3 py-2 text-sm text-[var(--plum-text)]">
            <div className="font-medium">{formatActivityPath(current)}</div>
            <div className="text-[11px] uppercase tracking-[0.08em] text-[var(--plum-muted)]">
              {formatActivityPhase(current.phase)}
            </div>
          </div>
        </div>
      )}
      {recentEntries.length > 0 && (
        <div className="space-y-1">
          <div className="text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--plum-muted)]">
            Recent
          </div>
          <div className="space-y-1">
            {recentEntries.map((entry) => (
              <div
                key={`${entry.at}-${entry.phase}-${entry.target}-${entry.relativePath}`}
                className="flex items-center justify-between gap-3 rounded-[var(--radius-sm)] bg-[var(--plum-bg)]/50 px-3 py-2 text-sm"
              >
                <span className="min-w-0 truncate text-[var(--plum-text)]">
                  {formatActivityPath(entry)}
                </span>
                <span className="shrink-0 text-[11px] uppercase tracking-[0.08em] text-[var(--plum-muted)]">
                  {formatActivityPhase(entry.phase)}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function ActivityStatusCard({ library, status }: ActivityLibraryStatus) {
  const label = getStatusLabel(status);
  const detailLine =
    status.identifyPhase === "queued"
      ? "Waiting for an identify worker"
      : status.phase === "queued" && status.queuePosition > 0
      ? `Queue position ${status.queuePosition}`
      : status.nextRetryAt && (status.retryCount ?? 0) < (status.maxRetries ?? 0)
        ? `Retry ${status.retryCount ?? 0}/${status.maxRetries ?? 0}`
        : status.lastError || status.error || "";

  return (
    <section
      className="space-y-3 rounded-[var(--radius-md)] border border-[var(--plum-border)] bg-[var(--plum-panel)]/90 p-3"
      data-testid={`library-activity-status-${library.id}`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-[var(--plum-text)]">
            {getLibraryTabLabel(library)}
          </div>
          <div className="text-xs uppercase tracking-[0.1em] text-[var(--plum-muted)]">{label}</div>
        </div>
        <span
          className={cn(
            "rounded-full px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.12em]",
            label === "Failed"
              ? "bg-rose-500/15 text-rose-300"
              : "bg-[var(--plum-accent-soft)] text-[var(--plum-accent)]",
          )}
        >
          {label}
        </span>
      </div>

      <StatusCounters status={status} />

      {detailLine && <div className="text-xs text-[var(--plum-muted)]">{detailLine}</div>}

      <ActivityList status={status} />
    </section>
  );
}

export function LibraryActivityCenter() {
  const { data: libraries = [] } = useLibraries();
  const { activeLibraryIds, activityScanStatuses } = useScanQueue();

  const visibleStatuses = useMemo(() => {
    const libraryById = new Map(libraries.map((library) => [library.id, library]));
    return activityScanStatuses
      .map((status) => {
        const library = libraryById.get(status.libraryId);
        return library ? { library, status } : null;
      })
      .filter((value): value is ActivityLibraryStatus => value != null)
      .toSorted((left, right) => {
        const orderDiff = getStatusSortOrder(left.status) - getStatusSortOrder(right.status);
        if (orderDiff !== 0) return orderDiff;
        return getLibraryTabLabel(left.library).localeCompare(getLibraryTabLabel(right.library));
      });
  }, [activityScanStatuses, libraries]);

  const activeCount = activeLibraryIds.length;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="icon"
          size="icon"
          aria-label="Library activity"
          className={cn(
            "relative",
            activeCount > 0 &&
              "bg-[var(--plum-accent-soft)] text-[var(--plum-accent)] hover:bg-[var(--plum-accent-soft)]/80 hover:text-[var(--plum-accent)]",
          )}
          data-testid="library-activity-trigger"
        >
          <Activity className="size-5" />
          {activeCount > 0 && (
            <span
              className="absolute -right-1 -top-1 flex min-w-5 items-center justify-center rounded-full bg-[var(--plum-accent)] px-1.5 py-0.5 text-[10px] font-semibold text-white"
              data-testid="library-activity-badge"
            >
              {activeCount}
            </span>
          )}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="w-[26rem] max-w-[calc(100vw-1rem)] space-y-3 p-3 max-h-[70vh] overflow-y-auto"
      >
        <div className="space-y-1">
          <div className="text-sm font-semibold text-[var(--plum-text)]">Library Activity</div>
          <div className="text-xs text-[var(--plum-muted)]">
            Live import, finishing, and identify work across your libraries.
          </div>
        </div>

        {visibleStatuses.length === 0 ? (
          <div className="rounded-[var(--radius-md)] border border-dashed border-[var(--plum-border)] px-3 py-5 text-sm text-[var(--plum-muted)]">
            No active library activity.
          </div>
        ) : (
          <div className="space-y-3">
            {visibleStatuses.map(({ library, status }) => (
              <ActivityStatusCard key={library.id} library={library} status={status} />
            ))}
          </div>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
