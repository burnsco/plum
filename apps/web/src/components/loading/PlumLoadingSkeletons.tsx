import { Skeleton } from "@/components/ui/skeleton";

const PAGE_GRID_PLACEHOLDER_KEYS = [
  "pg-a",
  "pg-b",
  "pg-c",
  "pg-d",
  "pg-e",
  "pg-f",
  "pg-g",
  "pg-h",
  "pg-i",
  "pg-j",
  "pg-k",
  "pg-l",
  "pg-m",
  "pg-n",
  "pg-o",
  "pg-p",
] as const;

/** Full-screen auth-style placeholder while onboarding/login chunks load. */
export function AuthScreenSkeleton() {
  return (
    <main className="auth-screen">
      <div className="auth-card flex w-full max-w-md flex-col gap-4">
        <Skeleton className="mx-auto h-10 w-10 rounded-full" />
        <Skeleton className="h-6 w-3/5" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-4/5" />
        <Skeleton className="mt-2 h-10 w-full rounded-[var(--radius-md)]" />
      </div>
    </main>
  );
}

/** Main content area while lazy route chunks load (inside MainLayout). */
export function PageRouteSkeleton() {
  return (
    <div className="mx-auto w-full max-w-[var(--page-max-width)] animate-in fade-in duration-200">
      <div className="mb-6 flex flex-col gap-2">
        <Skeleton className="h-8 w-48 max-w-[60%]" />
        <Skeleton className="h-4 w-96 max-w-full" />
      </div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {PAGE_GRID_PLACEHOLDER_KEYS.map((key) => (
          <div key={key} className="flex flex-col gap-2">
            <Skeleton className="aspect-2/3 w-full rounded-[var(--radius-lg)]" />
            <Skeleton className="h-4 w-4/5" />
            <Skeleton className="h-3 w-1/2" />
          </div>
        ))}
      </div>
    </div>
  );
}

/** Poster grid placeholder (discover / library home). */
export function MediaGridSkeleton({ count = 8 }: { count?: number }) {
  const keys = PAGE_GRID_PLACEHOLDER_KEYS.slice(0, count);
  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      {keys.map((key) => (
        <div key={key} className="flex flex-col gap-2">
          <Skeleton className="aspect-2/3 w-full rounded-[var(--radius-lg)]" />
          <Skeleton className="h-4 w-4/5" />
          <Skeleton className="h-3 w-1/2" />
        </div>
      ))}
    </div>
  );
}

/** Hero + meta rows for movie/show detail routes. */
export function DetailViewSkeleton() {
  return (
    <div className="flex flex-col gap-8 lg:flex-row">
      <Skeleton className="aspect-2/3 w-full max-w-[18rem] shrink-0 rounded-[var(--radius-lg)] lg:max-w-[20rem]" />
      <div className="min-w-0 flex-1 space-y-4">
        <Skeleton className="h-9 w-2/3 max-w-md" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-5/6" />
        <div className="flex flex-wrap gap-2 pt-2">
          <Skeleton className="h-8 w-24 rounded-full" />
          <Skeleton className="h-8 w-28 rounded-full" />
        </div>
        <Skeleton className="h-10 w-40 rounded-[var(--radius-md)]" />
      </div>
    </div>
  );
}

/** Bottom dock chrome while PlaybackDock chunk loads. */
export function PlaybackDockSkeleton() {
  return (
    <div
      className="pointer-events-none flex shrink-0 border-t border-(--plum-chrome-border) bg-(--plum-sidebar-bg) px-4 py-3"
      role="status"
      aria-live="polite"
      aria-label="Loading player"
    >
      <div className="flex w-full items-center gap-4">
        <Skeleton className="size-14 shrink-0 rounded-[var(--radius-md)]" />
        <div className="min-w-0 flex-1 space-y-2">
          <Skeleton className="h-4 w-1/3 max-w-[12rem]" />
          <Skeleton className="h-3 w-1/4 max-w-[8rem]" />
        </div>
        <Skeleton className="hidden h-9 w-32 rounded-[var(--radius-md)] sm:block" />
      </div>
    </div>
  );
}
