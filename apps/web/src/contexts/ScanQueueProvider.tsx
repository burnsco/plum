import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  getLibraryScanStatus as fetchLibraryScanStatus,
  startLibraryScan,
  type Library,
  type LibraryScanStatus,
} from "../api";
import { getEnrichmentPhase, isLibraryScanProcessing } from "../lib/libraryActivity";
import {
  invalidateDiscoverRelatedQueries,
  invalidateLibraryCatalogQueries,
  invalidateSearchAfterLibraryDataChange,
  useLibraries,
} from "../queries";
import { ScanQueueContext, type QueueScanOptions, type RecentLibraryActivity } from "./ScanQueueContext";
import { useWsEvent } from "./WsContext";

const SCAN_POLL_INTERVAL_MS = 2_000;
const JUST_FINISHED_TTL_MS = 5 * 60 * 1000;
const JUST_FINISHED_MAX_ITEMS = 5;

/** Stable fallback so `useLibraries` loading state does not pass a fresh `[]` every render. */
const EMPTY_LIBRARIES: Library[] = [];

function isLibraryProcessing(status?: LibraryScanStatus) {
  return status != null && isLibraryScanProcessing(status);
}

function hasActivityDetails(status?: LibraryScanStatus) {
  return status?.activity != null;
}

function sameActivityEntry(
  left?: NonNullable<LibraryScanStatus["activity"]>["current"],
  right?: NonNullable<LibraryScanStatus["activity"]>["current"],
) {
  if (left == null && right == null) return true;
  if (left == null || right == null) return false;
  return (
    left.phase === right.phase &&
    left.target === right.target &&
    left.relativePath === right.relativePath &&
    left.at === right.at
  );
}

function sameActivity(statusA?: LibraryScanStatus["activity"], statusB?: LibraryScanStatus["activity"]) {
  if (statusA == null && statusB == null) return true;
  if (statusA == null || statusB == null) return false;
  if (statusA.stage !== statusB.stage || !sameActivityEntry(statusA.current, statusB.current)) {
    return false;
  }
  if (statusA.recent.length !== statusB.recent.length) return false;
  return statusA.recent.every((entry, index) => sameActivityEntry(entry, statusB.recent[index]));
}

function sameScanStatus(previous?: LibraryScanStatus, next?: LibraryScanStatus) {
  if (previous == null || next == null) return false;
  return (
    previous.phase === next.phase &&
    previous.enrichmentPhase === next.enrichmentPhase &&
    previous.enriching === next.enriching &&
    previous.identifyPhase === next.identifyPhase &&
    previous.identified === next.identified &&
    previous.identifyFailed === next.identifyFailed &&
    previous.processed === next.processed &&
    previous.added === next.added &&
    previous.updated === next.updated &&
    previous.removed === next.removed &&
    previous.unmatched === next.unmatched &&
    previous.skipped === next.skipped &&
    previous.identifyRequested === next.identifyRequested &&
    previous.queuedAt === next.queuedAt &&
    previous.estimatedItems === next.estimatedItems &&
    previous.queuePosition === next.queuePosition &&
    previous.error === next.error &&
    previous.retryCount === next.retryCount &&
    previous.maxRetries === next.maxRetries &&
    previous.nextRetryAt === next.nextRetryAt &&
    previous.lastError === next.lastError &&
    previous.nextScheduledAt === next.nextScheduledAt &&
    previous.startedAt === next.startedAt &&
    previous.finishedAt === next.finishedAt &&
    sameActivity(previous.activity, next.activity)
  );
}

function isSuccessfulLibraryCompletion(status?: LibraryScanStatus) {
  return (
    status != null &&
    status.phase === "completed" &&
    !status.enriching &&
    getEnrichmentPhase(status) === "idle" &&
    status.identifyPhase !== "queued" &&
    status.identifyPhase !== "identifying" &&
    status.identifyPhase !== "failed"
  );
}

function buildRecentLibraryActivity(
  previous: LibraryScanStatus | undefined,
  next: LibraryScanStatus,
): RecentLibraryActivity | null {
  if (previous == null || !isLibraryProcessing(previous) || isLibraryProcessing(next)) {
    return null;
  }
  if (next.phase === "failed" || next.identifyPhase === "failed") {
    return {
      libraryId: next.libraryId,
      status: "failed",
      summary: "Failed",
      detail: next.lastError || next.error || undefined,
      finishedAt: next.finishedAt || new Date().toISOString(),
    };
  }
  if (isSuccessfulLibraryCompletion(next)) {
    const detail =
      next.identifyRequested && next.identified > 0
        ? `Identified ${next.identified} item${next.identified === 1 ? "" : "s"}`
        : next.processed > 0
          ? `Processed ${next.processed} item${next.processed === 1 ? "" : "s"}`
          : undefined;
    return {
      libraryId: next.libraryId,
      status: "done",
      summary: "Done",
      detail,
      finishedAt: next.finishedAt || new Date().toISOString(),
    };
  }
  return null;
}

function pruneRecentLibraryActivities(
  activities: RecentLibraryActivity[],
  validLibraryIds?: Set<number>,
) {
  const now = Date.now();
  return activities
    .filter((activity) => {
      if (validLibraryIds && !validLibraryIds.has(activity.libraryId)) {
        return false;
      }
      const finishedAt = Date.parse(activity.finishedAt);
      return Number.isFinite(finishedAt) && now - finishedAt <= JUST_FINISHED_TTL_MS;
    })
    .slice(0, JUST_FINISHED_MAX_ITEMS);
}

function hasMeaningfulStatusChange(previous: LibraryScanStatus | undefined, next: LibraryScanStatus) {
  if (previous == null) return true;
  return (
    previous.phase !== next.phase ||
    previous.enrichmentPhase !== next.enrichmentPhase ||
    previous.enriching !== next.enriching ||
    previous.identifyPhase !== next.identifyPhase ||
    previous.identified !== next.identified ||
    previous.identifyFailed !== next.identifyFailed ||
    previous.processed !== next.processed ||
    previous.added !== next.added ||
    previous.updated !== next.updated ||
    previous.removed !== next.removed ||
    previous.unmatched !== next.unmatched ||
    previous.skipped !== next.skipped ||
    previous.identifyRequested !== next.identifyRequested ||
    previous.error !== next.error ||
    previous.lastError !== next.lastError ||
    previous.finishedAt !== next.finishedAt
  );
}

export function ScanQueueProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const { data: librariesData } = useLibraries();
  const libraries = librariesData ?? EMPTY_LIBRARIES;
  const { latestEvent, eventSequence } = useWsEvent();
  const [scanStatuses, setScanStatuses] = useState<Record<number, LibraryScanStatus>>({});
  const [recentLibraryActivities, setRecentLibraryActivities] = useState<RecentLibraryActivity[]>([]);
  const scanStatusesRef = useRef<Record<number, LibraryScanStatus>>({});

  const setScanStatus = useCallback((status: LibraryScanStatus) => {
    const previous = scanStatusesRef.current[status.libraryId];
    if (sameScanStatus(previous, status)) {
      return;
    }
    const next = { ...scanStatusesRef.current, [status.libraryId]: status };
    scanStatusesRef.current = next;
    setScanStatuses(next);
    const recentActivity = buildRecentLibraryActivity(previous, status);
    if (recentActivity == null) {
      return;
    }
    setRecentLibraryActivities((current) =>
      pruneRecentLibraryActivities([
        recentActivity,
        ...current.filter(
          (activity) =>
            !(
              activity.libraryId === recentActivity.libraryId &&
              activity.finishedAt === recentActivity.finishedAt &&
              activity.status === recentActivity.status
            ),
        ),
      ]),
    );
  }, []);

  const refreshLibraryScanStatus = useCallback(
    async (libraryId: number) => {
      const previous = scanStatusesRef.current[libraryId];
      const status = await fetchLibraryScanStatus(libraryId);
      setScanStatus(status);
      if (isLibraryProcessing(status) || status.phase === "completed" || status.phase === "failed") {
        invalidateLibraryCatalogQueries(queryClient, libraryId);
      }
      const wasTerminal = previous?.phase === "completed" || previous?.phase === "failed";
      const isTerminal = status.phase === "completed" || status.phase === "failed";
      if (isTerminal && !wasTerminal) {
        invalidateDiscoverRelatedQueries(queryClient);
      }
      return status;
    },
    [queryClient, setScanStatus],
  );

  const queueLibraryScan = useCallback(
    async (libraryId: number, options?: QueueScanOptions) => {
      const status = await startLibraryScan(libraryId, options);
      setScanStatus(status);
      invalidateLibraryCatalogQueries(queryClient, libraryId);
      return status;
    },
    [queryClient, setScanStatus],
  );

  const getLibraryScanStatus = useCallback(
    (libraryId: number | null) => (libraryId == null ? undefined : scanStatuses[libraryId]),
    [scanStatuses],
  );

  const hasLibraryScanStatus = useCallback(
    (libraryId: number | null) => libraryId != null && libraryId in scanStatuses,
    [scanStatuses],
  );

  useEffect(() => {
    scanStatusesRef.current = scanStatuses;
  }, [scanStatuses]);

  useEffect(() => {
    const activeLibraryIds = new Set(libraries.map((library) => library.id));
    setScanStatuses((current) => {
      const nextEntries = Object.entries(current).filter(([libraryId]) =>
        activeLibraryIds.has(parseInt(libraryId, 10)),
      );
      if (nextEntries.length === Object.keys(current).length) {
        scanStatusesRef.current = current;
        return current;
      }
      const next = Object.fromEntries(nextEntries);
      scanStatusesRef.current = next;
      return next;
    });
    setRecentLibraryActivities((current) => pruneRecentLibraryActivities(current, activeLibraryIds));

    if (libraries.length === 0) return;
    void Promise.allSettled(libraries.map((library) => refreshLibraryScanStatus(library.id)));
  }, [libraries, refreshLibraryScanStatus]);

  useEffect(() => {
    const intervalId = window.setInterval(() => {
      setRecentLibraryActivities((current) => pruneRecentLibraryActivities(current));
    }, 60_000);
    return () => window.clearInterval(intervalId);
  }, []);

  useEffect(() => {
    const activeScanIds = Object.values(scanStatuses)
      .filter((status) => isLibraryProcessing(status))
      .map((status) => status.libraryId);
    if (activeScanIds.length === 0) return;

    const intervalId = window.setInterval(() => {
      void Promise.allSettled(activeScanIds.map((libraryId) => refreshLibraryScanStatus(libraryId)));
    }, SCAN_POLL_INTERVAL_MS);
    return () => window.clearInterval(intervalId);
  }, [refreshLibraryScanStatus, scanStatuses]);

  useEffect(() => {
    if (!latestEvent || latestEvent.type !== "library_scan_update") return;

    const nextStatus = latestEvent.scan;
    const previous = scanStatusesRef.current[nextStatus.libraryId];
    setScanStatus(nextStatus);

    if (
      hasMeaningfulStatusChange(previous, nextStatus) &&
      (isLibraryProcessing(nextStatus) ||
        nextStatus.phase === "completed" ||
        nextStatus.phase === "failed")
    ) {
      invalidateLibraryCatalogQueries(queryClient, nextStatus.libraryId);
    }
    if (
      hasMeaningfulStatusChange(previous, nextStatus) &&
      (nextStatus.phase === "completed" || nextStatus.phase === "failed")
    ) {
      invalidateDiscoverRelatedQueries(queryClient);
    }
  }, [eventSequence, latestEvent, queryClient, setScanStatus]);

  useEffect(() => {
    if (!latestEvent || latestEvent.type !== "library_catalog_changed") return;
    invalidateLibraryCatalogQueries(queryClient, latestEvent.libraryId);
    invalidateSearchAfterLibraryDataChange(queryClient, latestEvent.libraryId);
  }, [eventSequence, latestEvent, queryClient]);

  const activeLibraryIds = useMemo(
    () =>
      Object.values(scanStatuses)
        .filter((status) => isLibraryProcessing(status))
        .map((status) => status.libraryId)
        .sort((left, right) => left - right),
    [scanStatuses],
  );

  const activityScanStatuses = useMemo(
    () =>
      Object.values(scanStatuses).filter((status) => isLibraryProcessing(status) || hasActivityDetails(status)),
    [scanStatuses],
  );

  const visibleRecentLibraryActivities = useMemo(
    () => pruneRecentLibraryActivities(recentLibraryActivities),
    [recentLibraryActivities],
  );

  const value = useMemo(
    () => ({
      scanStatuses,
      activeLibraryIds,
      activityScanStatuses,
      recentLibraryActivities: visibleRecentLibraryActivities,
      getLibraryScanStatus,
      hasLibraryScanStatus,
      queueLibraryScan,
    }),
    [
      activeLibraryIds,
      activityScanStatuses,
      getLibraryScanStatus,
      hasLibraryScanStatus,
      queueLibraryScan,
      visibleRecentLibraryActivities,
      scanStatuses,
    ],
  );

  return <ScanQueueContext.Provider value={value}>{children}</ScanQueueContext.Provider>;
}

