import type { LibraryScanStatus } from "@/api";
import type { IdentifyLibraryPhase } from "@/contexts/IdentifyQueueContext";
import {
  getLibraryActivity,
  isLibraryScanProcessing,
  type LibraryActivity,
} from "@/lib/libraryActivity";

const IDENTIFY_POLL_INTERVAL_MS = 5_000;
const SCAN_POLL_INTERVAL_MS = 2_000;

type UseLibraryActivityStateArgs = {
  selectedLibraryId: number | null;
  getLibraryPhase: (libraryId: number | null) => IdentifyLibraryPhase | undefined;
  getLibraryScanStatus: (libraryId: number | null) => LibraryScanStatus | undefined;
};

export type LibraryActivityState = {
  activity: LibraryActivity | undefined;
  identifyPhase: IdentifyLibraryPhase | undefined;
  isProcessing: boolean;
  pollInterval: number | false;
  scanStatus: LibraryScanStatus | undefined;
};

export function canShowLibraryIdentifyFailure(
  identifyPhase: IdentifyLibraryPhase | undefined,
  isProcessing: boolean,
  hasActiveIdentifyItems: boolean,
  identifyFailedCount: number,
) {
  const explicitFailure = identifyPhase === "identify-failed" || identifyPhase === "partial";
  // Do not gate on react-query isFetching: background refetches (e.g. identify poll) would
  // briefly hide failure and flip cards back to "Searching...", which looks like a glitch.
  return (
    !hasActiveIdentifyItems &&
    (explicitFailure || (!isProcessing && identifyPhase === "complete" && identifyFailedCount > 0))
  );
}

function mapBackendIdentifyPhase(phase?: string): IdentifyLibraryPhase | undefined {
  switch (phase) {
    case "queued":
      return "queued";
    case "identifying":
      return "identifying";
    case "completed":
      return "complete";
    case "partial":
      return "partial";
    case "failed":
      return "identify-failed";
    default:
      return undefined;
  }
}

function resolveLibraryIdentifyPhase(
  localPhase: IdentifyLibraryPhase | undefined,
  backendPhase: IdentifyLibraryPhase | undefined,
) {
  if (localPhase === "queued" || localPhase === "identifying" || localPhase === "soft-reveal") {
    return localPhase;
  }
  if (backendPhase === "queued" || backendPhase === "identifying") {
    return backendPhase;
  }
  return localPhase ?? backendPhase;
}

export function useLibraryActivityState({
  selectedLibraryId,
  getLibraryPhase,
  getLibraryScanStatus,
}: UseLibraryActivityStateArgs): LibraryActivityState {
  const scanStatus = getLibraryScanStatus(selectedLibraryId);
  const backendIdentifyPhase = mapBackendIdentifyPhase(scanStatus?.identifyPhase);
  const identifyPhase = resolveLibraryIdentifyPhase(
    getLibraryPhase(selectedLibraryId),
    backendIdentifyPhase,
  );
  const activity = getLibraryActivity({
    scanPhase: scanStatus?.phase,
    enrichmentPhase: scanStatus?.enrichmentPhase,
    enriching: scanStatus?.enriching === true,
    identifyPhase: scanStatus?.identifyPhase,
    localIdentifyPhase: identifyPhase,
  });
  const isProcessing = scanStatus != null && isLibraryScanProcessing(scanStatus);
  const pollInterval =
    selectedLibraryId == null
      ? false
      : isProcessing
        ? SCAN_POLL_INTERVAL_MS
        : identifyPhase === "identifying" || identifyPhase === "soft-reveal"
          ? IDENTIFY_POLL_INTERVAL_MS
          : false;

  return {
    activity,
    identifyPhase,
    isProcessing,
    pollInterval,
    scanStatus,
  };
}
