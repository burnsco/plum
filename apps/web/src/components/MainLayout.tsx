import { Outlet } from "react-router-dom";
import { usePlayer } from "@/contexts/PlayerContext";
import { PlaybackDock } from "./PlaybackDock";
import { TopBar } from "./TopBar";
import { Sidebar } from "./Sidebar";

export function MainLayout() {
  const { activeMode, isDockOpen, viewMode } = usePlayer();
  const reserveDockSpace = isDockOpen && activeMode === "music" && viewMode === "docked";

  return (
    <div className="flex h-screen overflow-hidden flex-col bg-transparent">
      <TopBar />
      <div className="flex flex-1 min-h-0">
        <Sidebar />
        <main className="flex min-w-0 flex-1 flex-col">
          <section
            className={`main-content flex-1 overflow-auto px-4 py-4 md:px-6 md:py-5 xl:px-8 ${reserveDockSpace ? "main-content--with-dock" : ""}`}
          >
            <div className="app-shell__content mx-auto flex min-h-full w-full max-w-[var(--page-max-width)] flex-col">
              <Outlet />
            </div>
          </section>
        </main>
      </div>
      <PlaybackDock />
    </div>
  );
}
