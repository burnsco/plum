import { useEffect, useRef, type RefObject } from "react";
import { ignorePromise } from "@/lib/ignorePromise";

type UpNextActions = {
  dismissUpNext: () => void;
  confirmUpNextNow: () => void;
};

/** Escape / Enter while the “up next” overlay is visible (theater player). */
export function usePlaybackUpNextKeyboard(enabled: boolean, actions: UpNextActions): void {
  const ref = useRef(actions);
  ref.current = actions;
  useEffect(() => {
    if (!enabled) return;
    const onKeyDown = (event: KeyboardEvent) => {
      const tag = (event.target as HTMLElement)?.tagName;
      if (tag === "INPUT" || tag === "SELECT" || tag === "TEXTAREA") return;
      if (event.key === "Escape") {
        event.preventDefault();
        ref.current.dismissUpNext();
        return;
      }
      if (event.key === "Enter") {
        event.preventDefault();
        ref.current.confirmUpNextNow();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [enabled]);
}

export type FullscreenPlaybackKeyboardDeps = {
  playerRootRef: RefObject<HTMLElement | null>;
  videoRef: RefObject<HTMLVideoElement | null>;
  volume: number;
  muted: boolean;
  dismissDock: () => void;
  toggleBrowserFullscreen: () => void;
  togglePlayPause: () => void;
  seekTo: (seconds: number) => void;
  setVolume: (v: number) => void;
  setMuted: (m: boolean) => void;
  resetHideTimer: () => void;
  captureVideoProgressSnapshot: (element: HTMLVideoElement | null) => unknown;
  persistPlaybackProgress: (opts?: {
    force?: boolean;
    completed?: boolean;
    snapshot?: unknown;
  }) => void | Promise<void>;
};

/** Space, arrows, F, M, Escape in fullscreen / window player (ignores focused inputs). */
export function useFullscreenPlaybackKeyboard(enabled: boolean, deps: FullscreenPlaybackKeyboardDeps): void {
  const ref = useRef(deps);
  ref.current = deps;
  useEffect(() => {
    if (!enabled) return;
    const onKeyDown = (event: KeyboardEvent) => {
      const tag = (event.target as HTMLElement)?.tagName;
      if (tag === "INPUT" || tag === "SELECT" || tag === "TEXTAREA") return;

      const {
        playerRootRef,
        videoRef,
        volume,
        muted,
        dismissDock,
        toggleBrowserFullscreen,
        togglePlayPause,
        seekTo,
        setVolume,
        setMuted,
        resetHideTimer,
        captureVideoProgressSnapshot,
        persistPlaybackProgress,
      } = ref.current;

      switch (event.key) {
        case "Escape":
          if (document.fullscreenElement === playerRootRef.current) {
            ignorePromise(document.exitFullscreen(), "PlaybackDock:exitFullscreenShortcut");
          } else {
            const snapshot = captureVideoProgressSnapshot(videoRef.current);
            void persistPlaybackProgress({ force: true, snapshot });
            dismissDock();
          }
          break;
        case "f":
        case "F":
          event.preventDefault();
          void toggleBrowserFullscreen();
          break;
        case " ":
          event.preventDefault();
          togglePlayPause();
          resetHideTimer();
          break;
        case "ArrowLeft":
          event.preventDefault();
          seekTo(Math.max(0, (videoRef.current?.currentTime ?? 0) - 10));
          resetHideTimer();
          break;
        case "ArrowRight":
          event.preventDefault();
          seekTo((videoRef.current?.currentTime ?? 0) + 10);
          resetHideTimer();
          break;
        case "ArrowUp":
          event.preventDefault();
          setVolume(Math.min(1, volume + 0.1));
          resetHideTimer();
          break;
        case "ArrowDown":
          event.preventDefault();
          setVolume(Math.max(0, volume - 0.1));
          resetHideTimer();
          break;
        case "m":
        case "M":
          setMuted(!muted);
          resetHideTimer();
          break;
        default:
          break;
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [enabled]);
}
