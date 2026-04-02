import { Link } from "react-router-dom";
import { ArrowDownCircle, Clock3, Download, RefreshCw, ServerCog } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuthState } from "@/contexts/AuthContext";
import { useDownloads } from "@/queries";
import type { DownloadItem } from "@/api";

function formatBytes(value: number | undefined): string {
  if (!value || value <= 0) {
    return "—";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let unitIndex = 0;
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }
  return `${size.toFixed(size >= 100 || unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function formatEta(seconds: number | undefined): string {
  if (!seconds || seconds <= 0) {
    return "—";
  }
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (hours > 0) {
    return `${hours}h ${minutes}m`;
  }
  if (minutes > 0) {
    return `${minutes}m`;
  }
  return `${seconds}s`;
}

function ProgressCell({ progress }: { progress: number }) {
  const pct = Math.max(0, Math.min(100, progress));
  return (
    <div className="min-w-20">
      <span className="text-sm font-medium tabular-nums text-(--plum-text)">
        {Math.round(pct)}%
      </span>
      <div className="mt-1 h-0.75 w-full overflow-hidden rounded-full bg-[rgba(255,255,255,0.08)]">
        <div
          className="h-full rounded-full bg-[linear-gradient(90deg,var(--plum-accent),#38bdf8)] transition-[width] duration-500"
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}

function DownloadRow({ item }: { item: DownloadItem }) {
  return (
    <>
      <tr className="group border-b border-(--plum-border) transition-colors hover:bg-[rgba(181,123,255,0.04)]">
        {/* Title */}
        <td className="py-3 pl-4 pr-3">
          <span className="block max-w-xs truncate text-sm font-medium text-(--plum-text) lg:max-w-sm xl:max-w-md" title={item.title}>
            {item.title}
          </span>
          <span className="mt-0.5 block text-xs text-(--plum-muted)">{item.status_text}</span>
        </td>

        {/* Type */}
        <td className="whitespace-nowrap px-3 py-3">
          <span className="rounded-full border border-(--plum-border) bg-(--plum-panel-alt) px-2 py-0.5 text-[11px] font-semibold uppercase tracking-[0.14em] text-(--plum-muted)">
            {item.media_type === "movie" ? "Movie" : "TV"}
          </span>
        </td>

        {/* Source */}
        <td className="whitespace-nowrap px-3 py-3">
          <span className="rounded-full bg-[color-mix(in_srgb,var(--plum-accent)_14%,transparent)] px-2 py-0.5 text-[11px] font-semibold uppercase tracking-[0.14em] text-(--plum-text-2)">
            {item.source}
          </span>
        </td>

        {/* Progress */}
        <td className="px-3 py-3">
          <ProgressCell progress={item.progress ?? 0} />
        </td>

        {/* Remaining */}
        <td className="whitespace-nowrap px-3 py-3 text-right text-sm tabular-nums text-(--plum-text-2)">
          {formatBytes(item.size_left_bytes)}
        </td>

        {/* ETA */}
        <td className="whitespace-nowrap px-3 py-3 text-right">
          <span className="inline-flex items-center gap-1 text-sm tabular-nums text-(--plum-text-2)">
            <Clock3 className="size-3 shrink-0 text-(--plum-muted)" />
            {formatEta(item.eta_seconds)}
          </span>
        </td>
      </tr>
      {item.error_message ? (
        <tr className="border-b border-(--plum-border)">
          <td
            colSpan={6}
            className="border-l-2 border-amber-500/60 bg-amber-500/6 py-2 pl-4 pr-4 text-xs text-amber-200"
          >
            {item.error_message}
          </td>
        </tr>
      ) : null}
    </>
  );
}

export function Downloads() {
  const { user } = useAuthState();
  const isAdmin = user?.is_admin ?? false;
  const { data, error, isLoading, refetch } = useDownloads({ refetchInterval: 5_000 });

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-5">
      {/* Page header */}
      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-2.5">
          <ArrowDownCircle className="size-5 text-(--plum-accent)" />
          <div>
            <h1 className="text-xl font-semibold text-(--plum-text)">Downloads</h1>
            <p className="text-xs text-(--plum-muted)">
              Live queue from Radarr and Sonarr TV
            </p>
          </div>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => void refetch()}
          className="flex items-center gap-1.5"
        >
          <RefreshCw className="size-3.5" />
          Refresh
        </Button>
      </div>

      {/* States */}
      {isLoading && !data ? (
        <p className="text-sm text-(--plum-muted)">Loading download activity…</p>
      ) : error ? (
        <div className="rounded-(--radius-xl) border border-dashed border-(--plum-border) bg-(--plum-panel)/45 p-8">
          <h2 className="text-base font-semibold text-(--plum-text)">Downloads unavailable</h2>
          <p className="mt-2 text-sm text-(--plum-muted)">{error.message}</p>
        </div>
      ) : !data?.configured ? (
        <div className="rounded-(--radius-xl) border border-dashed border-(--plum-border) bg-(--plum-panel)/45 p-8">
          <div className="flex items-start gap-3">
            <ServerCog className="mt-0.5 size-5 text-(--plum-accent)" />
            <div>
              <h2 className="text-base font-semibold text-(--plum-text)">
                Media stack not configured
              </h2>
              <p className="mt-2 text-sm leading-6 text-(--plum-muted)">
                Radarr and Sonarr TV need to be connected before Plum can show direct download
                activity.
              </p>
              {isAdmin ? (
                <div className="mt-4">
                  <Button asChild>
                    <Link to="/settings">Open Settings</Link>
                  </Button>
                </div>
              ) : null}
            </div>
          </div>
        </div>
      ) : (data?.items.length ?? 0) === 0 ? (
        <div className="rounded-(--radius-xl) border border-dashed border-(--plum-border) bg-(--plum-panel)/45 p-8">
          <div className="flex items-start gap-3">
            <Download className="mt-0.5 size-5 text-(--plum-accent)" />
            <div>
              <h2 className="text-base font-semibold text-(--plum-text)">
                No active downloads
              </h2>
              <p className="mt-2 text-sm leading-6 text-(--plum-muted)">
                New items you add from Discover will show up here while Radarr or Sonarr TV is
                working on them.
              </p>
            </div>
          </div>
        </div>
      ) : (
        /* Download table */
        <div className="overflow-hidden rounded-lg border border-(--plum-border) bg-(--plum-panel)">
          <table className="w-full text-left">
            <thead>
              <tr className="border-b border-(--plum-border) bg-(--plum-panel-alt)/60">
                <th className="py-2.5 pl-4 pr-3 text-[11px] font-semibold uppercase tracking-[0.16em] text-(--plum-muted)">
                  Title
                </th>
                <th className="px-3 py-2.5 text-[11px] font-semibold uppercase tracking-[0.16em] text-(--plum-muted)">
                  Type
                </th>
                <th className="px-3 py-2.5 text-[11px] font-semibold uppercase tracking-[0.16em] text-(--plum-muted)">
                  Source
                </th>
                <th className="px-3 py-2.5 text-[11px] font-semibold uppercase tracking-[0.16em] text-(--plum-muted)">
                  Progress
                </th>
                <th className="px-3 py-2.5 text-right text-[11px] font-semibold uppercase tracking-[0.16em] text-(--plum-muted)">
                  Remaining
                </th>
                <th className="px-3 py-2.5 text-right text-[11px] font-semibold uppercase tracking-[0.16em] text-(--plum-muted)">
                  ETA
                </th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((item) => (
                <DownloadRow key={item.id} item={item} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
