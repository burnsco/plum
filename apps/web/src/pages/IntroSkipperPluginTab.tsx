import { useState, useMemo, useEffect } from "react";
import type {
  IntroRefreshLibraryStatus,
  IntroScanLibrarySummary,
  IntroScanShowSummary,
} from "@/api";
import { Button } from "@/components/ui/button";
import {
  useIntroScanSummary,
  useIntroScanShowSummary,
  useIntroRefreshStatus,
  useRefreshLibraryIntros,
} from "@/queries";
import { cn } from "@/lib/utils";
import { ChevronDown, ChevronRight, RefreshCw } from "lucide-react";

function libraryTypeLabel(mediaType: string): string {
  switch (mediaType) {
    case "movie":
      return "Movies";
    case "tv":
      return "TV";
    case "anime":
      return "Anime";
    default:
      return mediaType;
  }
}

function introSkipModeLabel(mode: string): string {
  switch (mode) {
    case "off":
      return "Off";
    case "manual":
      return "Skip button";
    case "auto":
      return "Auto-skip";
    default:
      return mode;
  }
}

function introSkipModeBadgeClass(mode: string): string {
  switch (mode) {
    case "off":
      return "bg-zinc-700/60 text-zinc-300";
    case "auto":
      return "bg-emerald-900/40 text-emerald-300";
    default:
      return "bg-violet-900/40 text-violet-300";
  }
}

function displayFileName(fullPath: string): string {
  const parts = fullPath.split("/");
  return parts[parts.length - 1] || fullPath;
}

function ProgressBar({ value, max }: { value: number; max: number }) {
  const pct = max > 0 ? Math.round((value / max) * 100) : 0;
  return (
    <div className="flex items-center gap-3">
      <div className="h-2 flex-1 overflow-hidden rounded-full bg-(--plum-panel-alt)">
        <div
          className="h-full rounded-full bg-(--plum-accent) transition-all duration-300"
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className="shrink-0 text-xs tabular-nums text-(--plum-muted)">
        {value}/{max} ({pct}%)
      </span>
    </div>
  );
}

function ShowRow({ show }: { show: IntroScanShowSummary }) {
  return (
    <div className="flex items-center justify-between gap-3 py-1.5">
      <span className="min-w-0 truncate text-sm text-(--plum-text-secondary)">
        {show.show_title}
      </span>
      <span className="shrink-0 text-xs tabular-nums text-(--plum-muted)">
        {show.with_intro}/{show.total_episodes}
      </span>
    </div>
  );
}

function LibraryShowList({ libraryId }: { libraryId: number }) {
  const showsQuery = useIntroScanShowSummary(libraryId);

  if (showsQuery.isLoading) {
    return (
      <p className="py-2 text-xs text-(--plum-muted)">Loading shows...</p>
    );
  }
  if (showsQuery.isError) {
    return (
      <p className="py-2 text-xs text-red-400">
        Failed to load show breakdown.
      </p>
    );
  }
  const shows = showsQuery.data?.shows ?? [];
  if (shows.length === 0) {
    return (
      <p className="py-2 text-xs text-(--plum-muted)">No shows found.</p>
    );
  }
  return (
    <div className="mt-2 max-h-80 divide-y divide-(--plum-border)/50 overflow-y-auto rounded-md border border-(--plum-border)/50 bg-(--plum-panel)/60 px-3">
      {shows.map((show) => (
        <ShowRow key={show.show_key} show={show} />
      ))}
    </div>
  );
}

function RefreshProgressIndicator({
  status,
}: {
  status: IntroRefreshLibraryStatus;
}) {
  const pct =
    status.total > 0
      ? Math.round((status.processed / status.total) * 100)
      : 0;
  return (
    <div className="mt-3 rounded-md border border-amber-800/40 bg-amber-950/30 px-3 py-2">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs font-medium text-amber-300">
          Scanning... {status.processed}/{status.total} ({pct}%)
        </span>
      </div>
      {status.current_path ? (
        <p className="mt-1 truncate text-xs text-amber-300/70">
          {displayFileName(status.current_path)}
        </p>
      ) : null}
      <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-amber-900/40">
        <div
          className="h-full rounded-full bg-amber-500 transition-all duration-300"
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

function LibraryCard({
  library,
  refreshIntros,
  isRefreshing,
  refreshStatus,
}: {
  library: IntroScanLibrarySummary;
  refreshIntros: (libraryId: number) => void;
  isRefreshing: boolean;
  refreshStatus: IntroRefreshLibraryStatus | undefined;
}) {
  const [expanded, setExpanded] = useState(false);
  const isShowLibrary = library.type === "tv" || library.type === "anime";

  return (
    <article className="rounded-md border border-(--plum-border) bg-(--plum-panel-alt)/60 p-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <h3 className="text-base font-medium text-(--plum-text)">
              {library.name}
            </h3>
            <span className="rounded-full bg-(--plum-panel) px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-(--plum-muted)">
              {libraryTypeLabel(library.type)}
            </span>
            <span
              className={cn(
                "rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider",
                introSkipModeBadgeClass(library.intro_skip_mode),
              )}
            >
              {introSkipModeLabel(library.intro_skip_mode)}
            </span>
          </div>
          <p className="mt-1 text-sm text-(--plum-muted)">
            {library.with_intro} of {library.total_episodes}{" "}
            {library.type === "movie" ? "items" : "episodes"} have intros
            detected
          </p>
        </div>
        <Button
          type="button"
          variant="secondary"
          size="sm"
          disabled={isRefreshing}
          onClick={() => refreshIntros(library.library_id)}
        >
          <RefreshCw
            className={cn("mr-1.5 size-3.5", isRefreshing && "animate-spin")}
          />
          {isRefreshing ? "Scanning..." : "Re-scan intros"}
        </Button>
      </div>

      <div className="mt-3">
        <ProgressBar value={library.with_intro} max={library.total_episodes} />
      </div>

      {refreshStatus ? (
        <RefreshProgressIndicator status={refreshStatus} />
      ) : null}

      {isShowLibrary ? (
        <div className="mt-3">
          <button
            type="button"
            onClick={() => setExpanded((prev) => !prev)}
            className="flex items-center gap-1 text-xs font-medium text-(--plum-muted) hover:text-(--plum-text) transition-colors"
          >
            {expanded ? (
              <ChevronDown className="size-3.5" />
            ) : (
              <ChevronRight className="size-3.5" />
            )}
            {expanded ? "Hide shows" : "Show breakdown by series"}
          </button>
          {expanded ? (
            <LibraryShowList libraryId={library.library_id} />
          ) : null}
        </div>
      ) : null}
    </article>
  );
}

export function IntroSkipperPluginTab() {
  const summaryQuery = useIntroScanSummary();
  const refreshMutation = useRefreshLibraryIntros();
  const [pendingRefreshes, setPendingRefreshes] = useState<Set<number>>(
    () => new Set(),
  );

  const refreshStatusQuery = useIntroRefreshStatus(pendingRefreshes.size > 0);
  const refreshStatusMap = useMemo(() => {
    const map = new Map<number, IntroRefreshLibraryStatus>();
    for (const s of refreshStatusQuery.data?.libraries ?? []) {
      map.set(s.library_id, s);
    }
    return map;
  }, [refreshStatusQuery.data]);

  // Clear pending state once server confirms scan is done (no longer in status map)
  useEffect(() => {
    if (!refreshStatusQuery.data) return;
    let changed = false;
    const next = new Set<number>();
    for (const id of pendingRefreshes) {
      if (refreshStatusMap.has(id)) {
        next.add(id);
      } else {
        changed = true;
      }
    }
    if (changed) {
      setPendingRefreshes(next);
      if (next.size === 0) {
        void summaryQuery.refetch();
      }
    }
  }, [refreshStatusQuery.data]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleRefresh = (libraryId: number) => {
    setPendingRefreshes((prev) => new Set(prev).add(libraryId));
    refreshMutation.mutate({ libraryId });
  };

  if (summaryQuery.isLoading) {
    return (
      <div className="flex flex-col gap-3 rounded-lg border border-(--plum-border) bg-(--plum-panel)/85 p-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]">
        <p className="text-sm text-(--plum-muted)">
          Loading intro scan summary...
        </p>
      </div>
    );
  }

  if (summaryQuery.isError) {
    return (
      <div className="flex flex-col gap-3 rounded-lg border border-(--plum-border) bg-(--plum-panel)/85 p-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]">
        <p className="text-sm text-red-400">
          Failed to load intro scan summary.
        </p>
      </div>
    );
  }

  const libraries = summaryQuery.data?.libraries ?? [];

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-3 rounded-lg border border-(--plum-border) bg-(--plum-panel)/85 p-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)] md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-xl font-semibold text-(--plum-text)">
            Intro Skipper
          </h2>
          <p className="mt-1 max-w-2xl text-sm text-(--plum-muted)">
            Plum detects intros by reading chapter markers embedded in your
            media files (titles like "Intro", "Opening", "OP", etc.). Library
            scans analyze new and changed files, and run a one-time chapter
            pass on existing files until each has been checked. Use Re-scan
            intros to force ffprobe on every item in a library (for example
            after re-muxing with new chapters).
          </p>
        </div>
      </div>

      {libraries.length === 0 ? (
        <div className="rounded-md border border-(--plum-border) bg-(--plum-panel-alt)/60 p-6 text-center">
          <p className="text-sm text-(--plum-muted)">
            No libraries found. Add a library to get started with intro
            detection.
          </p>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {libraries.map((lib) => (
            <LibraryCard
              key={lib.library_id}
              library={lib}
              refreshIntros={handleRefresh}
              isRefreshing={pendingRefreshes.has(lib.library_id)}
              refreshStatus={refreshStatusMap.get(lib.library_id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
