import {
  createContext,
  useCallback,
  useContext,
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
  type LibraryScanStatus,
} from "../api";
import { queryKeys, useLibraries } from "../queries";
import { useWs } from "./WsContext";

type QueueScanOptions = {
  identify?: boolean;
  subpath?: string;
};

type ScanQueueContextValue = {
  scanStatuses: Record<number, LibraryScanStatus>;
  activeLibraryIds: number[];
  activityScanStatuses: LibraryScanStatus[];
  getLibraryScanStatus: (libraryId: number | null) => LibraryScanStatus | undefined;
  hasLibraryScanStatus: (libraryId: number | null) => boolean;
  queueLibraryScan: (libraryId: number, options?: QueueScanOptions) => Promise<LibraryScanStatus>;
};

const SCAN_POLL_INTERVAL_MS = 2_000;

const ScanQueueContext = createContext<ScanQueueContextValue | null>(null);

function isActiveScan(phase?: string) {
  return phase === "queued" || phase === "scanning";
}

function isLibraryProcessing(status?: LibraryScanStatus) {
  return (
    status != null &&
    (isActiveScan(status.phase) ||
      status.enriching ||
      status.identifyPhase === "queued" ||
      status.identifyPhase === "identifying")
  );
}

function hasActivityDetails(status?: LibraryScanStatus) {
  return status?.activity != null;
}

function shouldShowInActivityCenter(status?: LibraryScanStatus) {
  return (
    status != null &&
    (isLibraryProcessing(status) || status.phase === "failed" || status.identifyPhase === "failed")
  );
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

function hasMeaningfulStatusChange(previous: LibraryScanStatus | undefined, next: LibraryScanStatus) {
  if (previous == null) return true;
  return (
    previous.phase !== next.phase ||
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
  const { data: libraries = [] } = useLibraries();
  const { latestEvent, eventSequence } = useWs();
  const [scanStatuses, setScanStatuses] = useState<Record<number, LibraryScanStatus>>({});
  const scanStatusesRef = useRef<Record<number, LibraryScanStatus>>({});

  const setScanStatus = useCallback((status: LibraryScanStatus) => {
    setScanStatuses((current) => {
      const previous = current[status.libraryId];
      if (sameScanStatus(previous, status)) {
        return current;
      }
      const next = { ...current, [status.libraryId]: status };
      scanStatusesRef.current = next;
      return next;
    });
  }, []);

  const refreshLibraryScanStatus = useCallback(
    async (libraryId: number) => {
      const status = await fetchLibraryScanStatus(libraryId);
      setScanStatus(status);
      if (isLibraryProcessing(status) || status.phase === "completed" || status.phase === "failed") {
        void queryClient.invalidateQueries({ queryKey: queryKeys.library(libraryId) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.libraries });
        void queryClient.invalidateQueries({ queryKey: queryKeys.home });
      }
      return status;
    },
    [queryClient, setScanStatus],
  );

  const queueLibraryScan = useCallback(
    async (libraryId: number, options?: QueueScanOptions) => {
      const status = await startLibraryScan(libraryId, options);
      setScanStatus(status);
      void queryClient.invalidateQueries({ queryKey: queryKeys.library(libraryId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.libraries });
      void queryClient.invalidateQueries({ queryKey: queryKeys.home });
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

    if (libraries.length === 0) return;
    void Promise.allSettled(libraries.map((library) => refreshLibraryScanStatus(library.id)));
  }, [libraries, refreshLibraryScanStatus]);

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
      void queryClient.invalidateQueries({ queryKey: queryKeys.library(nextStatus.libraryId) });
      void queryClient.invalidateQueries({ queryKey: queryKeys.libraries });
      void queryClient.invalidateQueries({ queryKey: queryKeys.home });
    }
  }, [eventSequence, latestEvent, queryClient, setScanStatus]);

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
      Object.values(scanStatuses).filter((status) =>
        shouldShowInActivityCenter(status) || hasActivityDetails(status),
      ),
    [scanStatuses],
  );

  const value = useMemo<ScanQueueContextValue>(
    () => ({
      scanStatuses,
      activeLibraryIds,
      activityScanStatuses,
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
      scanStatuses,
    ],
  );

  return <ScanQueueContext.Provider value={value}>{children}</ScanQueueContext.Provider>;
}

export function useScanQueue() {
  const ctx = useContext(ScanQueueContext);
  if (!ctx) throw new Error("useScanQueue must be used within ScanQueueProvider");
  return ctx;
}
