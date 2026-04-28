import { Suspense, lazy } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { DownloadCompletionNotifier } from "@/components/DownloadCompletionNotifier";
import { PageRouteSkeleton, PlaybackDockSkeleton } from "@/components/loading/PlumLoadingSkeletons";
import { usePlayerSession } from "@/contexts/PlayerContext";
import { RouteErrorBoundary } from "./RouteErrorBoundary";
import { TopBar } from "./TopBar";
import { Sidebar } from "./Sidebar";
import { Toaster } from "./ui/sonner";

const PlaybackDock = lazy(() =>
  import("./PlaybackDock").then((m) => ({ default: m.PlaybackDock })),
);

function PlaybackDockSlot() {
  const { activeItem, isDockOpen } = usePlayerSession();
  if (activeItem == null && !isDockOpen) {
    return null;
  }
  return (
    <Suspense fallback={<PlaybackDockSkeleton />}>
      <PlaybackDock />
    </Suspense>
  );
}

export function MainLayout() {
  const location = useLocation();
  return (
    <div className="flex h-screen overflow-hidden flex-col">
      <DownloadCompletionNotifier />
      <Toaster />
      <TopBar />
      <div className="flex flex-1 min-h-0">
        <Sidebar />
        <main className="flex min-w-0 flex-1 flex-col bg-(--plum-main-bg)">
          <section className="main-content flex-1 overflow-auto bg-(--plum-main-bg) p-4 md:p-6">
            <RouteErrorBoundary key={location.pathname}>
              <Suspense fallback={<PageRouteSkeleton />}>
                <Outlet />
              </Suspense>
            </RouteErrorBoundary>
          </section>
        </main>
      </div>
      <PlaybackDockSlot />
    </div>
  );
}
