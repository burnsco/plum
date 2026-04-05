import { useMemo } from "react";
import { Activity } from "lucide-react";
import { type Library, type LibraryScanActivityEntry, type LibraryScanStatus } from "@/api";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { useScanQueue } from "@/contexts/ScanQueueContext";
import { getEnrichmentPhase, isLibraryScanProcessing } from "@/lib/libraryActivity";
import { getLibraryTabLabel } from "@/lib/showGrouping";
import { cn } from "@/lib/utils";
import { useLibraries } from "@/queries";

type ActivityLibraryStatus = {
  library: Library;
  status: LibraryScanStatus;
};

const STALE_ACTIVITY_DETAIL_MS = 15_000;

function getActivityAgeMs(entry?: LibraryScanActivityEntry | null) {
  if (!entry?.at) return Number.POSITIVE_INFINITY;
  const parsed = Date.parse(entry.at);
  if (!Number.isFinite(parsed)) return Number.POSITIVE_INFINITY;
  return Date.now() - parsed;
}

function hasFreshActivityDetail(entry?: LibraryScanActivityEntry | null) {
  return getActivityAgeMs(entry) <= STALE_ACTIVITY_DETAIL_MS;
}

function activityPhaseLabel(phase: LibraryScanActivityEntry["phase"]): string {
  switch (phase) {
    case "discovery":
      return "Importing";
    case "enrichment":
      return "Analyzing media";
    case "identify":
      return "Identifying";
    default:
      return "";
  }
}

function pickDisplayActivityEntry(status: LibraryScanStatus): LibraryScanActivityEntry | undefined {
  const act = status.activity;
  if (!act) return undefined;
  if (act.current && hasFreshActivityDetail(act.current)) return act.current;
  for (const entry of act.recent) {
    if (hasFreshActivityDetail(entry)) return entry;
  }
  return undefined;
}

function formatActivityPath(entry?: LibraryScanActivityEntry | null) {
  if (!entry) return "";
  const basePath = entry.relativePath.trim();
  if (entry.target === "directory") {
    return basePath ? `${basePath}/` : "Library root/";
  }
  return basePath || "Library root";
}

function getStatusSortOrder(status: LibraryScanStatus) {
  if (status.phase === "scanning") return 0;
  if (getEnrichmentPhase(status) === "running") return 1;
  if (status.identifyPhase === "identifying") return 2;
  if (status.phase === "queued") return 3;
  if (getEnrichmentPhase(status) === "queued") return 4;
  if (status.identifyPhase === "queued") return 5;
  return 6;
}

function getNowSummary(status: LibraryScanStatus): { label: string; details: string[] } {
  const enrichmentPhase = getEnrichmentPhase(status);
  const displayEntry = pickDisplayActivityEntry(status);
  const entryLine =
    displayEntry != null
      ? `${activityPhaseLabel(displayEntry.phase)}: ${formatActivityPath(displayEntry)}`
      : null;

  if (status.identifyPhase === "identifying") {
    const details: string[] = [];
    if (entryLine) details.push(entryLine);
    if (status.identified > 0) {
      details.push(
        `Identified ${status.identified} item${status.identified === 1 ? "" : "s"} so far`,
      );
    } else if (status.unmatched > 0) {
      details.push(
        `${status.unmatched} item${status.unmatched === 1 ? "" : "s"} still need metadata`,
      );
    }
    return { label: "Identifying", details };
  }

  if (enrichmentPhase === "running") {
    const details: string[] = [];
    if (entryLine) details.push(entryLine);
    if (status.processed > 0) {
      details.push(`Processed ${status.processed} item${status.processed === 1 ? "" : "s"}`);
    }
    return { label: "Analyzing media", details };
  }

  if (status.phase === "scanning") {
    const details: string[] = [];
    if (entryLine) details.push(entryLine);
    if (status.estimatedItems > 0) {
      details.push(`Progress: ${status.processed} / ~${status.estimatedItems} items`);
    } else {
      details.push(`Processed ${status.processed} item${status.processed === 1 ? "" : "s"}`);
    }
    return { label: "Importing", details };
  }

  if (status.phase === "queued") {
    const details = ["Waiting for the scanner to start this import."];
    if (status.queuePosition > 0) {
      details.push(`Queue position: ${status.queuePosition}`);
    }
    return { label: "Import queued", details };
  }

  if (enrichmentPhase === "queued") {
    const details = ["Analysis will run after import completes."];
    if (status.processed > 0) {
      details.push(`${status.processed} items ready for analysis`);
    }
    return { label: "Analyze queued", details };
  }

  if (status.identifyPhase === "queued") {
    const details = ["Metadata matching will start when analysis is ready."];
    if (status.processed > 0) {
      details.push(`${status.processed} items in this library`);
    }
    return { label: "Identify queued", details };
  }

  return { label: "Idle", details: [] };
}

function ActivityStatusCard({ library, status }: ActivityLibraryStatus) {
  const summary = getNowSummary(status);

  return (
    <section
      className="space-y-2 rounded-md border border-(--plum-border) bg-(--plum-panel)/90 p-3"
      data-testid={`library-activity-status-${library.id}`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-(--plum-text)">
            {getLibraryTabLabel(library)}
          </div>
          {summary.details.length > 0 ? (
            <p className="mt-1.5 whitespace-pre-line text-xs text-(--plum-muted)">
              {summary.details.join("\n")}
            </p>
          ) : null}
        </div>
        <span className="rounded-full bg-(--plum-accent-soft) px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-(--plum-accent)">
          {summary.label}
        </span>
      </div>
    </section>
  );
}

export function LibraryActivityCenter() {
  const { data: libraries = [] } = useLibraries();
  const { activityScanStatuses, recentLibraryActivities } = useScanQueue();

  const libraryById = useMemo(
    () => new Map(libraries.map((library) => [library.id, library])),
    [libraries],
  );

  const nowStatuses = useMemo(() => {
    return activityScanStatuses
      .filter(isLibraryScanProcessing)
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
        .filter(
          (value): value is { activity: (typeof recentLibraryActivities)[number]; library: Library } =>
            value != null,
        ),
    [libraryById, recentLibraryActivities],
  );

  const activeCount = nowStatuses.length;
  const hasRecentOnly = activeCount === 0 && recentItems.length > 0;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="icon"
          size="icon"
          aria-label={
            activeCount > 0 ? `Server activity ${activeCount} active` : "Server activity"
          }
          className={cn(
            "relative transition-all duration-500",
            activeCount > 0 &&
              "bg-(--plum-accent-soft) text-(--plum-accent) hover:bg-(--plum-accent-soft)/80 hover:text-(--plum-accent)",
            activeCount > 0 &&
              "animate-pulse shadow-[0_0_15px_var(--plum-accent-glow)] ring-1 ring-(--plum-accent-soft)",
            hasRecentOnly && "text-(--plum-text-2) hover:text-(--plum-text)",
          )}
          data-testid="library-activity-trigger"
        >
          <Activity className="size-5" />
          {activeCount > 0 && (
            <span
              className="absolute -right-1 -top-1 flex min-w-5 items-center justify-center rounded-full bg-(--plum-accent) px-1.5 py-0.5 text-[10px] font-semibold text-white"
              data-testid="library-activity-badge"
              aria-hidden="true"
            >
              {activeCount}
            </span>
          )}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="end"
        className="w-92 max-w-[calc(100vw-1rem)] space-y-4 p-3 max-h-[70vh] overflow-y-auto"
      >
        <div className="text-sm font-semibold text-(--plum-text)">Server activity</div>

        {nowStatuses.length > 0 ? (
          <div className="space-y-2">
            <div className="space-y-2">
              {nowStatuses.map(({ library, status }) => (
                <ActivityStatusCard key={library.id} library={library} status={status} />
              ))}
            </div>
          </div>
        ) : null}

        {recentItems.length > 0 ? (
          <div className="space-y-2">
            <div className="space-y-2">
              {recentItems.map(({ activity, library }) => (
                <section
                  key={`${activity.libraryId}-${activity.finishedAt}-${activity.status}`}
                  className="flex items-start justify-between gap-3 rounded-md border border-(--plum-border) bg-(--plum-panel)/70 p-3"
                >
                  <div className="min-w-0">
                    <div className="truncate text-sm font-semibold text-(--plum-text)">
                      {getLibraryTabLabel(library)}
                    </div>
                    {activity.detail ? (
                      <div className="mt-1 text-xs text-(--plum-muted)">{activity.detail}</div>
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
          <div className="rounded-md border border-dashed border-(--plum-border) px-3 py-5 text-sm text-(--plum-muted)">
            Nothing is happening right now.
          </div>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
