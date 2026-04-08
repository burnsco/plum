import { useState, useMemo, useEffect } from "react";
import type {
  IntroRefreshLibraryStatus,
  IntroRefreshStatusResponse,
  IntroScanLibrarySummary,
  IntroScanShowSummary,
  IntroSkipMode,
  Library,
} from "@/api";
import { Button } from "@/components/ui/button";
import {
  useIntroScanSummary,
  useIntroScanShowSummary,
  useIntroRefreshStatus,
  useRefreshLibraryIntroOnly,
  usePostLibraryIntroChromaprintScan,
  useLibraries,
  useUpdateLibraryPlaybackPreferences,
} from "@/queries";
import { cn } from "@/lib/utils";
import { ChevronDown, ChevronRight, RefreshCw, Sparkles } from "lucide-react";

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
  label,
  status,
}: {
  label: string;
  status: IntroRefreshLibraryStatus;
}) {
  const pct =
    status.total > 0
      ? Math.round((status.processed / status.total) * 100)
      : 0;
  return (
    <div className="mt-2 rounded-md border border-amber-800/40 bg-amber-950/30 px-3 py-2">
      <div className="flex items-center justify-between gap-2">
        <span className="text-xs font-medium text-amber-300">
          {label}{" "}
          {status.total > 0
            ? `${status.processed}/${status.total} (${pct}%)`
            : "…"}
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

function jobMapsFromResponse(data: IntroRefreshStatusResponse | undefined) {
  const intro = new Map<number, IntroRefreshLibraryStatus>();
  const chroma = new Map<number, IntroRefreshLibraryStatus>();
  const playback = new Map<number, IntroRefreshLibraryStatus>();
  for (const s of data?.intro_only_libraries ?? []) {
    intro.set(s.library_id, s);
  }
  for (const s of data?.chromaprint_libraries ?? []) {
    chroma.set(s.library_id, s);
  }
  for (const s of data?.libraries ?? []) {
    playback.set(s.library_id, s);
  }
  return { intro, chroma, playback };
}

function LibraryCard({
  library,
  fullLibrary,
  onRefreshIntroOnly,
  onChromaprint,
  onIntroSkipMode,
  isBusy,
  introStatus,
  chromaStatus,
  chromaprintPending,
}: {
  library: IntroScanLibrarySummary;
  fullLibrary: Library | undefined;
  onRefreshIntroOnly: (libraryId: number) => void;
  onChromaprint: (libraryId: number) => void;
  onIntroSkipMode: (libraryId: number, mode: IntroSkipMode) => void;
  isBusy: boolean;
  introStatus: IntroRefreshLibraryStatus | undefined;
  chromaStatus: IntroRefreshLibraryStatus | undefined;
  chromaprintPending: boolean;
}) {
  const [expanded, setExpanded] = useState(false);
  const isShowLibrary = library.type === "tv" || library.type === "anime";

  return (
    <article className="rounded-md border border-(--plum-border) bg-(--plum-panel-alt)/60 p-4">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-base font-medium text-(--plum-text)">
              {library.name}
            </h3>
            <span className="rounded-full bg-(--plum-panel) px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-(--plum-muted)">
              {libraryTypeLabel(library.type)}
            </span>
            {fullLibrary ? (
              <label className="flex items-center gap-1.5 text-[10px] font-medium uppercase tracking-wider text-(--plum-muted)">
                <span className="sr-only">Intro skip during playback</span>
                <select
                  className={cn(
                    "max-w-[140px] cursor-pointer rounded-full border-0 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider outline-none focus-visible:ring-2 focus-visible:ring-(--plum-accent)",
                    introSkipModeBadgeClass(library.intro_skip_mode),
                  )}
                  value={library.intro_skip_mode}
                  onChange={(e) =>
                    onIntroSkipMode(
                      library.library_id,
                      e.target.value as IntroSkipMode,
                    )
                  }
                >
                  <option value="off">Off</option>
                  <option value="manual">Skip button</option>
                  <option value="auto">Auto-skip</option>
                </select>
              </label>
            ) : (
              <span
                className={cn(
                  "rounded-full px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider",
                  introSkipModeBadgeClass(library.intro_skip_mode),
                )}
              >
                {introSkipModeLabel(library.intro_skip_mode)}
              </span>
            )}
          </div>
          <p className="mt-1 text-sm text-(--plum-muted)">
            {library.with_intro} of {library.total_episodes}{" "}
            {library.type === "movie" ? "items" : "episodes"} have intros
            detected
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={isBusy}
            onClick={() => onRefreshIntroOnly(library.library_id)}
          >
            <RefreshCw
              className={cn("mr-1.5 size-3.5", isBusy && "animate-spin")}
            />
            {isBusy ? "Working…" : "Re-scan intros"}
          </Button>
          {isShowLibrary ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={isBusy || chromaprintPending}
              title="Audio fingerprinting (requires ffmpeg with chromaprint). See server docs."
              onClick={() => onChromaprint(library.library_id)}
            >
              <Sparkles
                className={cn(
                  "mr-1.5 size-3.5",
                  chromaprintPending && "animate-pulse",
                )}
              />
              {chromaprintPending ? "Fingerprint…" : "Chromaprint"}
            </Button>
          ) : null}
        </div>
      </div>

      <div className="mt-3">
        <ProgressBar value={library.with_intro} max={library.total_episodes} />
      </div>

      {introStatus ? (
        <RefreshProgressIndicator
          label="Intro re-probe:"
          status={introStatus}
        />
      ) : null}
      {chromaStatus ? (
        <RefreshProgressIndicator
          label="Chromaprint scan:"
          status={chromaStatus}
        />
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
  const librariesQuery = useLibraries();
  const refreshIntroMutation = useRefreshLibraryIntroOnly();
  const chromaprintMutation = usePostLibraryIntroChromaprintScan();
  const prefsMutation = useUpdateLibraryPlaybackPreferences();
  const [pendingJobs, setPendingJobs] = useState<Set<number>>(() => new Set());

  const refreshStatusQuery = useIntroRefreshStatus(pendingJobs.size > 0);
  const { introMap, chromaMap, playbackMap } = useMemo(() => {
    const { intro, chroma, playback } = jobMapsFromResponse(
      refreshStatusQuery.data,
    );
    return { introMap: intro, chromaMap: chroma, playbackMap: playback };
  }, [refreshStatusQuery.data]);

  const libraryById = useMemo(() => {
    const m = new Map<number, Library>();
    for (const lib of librariesQuery.data ?? []) {
      m.set(lib.id, lib);
    }
    return m;
  }, [librariesQuery.data]);

  useEffect(() => {
    if (!refreshStatusQuery.data) return;
    let changed = false;
    const next = new Set<number>();
    for (const id of pendingJobs) {
      if (
        introMap.has(id) ||
        chromaMap.has(id) ||
        playbackMap.has(id)
      ) {
        next.add(id);
      } else {
        changed = true;
      }
    }
    if (changed) {
      setPendingJobs(next);
      if (next.size === 0) {
        void summaryQuery.refetch();
      }
    }
  }, [
    refreshStatusQuery.data,
    introMap,
    chromaMap,
    playbackMap,
    pendingJobs,
    summaryQuery,
  ]);

  const handleRefreshIntro = (libraryId: number) => {
    setPendingJobs((prev) => new Set(prev).add(libraryId));
    refreshIntroMutation.mutate({ libraryId });
  };

  const handleChromaprint = (libraryId: number) => {
    setPendingJobs((prev) => new Set(prev).add(libraryId));
    chromaprintMutation.mutate({ libraryId });
  };

  const handleIntroSkipMode = (libraryId: number, mode: IntroSkipMode) => {
    const lib = libraryById.get(libraryId);
    if (!lib) return;
    prefsMutation.mutate({
      libraryId,
      payload: {
        preferred_audio_language: lib.preferred_audio_language ?? "",
        preferred_subtitle_language: lib.preferred_subtitle_language ?? "",
        subtitles_enabled_by_default: lib.subtitles_enabled_by_default ?? false,
        intro_skip_mode: mode,
        watcher_enabled: lib.watcher_enabled,
        watcher_mode: lib.watcher_mode,
        scan_interval_minutes: lib.scan_interval_minutes,
      },
    });
    void summaryQuery.refetch();
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
            Plum prefers chapter markers embedded in your media (Intro,
            Opening, OP, etc.). When those are missing, it falls back to ffmpeg
            silence detection on the first audio stream. Re-scan intros runs a
            lightweight intro-only pass (chapters + opening silence). Chromaprint
            (TV/anime) clusters episodes by matching audio fingerprints; ffmpeg
            must include the chromaprint muxer (for example jellyfin-ffmpeg).
            Manual bounds: PATCH /api/media/&#123;id&#125;/intro. Locked intros
            are not overwritten by automatic scans.
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
          {libraries.map((lib) => {
            const id = lib.library_id;
            const serverBusy =
              introMap.has(id) || chromaMap.has(id) || playbackMap.has(id);
            const clientBusy =
              (refreshIntroMutation.isPending &&
                refreshIntroMutation.variables?.libraryId === id) ||
              (chromaprintMutation.isPending &&
                chromaprintMutation.variables?.libraryId === id);
            return (
              <LibraryCard
                key={id}
                library={lib}
                fullLibrary={libraryById.get(id)}
                onRefreshIntroOnly={handleRefreshIntro}
                onChromaprint={handleChromaprint}
                onIntroSkipMode={handleIntroSkipMode}
                isBusy={serverBusy || clientBusy}
                introStatus={introMap.get(id)}
                chromaStatus={chromaMap.get(id)}
                chromaprintPending={
                  chromaprintMutation.isPending &&
                  chromaprintMutation.variables?.libraryId === id
                }
              />
            );
          })}
        </div>
      )}
    </div>
  );
}
