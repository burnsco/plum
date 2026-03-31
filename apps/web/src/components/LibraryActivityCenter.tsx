import { useMemo } from "react";
import { Activity } from "lucide-react";
import { type Library, type LibraryScanActivityEntry, type LibraryScanStatus } from "@/api";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { useScanQueue } from "@/contexts/ScanQueueContext";
import { getEnrichmentPhase } from "@/lib/libraryActivity";
import { getLibraryTabLabel } from "@/lib/showGrouping";
import { cn } from "@/lib/utils";
import { useLibraries } from "@/queries";

type ActivityLibraryStatus = {
  library: Library;
  status: LibraryScanStatus;
};

function isActiveStatus(status: LibraryScanStatus) {
  const enrichmentPhase = getEnrichmentPhase(status);
  return (
    status.phase === "queued" ||
    status.phase === "scanning" ||
    enrichmentPhase === "queued" ||
    enrichmentPhase === "running" ||
    status.identifyPhase === "queued" ||
    status.identifyPhase === "identifying"
  );
}

function getStatusSortOrder(status: LibraryScanStatus) {
  if (status.phase === "scanning") return 0;
  if (status.phase === "queued") return 1;
  if (getEnrichmentPhase(status) === "running") return 2;
  if (getEnrichmentPhase(status) === "queued") return 3;
  if (status.identifyPhase === "queued") return 4;
  if (status.identifyPhase === "identifying") return 5;
  return 6;
}

function formatActivityPath(entry?: LibraryScanActivityEntry | null) {
  if (!entry) return "";
  const basePath = entry.relativePath.trim();
  if (entry.target === "directory") {
    return basePath ? `${basePath}/` : "Library root/";
  }
  return basePath || "Library root";
}

function getNowSummary(status: LibraryScanStatus) {
  const enrichmentPhase = getEnrichmentPhase(status);
  if (status.identifyPhase === "identifying") {
    return {
      label: "Identifying",
      detail:
        formatActivityPath(status.activity?.current) ||
        (status.identified > 0 ? `Identified ${status.identified} so far` : ""),
    };
  }
  if (status.identifyPhase === "queued") {
    return {
      label: "Waiting for identify worker",
      detail: status.queuePosition > 0 ? `Queue position ${status.queuePosition}` : "",
    };
  }
  if (enrichmentPhase === "running") {
    return {
      label: "Analyzing media",
      detail: formatActivityPath(status.activity?.current),
    };
  }
  if (enrichmentPhase === "queued") {
    return {
      label: "Waiting for analyzer",
      detail: status.queuePosition > 0 ? `Queue position ${status.queuePosition}` : "",
    };
  }
  if (status.phase === "scanning") {
    return {
      label: "Importing",
      detail: formatActivityPath(status.activity?.current) || `Processed ${status.processed}`,
    };
  }
  if (status.phase === "queued") {
    return {
      label: "Queued",
      detail: status.queuePosition > 0 ? `Queue position ${status.queuePosition}` : "",
    };
  }
  return { label: "Done", detail: "" };
}

function ActivityStatusCard({ library, status }: ActivityLibraryStatus) {
  const summary = getNowSummary(status);

  return (
    <section
      className="space-y-2 rounded-[var(--radius-md)] border border-[var(--plum-border)] bg-[var(--plum-panel)]/90 p-3"
      data-testid={`library-activity-status-${library.id}`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-[var(--plum-text)]">
            {getLibraryTabLabel(library)}
          </div>
          {summary.detail ? (
            <div className="mt-1 text-xs text-[var(--plum-muted)]">{summary.detail}</div>
          ) : null}
        </div>
        <span className="rounded-full bg-[var(--plum-accent-soft)] px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--plum-accent)]">
          {summary.label}
        </span>
      </div>
    </section>
  );
}

export function LibraryActivityCenter() {
  const { data: libraries = [] } = useLibraries();
  const { activeLibraryIds, activityScanStatuses, recentLibraryActivities } = useScanQueue();

  const libraryById = useMemo(
    () => new Map(libraries.map((library) => [library.id, library])),
    [libraries],
  );

  const nowStatuses = useMemo(() => {
    return activityScanStatuses
      .filter(isActiveStatus)
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
  }, [activityScanStatuses, libraryById]);

  const recentItems = useMemo(
    () =>
      recentLibraryActivities
        .map((activity) => {
          const library = libraryById.get(activity.libraryId);
          return library ? { activity, library } : null;
        })
        .filter((value): value is { activity: (typeof recentLibraryActivities)[number]; library: Library } => value != null),
    [libraryById, recentLibraryActivities],
  );

  const activeCount = activeLibraryIds.length;
  const hasUpdates = activeCount > 0 || recentItems.length > 0;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="icon"
          size="icon"
          aria-label="Server activity"
          className={cn(
            "relative transition-all duration-500",
            hasUpdates &&
              "bg-[var(--plum-accent-soft)] text-[var(--plum-accent)] hover:bg-[var(--plum-accent-soft)]/80 hover:text-[var(--plum-accent)]",
            activeCount > 0 && "animate-pulse shadow-[0_0_15px_var(--plum-accent-glow)] ring-1 ring-[var(--plum-accent-soft)]",
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
        className="w-[23rem] max-w-[calc(100vw-1rem)] space-y-4 p-3 max-h-[70vh] overflow-y-auto"
      >
        <div className="space-y-1">
          <div className="text-sm font-semibold text-[var(--plum-text)]">Server activity</div>
          <div className="text-xs text-[var(--plum-muted)]">
            What Plum is doing now, and what just finished.
          </div>
        </div>

        {nowStatuses.length > 0 ? (
          <div className="space-y-2">
            <div className="text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--plum-muted)]">
              Now
            </div>
            <div className="space-y-2">
              {nowStatuses.map(({ library, status }) => (
                <ActivityStatusCard key={library.id} library={library} status={status} />
              ))}
            </div>
          </div>
        ) : null}

        {recentItems.length > 0 ? (
          <div className="space-y-2">
            <div className="text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--plum-muted)]">
              Just finished
            </div>
            <div className="space-y-2">
              {recentItems.map(({ activity, library }) => (
                <section
                  key={`${activity.libraryId}-${activity.finishedAt}-${activity.status}`}
                  className="flex items-start justify-between gap-3 rounded-[var(--radius-md)] border border-[var(--plum-border)] bg-[var(--plum-panel)]/70 p-3"
                >
                  <div className="min-w-0">
                    <div className="truncate text-sm font-semibold text-[var(--plum-text)]">
                      {getLibraryTabLabel(library)}
                    </div>
                    {activity.detail ? (
                      <div className="mt-1 text-xs text-[var(--plum-muted)]">{activity.detail}</div>
                    ) : null}
                  </div>
                  <span
                    className={cn(
                      "rounded-full px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.12em]",
                      activity.status === "failed"
                        ? "bg-rose-500/15 text-rose-300"
                        : "bg-emerald-500/15 text-emerald-300",
                    )}
                  >
                    {activity.summary}
                  </span>
                </section>
              ))}
            </div>
          </div>
        ) : null}

        {nowStatuses.length === 0 && recentItems.length === 0 ? (
          <div className="rounded-[var(--radius-md)] border border-dashed border-[var(--plum-border)] px-3 py-5 text-sm text-[var(--plum-muted)]">
            Nothing is happening right now.
          </div>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
