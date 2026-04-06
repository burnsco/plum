import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type {
  AdminActivePlaybackResponse,
  AdminLogsResponse,
  AdminMaintenanceRunResponse,
  AdminMaintenanceScheduleResponse,
  AdminMaintenanceTaskId,
} from "@plum/contracts";
import {
  getAdminActivePlayback,
  getAdminLogs,
  getAdminMaintenanceSchedule,
  putAdminMaintenanceSchedule,
  runAdminMaintenanceTask,
} from "@/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";
import { contractsView } from "@/queries";
import { Activity, FileText, Gauge, Loader2, Shield } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

type AdminSubTab = "maintenance" | "logs" | "activity";

const TASKS: {
  id: AdminMaintenanceTaskId;
  label: string;
  description: string;
}[] = [
  {
    id: "optimize_database",
    label: "Optimize database",
    description:
      "Runs SQLite VACUUM to compact the database and reclaim free space. Use after large library changes.",
  },
  {
    id: "clean_transcode",
    label: "Clean transcode directory",
    description: "Deletes transcode session folders and legacy temp files older than one day (skips active sessions).",
  },
  {
    id: "clean_logs",
    label: "Clean log directory",
    description: "Removes .log files older than three days from the configured log directory (PLUM_LOG_FILE or PLUM_LOG_DIR).",
  },
  {
    id: "delete_cache",
    label: "Delete cache",
    description: "Prunes expired metadata provider cache rows and applies optional row-cap trimming from server settings.",
  },
  {
    id: "scan_all_media",
    label: "Scan all media",
    description: "Queues a full scan for every library to pick up added, moved, or removed files.",
  },
  {
    id: "extract_chapter_images",
    label: "Extract chapter metadata",
    description:
      "Starts playback metadata refresh (ffprobe chapters, embedded tracks, intro hints) for movie, TV, and anime libraries.",
  },
  {
    id: "check_metadata_updates",
    label: "Check for metadata updates",
    description: "Runs identify/refresh for every library in the background to pick up provider metadata changes.",
  },
];

function subTabClass(active: boolean): string {
  return cn(
    "inline-flex items-center gap-2 rounded-md px-3 py-1.5 text-sm font-medium transition-colors",
    active
      ? "bg-[rgba(181,123,255,0.12)] text-(--plum-text) shadow-[0_0_16px_rgba(139,92,246,0.12)]"
      : "text-(--plum-muted) hover:bg-[rgba(255,255,255,0.04)] hover:text-(--plum-text)",
  );
}

export function AdminSettingsTab() {
  const queryClient = useQueryClient();
  const [subTab, setSubTab] = useState<AdminSubTab>("maintenance");
  const [scheduleDraft, setScheduleDraft] = useState<Record<string, number>>({});
  const [scheduleDirty, setScheduleDirty] = useState(false);
  const [taskMessage, setTaskMessage] = useState<string | null>(null);
  const [runningTask, setRunningTask] = useState<AdminMaintenanceTaskId | null>(null);

  const scheduleQuery = useQuery({
    queryKey: ["admin", "maintenance-schedule"],
    queryFn: async () =>
      contractsView<AdminMaintenanceScheduleResponse>(await getAdminMaintenanceSchedule()),
  });

  useEffect(() => {
    if (!scheduleQuery.data || scheduleDirty) return;
    const next: Record<string, number> = {};
    for (const [k, v] of Object.entries(scheduleQuery.data.tasks)) {
      next[k] = v.intervalHours;
    }
    setScheduleDraft(next);
  }, [scheduleQuery.data, scheduleDirty]);

  const saveSchedule = useMutation({
    mutationFn: async (tasks: Record<string, { intervalHours: number }>) =>
      contractsView<AdminMaintenanceScheduleResponse>(await putAdminMaintenanceSchedule({ tasks })),
    onSuccess: () => {
      setScheduleDirty(false);
      void queryClient.invalidateQueries({ queryKey: ["admin", "maintenance-schedule"] });
    },
  });

  const runTask = useMutation({
    mutationFn: async (task: AdminMaintenanceTaskId) =>
      contractsView<AdminMaintenanceRunResponse>(await runAdminMaintenanceTask({ task })),
    onMutate: (task) => {
      setRunningTask(task);
    },
    onSuccess: (res) => {
      const parts = [res.detail, res.error].filter(Boolean);
      setTaskMessage(parts.join(" ") || (res.accepted ? "Task finished." : "Task was not accepted."));
      void queryClient.invalidateQueries({ queryKey: ["admin", "maintenance-schedule"] });
    },
    onError: (e: Error) => {
      setTaskMessage(e.message);
    },
    onSettled: () => {
      setRunningTask(null);
    },
  });

  const logsQuery = useQuery({
    queryKey: ["admin", "logs"],
    queryFn: async () => contractsView<AdminLogsResponse>(await getAdminLogs({ lines: 500 })),
    enabled: subTab === "logs",
    refetchInterval: subTab === "logs" ? 4000 : false,
  });

  const activityQuery = useQuery({
    queryKey: ["admin", "active-playback"],
    queryFn: async () => contractsView<AdminActivePlaybackResponse>(await getAdminActivePlayback()),
    enabled: subTab === "activity",
    refetchInterval: subTab === "activity" ? 3000 : false,
  });

  const onIntervalChange = useCallback((taskId: string, raw: string) => {
    const n = parseInt(raw, 10);
    setScheduleDirty(true);
    setScheduleDraft((prev) => ({
      ...prev,
      [taskId]: Number.isFinite(n) && n >= 0 ? n : 0,
    }));
  }, []);

  const persistSchedule = useCallback(() => {
    const tasks: Record<string, { intervalHours: number }> = {};
    for (const t of TASKS) {
      const hours = scheduleDraft[t.id] ?? 0;
      tasks[t.id] = { intervalHours: Math.min(8760, Math.max(0, hours)) };
    }
    saveSchedule.mutate(tasks);
  }, [scheduleDraft, saveSchedule]);

  const lastRun = scheduleQuery.data?.lastRun ?? {};

  const subTabNav = useMemo(
    () => (
      <div className="flex flex-wrap gap-2 border-b border-(--plum-border) pb-3">
        <button type="button" className={subTabClass(subTab === "maintenance")} onClick={() => setSubTab("maintenance")}>
          <Gauge className="size-4 shrink-0" aria-hidden />
          Maintenance
        </button>
        <button type="button" className={subTabClass(subTab === "logs")} onClick={() => setSubTab("logs")}>
          <FileText className="size-4 shrink-0" aria-hidden />
          Logs
        </button>
        <button type="button" className={subTabClass(subTab === "activity")} onClick={() => setSubTab("activity")}>
          <Activity className="size-4 shrink-0" aria-hidden />
          Activity
        </button>
      </div>
    ),
    [subTab],
  );

  return (
    <div className="flex flex-col gap-6">
      <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-5">
        <div className="flex items-start gap-3">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-violet-500/15 text-violet-200">
            <Shield className="size-5" aria-hidden />
          </div>
          <div>
            <h2 className="text-lg font-semibold text-(--plum-text)">Administration</h2>
            <p className="mt-1 text-sm text-(--plum-muted)">
              Server maintenance, log tail, and live playback activity. Scheduled tasks run automatically when interval
              hours are set (0 disables).
            </p>
          </div>
        </div>
      </div>

      {subTabNav}

      {taskMessage ? (
        <p className="rounded-md border border-(--plum-border) bg-(--plum-panel)/60 px-3 py-2 text-sm text-(--plum-text-secondary)">
          {taskMessage}
        </p>
      ) : null}

      {subTab === "maintenance" ? (
        <div className="flex flex-col gap-6">
          {scheduleQuery.isLoading ? (
            <p className="text-sm text-(--plum-muted)">Loading schedule…</p>
          ) : scheduleQuery.isError ? (
            <p className="text-sm text-red-300">{(scheduleQuery.error as Error).message}</p>
          ) : (
            <section className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-5">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">Schedule</h3>
                <Button
                  type="button"
                  size="sm"
                  variant="secondary"
                  disabled={saveSchedule.isPending || !scheduleDirty}
                  onClick={() => persistSchedule()}
                >
                  {saveSchedule.isPending ? (
                    <>
                      <Loader2 className="mr-2 size-4 animate-spin" />
                      Saving…
                    </>
                  ) : (
                    "Save intervals"
                  )}
                </Button>
              </div>
              <p className="mt-2 text-xs text-(--plum-muted)">
                Interval is hours between automatic runs. Leave at 0 for manual-only.
              </p>
            </section>
          )}

          <section className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-5">
            <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">Tasks</h3>
            <ul className="mt-4 flex flex-col gap-4">
              {TASKS.map((task) => (
                <li
                  key={task.id}
                  className="flex flex-col gap-3 rounded-md border border-(--plum-border)/60 bg-(--plum-panel)/50 p-4 md:flex-row md:items-start md:justify-between"
                >
                  <div className="min-w-0 flex-1">
                    <p className="font-medium text-(--plum-text)">{task.label}</p>
                    <p className="mt-1 text-sm text-(--plum-muted)">{task.description}</p>
                    {lastRun[task.id] ? (
                      <p className="mt-2 text-xs text-(--plum-muted)">Last run: {lastRun[task.id]}</p>
                    ) : null}
                  </div>
                  <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                    <div className="flex items-center gap-2 text-xs text-(--plum-muted)">
                      <label className="whitespace-nowrap" htmlFor={`admin-schedule-${task.id}`}>
                        Every (h)
                      </label>
                      <Input
                        id={`admin-schedule-${task.id}`}
                        type="number"
                        min={0}
                        max={8760}
                        className="h-8 w-20 border-(--plum-chrome-border) bg-(--plum-field-fill)"
                        value={scheduleDraft[task.id] ?? 0}
                        onChange={(e) => onIntervalChange(task.id, e.target.value)}
                      />
                    </div>
                    <Button
                      type="button"
                      size="sm"
                      variant="secondary"
                      className="whitespace-nowrap"
                      disabled={runTask.isPending}
                      onClick={() => {
                        setTaskMessage(null);
                        runTask.mutate(task.id);
                      }}
                    >
                      {runTask.isPending && runningTask === task.id ? (
                        <Loader2 className="size-4 animate-spin" />
                      ) : (
                        "Run now"
                      )}
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          </section>
        </div>
      ) : null}

      {subTab === "logs" ? (
        <section className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-5">
          <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">Server log tail</h3>
          {logsQuery.isLoading ? (
            <p className="mt-4 text-sm text-(--plum-muted)">Loading…</p>
          ) : logsQuery.isError ? (
            <p className="mt-4 text-sm text-red-300">{(logsQuery.error as Error).message}</p>
          ) : (
            <>
              {logsQuery.data?.hint ? (
                <p className="mt-3 text-sm text-amber-200/90">{logsQuery.data.hint}</p>
              ) : null}
              {logsQuery.data?.source ? (
                <p className="mt-2 text-xs text-(--plum-muted)">Source: {logsQuery.data.source}</p>
              ) : null}
              <pre className="mt-4 max-h-[min(28rem,55vh)] overflow-auto rounded-md border border-(--plum-border)/50 bg-black/40 p-3 font-mono text-[11px] leading-relaxed text-(--plum-text-secondary)">
                {(logsQuery.data?.lines ?? []).length === 0
                  ? "No log lines yet."
                  : (logsQuery.data?.lines ?? []).join("\n")}
              </pre>
            </>
          )}
        </section>
      ) : null}

      {subTab === "activity" ? (
        <section className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-5">
          <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">Now playing</h3>
          <p className="mt-2 text-sm text-(--plum-muted)">
            Active transcoding and direct playback sessions on this server (refreshes every few seconds).
          </p>
          {activityQuery.isLoading ? (
            <p className="mt-4 text-sm text-(--plum-muted)">Loading…</p>
          ) : activityQuery.isError ? (
            <p className="mt-4 text-sm text-red-300">{(activityQuery.error as Error).message}</p>
          ) : (activityQuery.data?.sessions ?? []).length === 0 ? (
            <p className="mt-6 text-sm text-(--plum-muted)">Nobody is playing anything right now.</p>
          ) : (
            <ul className="mt-4 divide-y divide-(--plum-border)/50 rounded-md border border-(--plum-border)/50">
              {(activityQuery.data?.sessions ?? []).map((s) => (
                <li key={s.sessionId} className="flex flex-col gap-1 px-3 py-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="min-w-0">
                    <p className="truncate font-medium text-(--plum-text)">{s.title || `Media ${s.mediaId}`}</p>
                    <p className="text-xs text-(--plum-muted)">
                      {s.userEmail || `User ${s.userId}`} · {s.delivery || "—"} · {s.status || "—"}
                    </p>
                  </div>
                  <p className="shrink-0 text-xs tabular-nums text-(--plum-muted)">
                    {s.durationSeconds > 0 ? `${Math.round(s.durationSeconds / 60)} min` : "—"}
                  </p>
                </li>
              ))}
            </ul>
          )}
        </section>
      ) : null}
    </div>
  );
}
