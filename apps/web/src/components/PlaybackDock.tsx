import { usePlaybackDockController } from "./playback/usePlaybackDockController";

/** Video/music dock: rendering only; session, queue, tracks, and progress live in `usePlaybackDockController`. */
export function PlaybackDock() {
  return usePlaybackDockController();
}
