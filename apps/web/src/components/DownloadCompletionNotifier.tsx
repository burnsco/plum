import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  diffRemovedDownloads,
  downloadsToSnapshotMap,
  type DownloadSnapshotEntry,
} from "@/lib/downloadCompletion";
import { invalidateDiscoverRelatedQueries, useDownloads } from "@/queries";

export function DownloadCompletionNotifier() {
  const queryClient = useQueryClient();
  const prevRef = useRef<Map<string, DownloadSnapshotEntry> | null>(null);
  const sawConfiguredRef = useRef(false);
  const { data } = useDownloads({ refetchInterval: 5_000 });

  useEffect(() => {
    if (data == null) {
      return;
    }

    if (!data.configured) {
      prevRef.current = null;
      sawConfiguredRef.current = false;
      return;
    }

    const nextMap = downloadsToSnapshotMap(data.items);

    if (!sawConfiguredRef.current) {
      prevRef.current = nextMap;
      sawConfiguredRef.current = true;
      return;
    }

    const prev = prevRef.current;
    if (prev == null) {
      prevRef.current = nextMap;
      return;
    }

    const removed = diffRemovedDownloads(prev, data.items);
    if (removed.length > 0) {
      invalidateDiscoverRelatedQueries(queryClient);
      for (const item of removed) {
        if (item.hadError) {
          toast.error(`Download removed from queue: ${item.title}`);
        } else {
          toast.success(`Finished downloading: ${item.title}`);
        }
      }
    }

    prevRef.current = nextMap;
  }, [data, queryClient]);

  return null;
}
