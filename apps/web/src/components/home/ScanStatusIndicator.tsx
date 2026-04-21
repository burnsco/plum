import type { LibraryScanStatus } from "@/api";
import { getLibraryActivityStatusMessage, type LibraryActivity } from "@/lib/libraryActivity";

type ScanStatusIndicatorProps = {
  activity: LibraryActivity;
  scanStatus: LibraryScanStatus | undefined;
};

export function ScanStatusIndicator({ activity, scanStatus }: ScanStatusIndicatorProps) {
  return (
    <p className="text-sm text-(--plum-muted)">
      {getLibraryActivityStatusMessage(activity)}
      {activity === "importing" && scanStatus ? (
        <>
          {" "}
          {scanStatus.processed} processed • {scanStatus.added} added
        </>
      ) : null}
    </p>
  );
}
