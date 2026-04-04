import { Outlet } from "react-router-dom";
import { DownloadCompletionNotifier } from "@/components/DownloadCompletionNotifier";
import { LibraryReadyNotifier } from "@/components/LibraryReadyNotifier";
import { PlaybackDock } from "./PlaybackDock";
import { TopBar } from "./TopBar";
import { Sidebar } from "./Sidebar";
import { Toaster } from "./ui/sonner";

export function MainLayout() {
  return (
    <div className="flex h-screen overflow-hidden flex-col">
      <DownloadCompletionNotifier />
      <LibraryReadyNotifier />
      <Toaster />
      <TopBar />
      <div className="flex flex-1 min-h-0">
        <Sidebar />
        <main className="flex min-w-0 flex-1 flex-col bg-(--plum-main-bg)">
          <section className="main-content flex-1 overflow-auto bg-(--plum-main-bg) p-4 md:p-6">
            <Outlet />
          </section>
        </main>
      </div>
      <PlaybackDock />
    </div>
  );
}
