import { useMemo } from "react";
import { Activity } from "lucide-react";
import { type Library, type LibraryScanActivityEntry, type LibraryScanStatus } from "@/api";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { useScanQueue } from "@/contexts/ScanQueueContext";
import { getLibraryTabLabel } from "@/lib/showGrouping";
import { cn } from "@/lib/utils";
import { useLibraries } from "@/queries";

type ActivityLibraryStatus = {
  library: Library;
  status: LibraryScanStatus;
};

type ActivitySummary = {
  label: string;
  detail: string;
  secondaryDetail: string;
};

function isActiveStatus(status: LibraryScanStatus) {
  return (
    status.phase === "queued" ||
    status.phase === "scanning" ||
    status.enriching ||
    status.identifyPhase === "queued" ||
    status.identifyPhase === "identifying"
  );
}

function getStatusSortOrder(status: LibraryScanStatus) {
  if (status.phase === "scanning") return 0;
  if (status.phase === "queued") return 1;
  if (status.enriching) return 2;
  if (status.identifyPhase === "queued") return 3;
  if (status.identifyPhase === "identifying") return 4;
  return 5;
}

function formatActivityPath(entry?: LibraryScanActivityEntry | null) {
  if (!entry) return "";
  const basePath = entry.relativePath.trim();
  if (entry.target === "library") {
    return basePath;
  }
  if (entry.target === "directory") {
    return basePath ? `${basePath}/` : "Library root/";
  }
  return basePath || "Library root";
}

function getActivityDetail(entry?: LibraryScanActivityEntry | null) {
  return entry?.detail?.trim() ?? "";
}

function getNowSummary(status: LibraryScanStatus, library: Library): ActivitySummary {
  const currentDetail = getActivityDetail(status.activity?.current);
  const currentPath = formatActivityPath(status.activity?.current);

  if (status.identifyPhase === "identifying") {
    return {
      label: "Identifying",
      detail: currentDetail || (status.identified > 0 ? `Identified ${status.identified} so far` : "Matching metadata"),
      secondaryDetail: currentPath,
    };
  }
  if (status.identifyPhase === "queued") {
    return {
      label: "Waiting for identify worker",
      detail: currentDetail || (status.queuePosition > 0 ? `Queue position ${status.queuePosition}` : "Queued for identify"),
      secondaryDetail: currentPath,
    };
  }
  if (status.enriching) {
    return {
      label: "Analyzing media",
      detail:
        currentDetail ||
        (library.type === "music"
          ? "Reading music metadata"
          : "Collecting playback and stream details"),
      secondaryDetail: currentPath,
    };
  }
  if (status.phase === "scanning") {
    return {
      label: "Importing",
      detail: currentPath || `Processed ${status.processed}`,
      secondaryDetail: "",
    };
  }
  if (status.phase === "queued") {
    return {
      label: "Queued",
      detail: status.queuePosition > 0 ? `Queue position ${status.queuePosition}` : "Waiting to start",
      secondaryDetail: "",
    };
  }
  return {
    label: "Done",
    detail: "",
    secondaryDetail: "",
  };
}

function ActivityStatusCard({ library, status }: ActivityLibraryStatus) {
  const summary = getNowSummary(status, library);

  return (
    <section
      className="space-y-2 rounded-[var(--radius-md)] border border-[var(--nebula-border)] bg-[var(--nebula-panel)]/90 p-3"
      data-testid={`library-activity-status-${library.id}`}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-[var(--nebula-text)]">
            {getLibraryTabLabel(library)}
          </div>
          {summary.detail ? (
            <div className="mt-1 text-xs text-[var(--nebula-muted)]">{summary.detail}</div>
          ) : null}
          {summary.secondaryDetail ? (
            <div className="mt-1 truncate text-xs text-[var(--nebula-muted)]/80">{summary.secondaryDetail}</div>
          ) : null}
        </div>
        <div className="flex shrink-0 flex-col items-end gap-2">
          <span
            className="rounded-full bg-[var(--nebula-accent-soft)] px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--nebula-accent)]"
          >
            {summary.label}
          </span>
        </div>
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
            "relative",
            hasUpdates &&
              "bg-[var(--nebula-accent-soft)] text-[var(--nebula-accent)] hover:bg-[var(--nebula-accent-soft)]/80 hover:text-[var(--nebula-accent)]",
          )}
          data-testid="library-activity-trigger"
        >
          <Activity className="size-5" />
          {activeCount > 0 && (
            <span
              className="absolute -right-1 -top-1 flex min-w-5 items-center justify-center rounded-full bg-[var(--nebula-accent)] px-1.5 py-0.5 text-[10px] font-semibold text-white"
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
          <div className="text-sm font-semibold text-[var(--nebula-text)]">Server activity</div>
          <div className="text-xs text-[var(--nebula-muted)]">
            What Plum is doing now, and what just finished.
          </div>
        </div>

        {nowStatuses.length > 0 ? (
          <div className="space-y-2">
            <div className="text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--nebula-muted)]">
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
            <div className="text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--nebula-muted)]">
              Just finished
            </div>
            <div className="space-y-2">
              {recentItems.map(({ activity, library }) => (
                <section
                  key={`${activity.libraryId}-${activity.finishedAt}-${activity.status}`}
                  className="flex items-start justify-between gap-3 rounded-[var(--radius-md)] border border-[var(--nebula-border)] bg-[var(--nebula-panel)]/70 p-3"
                >
                  <div className="min-w-0">
                    <div className="truncate text-sm font-semibold text-[var(--nebula-text)]">
                      {getLibraryTabLabel(library)}
                    </div>
                    {activity.detail ? (
                      <div className="mt-1 text-xs text-[var(--nebula-muted)]">{activity.detail}</div>
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
          <div className="rounded-[var(--radius-md)] border border-dashed border-[var(--nebula-border)] px-3 py-5 text-sm text-[var(--nebula-muted)]">
            Nothing is happening right now.
          </div>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
