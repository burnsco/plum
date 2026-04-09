import { useCallback, useEffect, useRef, useState } from "react";
import { ignorePromise } from "@/lib/ignorePromise";

const CONTROLS_HIDE_DELAY = 3000;

export function usePlaybackDockWindowChrome(isWindowPlayer: boolean) {
  const playerRootRef = useRef<HTMLElement | null>(null);
  const [controlsVisible, setControlsVisible] = useState(true);
  const hideTimerRef = useRef<ReturnType<typeof setTimeout>>(0);

  const resetHideTimer = useCallback(() => {
    setControlsVisible(true);
    clearTimeout(hideTimerRef.current);
    hideTimerRef.current = setTimeout(() => {
      setControlsVisible(false);
    }, CONTROLS_HIDE_DELAY);
  }, []);

  const [browserFullscreenActive, setBrowserFullscreenActive] = useState(false);

  const syncBrowserFullscreenState = useCallback(() => {
    setBrowserFullscreenActive(
      document.fullscreenElement === playerRootRef.current,
    );
  }, []);

  useEffect(() => {
    syncBrowserFullscreenState();
    const handleFullscreenChange = () => syncBrowserFullscreenState();
    document.addEventListener("fullscreenchange", handleFullscreenChange);
    return () =>
      document.removeEventListener("fullscreenchange", handleFullscreenChange);
  }, [syncBrowserFullscreenState]);

  const toggleBrowserFullscreen = useCallback(() => {
    if (document.fullscreenElement === playerRootRef.current) {
      ignorePromise(
        document.exitFullscreen(),
        "PlaybackDock:exitFullscreenToggle",
      );
      return;
    }
    if (!playerRootRef.current) return;
    const p = playerRootRef.current.requestFullscreen?.();
    if (p) ignorePromise(p, "PlaybackDock:requestFullscreen");
  }, []);

  const handleVideoDoubleClick = useCallback(() => {
    void toggleBrowserFullscreen();
    resetHideTimer();
  }, [resetHideTimer, toggleBrowserFullscreen]);

  useEffect(() => {
    if (!isWindowPlayer) {
      setControlsVisible(true);
      clearTimeout(hideTimerRef.current);
      return;
    }
    resetHideTimer();
    return () => clearTimeout(hideTimerRef.current);
  }, [isWindowPlayer, resetHideTimer]);

  const handleFullscreenMouseMove = useCallback(() => {
    if (isWindowPlayer) resetHideTimer();
  }, [isWindowPlayer, resetHideTimer]);

  const handleOverlayMouseEnter = useCallback(() => {
    clearTimeout(hideTimerRef.current);
    setControlsVisible(true);
  }, []);

  const setPlayerRootNode = useCallback((node: HTMLElement | null) => {
    playerRootRef.current = node;
  }, []);

  return {
    playerRootRef,
    setPlayerRootNode,
    controlsVisible,
    resetHideTimer,
    browserFullscreenActive,
    toggleBrowserFullscreen,
    handleVideoDoubleClick,
    handleFullscreenMouseMove,
    handleOverlayMouseEnter,
  };
}
