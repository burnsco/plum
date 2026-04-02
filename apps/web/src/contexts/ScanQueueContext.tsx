import { createContext, useContext } from "react";
import type { LibraryScanStatus } from "../api";

export type QueueScanOptions = {
  identify?: boolean;
  subpath?: string;
};

export type RecentLibraryActivity = {
  libraryId: number;
  status: "done" | "failed";
  summary: string;
  detail?: string;
  finishedAt: string;
};

export type ScanQueueContextValue = {
  scanStatuses: Record<number, LibraryScanStatus>;
  activeLibraryIds: number[];
  activityScanStatuses: LibraryScanStatus[];
  recentLibraryActivities: RecentLibraryActivity[];
  getLibraryScanStatus: (libraryId: number | null) => LibraryScanStatus | undefined;
  hasLibraryScanStatus: (libraryId: number | null) => boolean;
  queueLibraryScan: (libraryId: number, options?: QueueScanOptions) => Promise<LibraryScanStatus>;
};

export const ScanQueueContext = createContext<ScanQueueContextValue | null>(null);

export function useScanQueue() {
  const ctx = useContext(ScanQueueContext);
  if (!ctx) throw new Error("useScanQueue must be used within ScanQueueProvider");
  return ctx;
}

