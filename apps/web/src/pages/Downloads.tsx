import { Link } from "react-router-dom";
import { ArrowDownCircle, Clock3, Download, ServerCog } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useAuthState } from "@/contexts/AuthContext";
import { useDownloads } from "@/queries";

function formatBytes(value: number | undefined): string {
  if (!value || value <= 0) {
    return "Unknown";
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
    return "Unknown";
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

export function Downloads() {
  const { user } = useAuthState();
  const isAdmin = user?.is_admin ?? false;
  const { data, error, isLoading, refetch } = useDownloads({ refetchInterval: 5_000 });

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-8">
      <section className="rounded-[var(--radius-xl)] border border-[var(--plum-border)] bg-[radial-gradient(circle_at_top_left,rgba(56,189,248,0.2),transparent_42%),linear-gradient(135deg,rgba(15,23,42,0.96),rgba(16,24,39,0.88))] p-6 text-white shadow-[0_20px_70px_rgba(9,12,20,0.28)]">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div className="max-w-2xl space-y-2">
            <div className="inline-flex items-center gap-2 rounded-full border border-white/15 bg-white/10 px-3 py-1 text-xs font-semibold uppercase tracking-[0.24em] text-white/75">
              <ArrowDownCircle className="size-3.5" />
              Downloads
            </div>
            <h1 className="text-3xl font-semibold tracking-tight">Track what Plum is pulling in.</h1>
            <p className="max-w-xl text-sm leading-6 text-white/75">
              See active movie and TV downloads flowing through Radarr and Sonarr TV without
              leaving Plum.
            </p>
          </div>
          <Button variant="outline" onClick={() => void refetch()} className="border-white/15 text-white hover:bg-white/10">
            Refresh now
          </Button>
        </div>
      </section>

      {isLoading && !data ? (
        <p className="text-sm text-[var(--plum-muted)]">Loading download activity...</p>
      ) : error ? (
        <div className="rounded-[var(--radius-xl)] border border-dashed border-[var(--plum-border)] bg-[var(--plum-panel)]/45 p-8">
          <h2 className="text-lg font-semibold text-[var(--plum-text)]">Downloads are unavailable</h2>
          <p className="mt-2 text-sm text-[var(--plum-muted)]">{error.message}</p>
        </div>
      ) : !data?.configured ? (
        <div className="rounded-[var(--radius-xl)] border border-dashed border-[var(--plum-border)] bg-[var(--plum-panel)]/45 p-8">
          <div className="flex items-start gap-3">
            <ServerCog className="mt-0.5 size-5 text-[var(--plum-accent)]" />
            <div>
              <h2 className="text-lg font-semibold text-[var(--plum-text)]">Media stack not configured</h2>
              <p className="mt-2 text-sm leading-6 text-[var(--plum-muted)]">
                Radarr and Sonarr TV need to be connected before Plum can show direct download activity.
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
      ) : (data.items.length ?? 0) === 0 ? (
        <div className="rounded-[var(--radius-xl)] border border-dashed border-[var(--plum-border)] bg-[var(--plum-panel)]/45 p-8">
          <div className="flex items-start gap-3">
            <Download className="mt-0.5 size-5 text-[var(--plum-accent)]" />
            <div>
              <h2 className="text-lg font-semibold text-[var(--plum-text)]">No active downloads</h2>
              <p className="mt-2 text-sm leading-6 text-[var(--plum-muted)]">
                New items you add from Discover will show up here while Radarr or Sonarr TV is working on them.
              </p>
            </div>
          </div>
        </div>
      ) : (
        <div className="grid gap-4">
          {data.items.map((item) => (
            <article
              key={item.id}
              className="rounded-[var(--radius-xl)] border border-[var(--plum-border)] bg-[var(--plum-panel)] p-5 shadow-[0_18px_45px_rgba(5,10,18,0.16)]"
            >
              <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="rounded-full border border-[var(--plum-border)] bg-[var(--plum-panel-alt)] px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--plum-muted)]">
                      {item.media_type === "movie" ? "Movie" : "TV"}
                    </span>
                    <span className="rounded-full bg-[color-mix(in_srgb,var(--plum-accent)_16%,transparent)] px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-[var(--plum-text)]">
                      {item.source}
                    </span>
                  </div>
                  <h2 className="mt-3 truncate text-xl font-semibold text-[var(--plum-text)]">
                    {item.title}
                  </h2>
                  <p className="mt-1 text-sm text-[var(--plum-muted)]">{item.status_text}</p>
                  {item.error_message ? (
                    <p className="mt-3 rounded-[var(--radius-md)] border border-amber-500/25 bg-amber-500/10 px-3 py-2 text-sm text-amber-100">
                      {item.error_message}
                    </p>
                  ) : null}
                </div>

                <div className="grid min-w-[14rem] gap-3 text-sm text-[var(--plum-text)] md:grid-cols-3 lg:grid-cols-1">
                  <div className="rounded-[var(--radius-lg)] border border-[var(--plum-border)] bg-[var(--plum-panel-alt)] px-4 py-3">
                    <div className="text-xs uppercase tracking-[0.16em] text-[var(--plum-muted)]">
                      Progress
                    </div>
                    <div className="mt-2 text-lg font-semibold">{Math.round(item.progress ?? 0)}%</div>
                  </div>
                  <div className="rounded-[var(--radius-lg)] border border-[var(--plum-border)] bg-[var(--plum-panel-alt)] px-4 py-3">
                    <div className="text-xs uppercase tracking-[0.16em] text-[var(--plum-muted)]">
                      Remaining
                    </div>
                    <div className="mt-2 text-lg font-semibold">{formatBytes(item.size_left_bytes)}</div>
                  </div>
                  <div className="rounded-[var(--radius-lg)] border border-[var(--plum-border)] bg-[var(--plum-panel-alt)] px-4 py-3">
                    <div className="flex items-center gap-2 text-xs uppercase tracking-[0.16em] text-[var(--plum-muted)]">
                      <Clock3 className="size-3.5" />
                      ETA
                    </div>
                    <div className="mt-2 text-lg font-semibold">{formatEta(item.eta_seconds)}</div>
                  </div>
                </div>
              </div>

              <div className="mt-4 h-2 overflow-hidden rounded-full bg-[rgba(255,255,255,0.08)]">
                <div
                  className="h-full rounded-full bg-[linear-gradient(90deg,var(--plum-accent),#38bdf8)]"
                  style={{ width: `${Math.max(0, Math.min(100, item.progress ?? 0))}%` }}
                />
              </div>
            </article>
          ))}
        </div>
      )}
    </div>
  );
}
