import { useEffect, useRef } from "react";
import { toast } from "sonner";
import { useLibraries, useHomeDashboard } from "@/queries";
import {
  buildRecentlyAddedToastMessage,
  recentlyAddedEntryKey,
} from "@/lib/libraryReadyNotifications";

export function LibraryReadyNotifier() {
  const initializedRef = useRef(false);
  const seenRef = useRef<Set<string>>(new Set());
  const { data: libraries = [] } = useLibraries();
  const { data: dashboard } = useHomeDashboard();

  useEffect(() => {
    if (dashboard == null) {
      return;
    }
    const recentlyAdded = dashboard.recentlyAdded ?? [];

    const currentKeys = new Set(recentlyAdded.map(recentlyAddedEntryKey));
    if (!initializedRef.current) {
      seenRef.current = currentKeys;
      initializedRef.current = true;
      return;
    }

    const nextSeen = new Set(seenRef.current);
    for (const entry of recentlyAdded) {
      const key = recentlyAddedEntryKey(entry);
      if (nextSeen.has(key)) {
        continue;
      }
      nextSeen.add(key);
      toast.success(buildRecentlyAddedToastMessage(entry, libraries));
    }
    seenRef.current = nextSeen;
  }, [dashboard, libraries]);

  return null;
}
