import type { DownloadItem } from "@/api";

export type DownloadSnapshotEntry = Pick<DownloadItem, "title" | "error_message">;

export function downloadsToSnapshotMap(items: DownloadItem[]): Map<string, DownloadSnapshotEntry> {
  return new Map(
    items.map((item) => [
      item.id,
      { title: item.title, error_message: item.error_message },
    ]),
  );
}

export function diffRemovedDownloads(
  previous: Map<string, DownloadSnapshotEntry>,
  nextItems: DownloadItem[],
): { id: string; title: string; hadError: boolean }[] {
  const nextIds = new Set(nextItems.map((item) => item.id));
  const removed: { id: string; title: string; hadError: boolean }[] = [];
  for (const [id, entry] of previous) {
    if (!nextIds.has(id)) {
      removed.push({
        id,
        title: entry.title.trim() || "Unknown title",
        hadError: Boolean(entry.error_message?.trim()),
      });
    }
  }
  return removed;
}
