import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { BASE_URL, type MediaItem } from "../../api";
import { resolveBackdropUrl, resolvePosterUrl } from "@plum/shared";
import type { PlaybackKind } from "../../contexts/playerTypes";
import { UPNEXT_COUNTDOWN_SECONDS } from "./constants";

export function usePlaybackDockUpNext(options: {
  playNextInQueue: () => void;
  queueIndex: number;
  activeMode: PlaybackKind | null;
  isDockOpen: boolean;
}) {
  const { playNextInQueue, queueIndex, activeMode, isDockOpen } = options;
  const playNextInQueueRef = useRef(playNextInQueue);
  useEffect(() => {
    playNextInQueueRef.current = playNextInQueue;
  }, [playNextInQueue]);

  const [upNextTarget, setUpNextTarget] = useState<MediaItem | null>(null);
  const [upNextSecondsLeft, setUpNextSecondsLeft] = useState(UPNEXT_COUNTDOWN_SECONDS);

  const upNextIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const clearUpNextTimer = useCallback(() => {
    if (upNextIntervalRef.current != null) {
      clearInterval(upNextIntervalRef.current);
      upNextIntervalRef.current = null;
    }
  }, []);

  const dismissUpNext = useCallback(() => {
    clearUpNextTimer();
    setUpNextTarget(null);
  }, [clearUpNextTimer]);

  const confirmUpNextNow = useCallback(() => {
    clearUpNextTimer();
    setUpNextTarget(null);
    playNextInQueue();
  }, [clearUpNextTimer, playNextInQueue]);

  useEffect(() => {
    if (!upNextTarget) {
      clearUpNextTimer();
      return;
    }
    setUpNextSecondsLeft(UPNEXT_COUNTDOWN_SECONDS);
    let remaining = UPNEXT_COUNTDOWN_SECONDS;
    upNextIntervalRef.current = setInterval(() => {
      remaining -= 1;
      setUpNextSecondsLeft(remaining);
      if (remaining <= 0) {
        clearUpNextTimer();
        setUpNextTarget(null);
        playNextInQueueRef.current();
      }
    }, 1000);
    return () => clearUpNextTimer();
  }, [upNextTarget, clearUpNextTimer]);

  const prevQueueIndexRef = useRef(queueIndex);
  useEffect(() => {
    if (upNextTarget != null && queueIndex !== prevQueueIndexRef.current) {
      dismissUpNext();
    }
    prevQueueIndexRef.current = queueIndex;
  }, [queueIndex, upNextTarget, dismissUpNext]);

  useEffect(() => {
    if (activeMode !== "video" || !isDockOpen) {
      dismissUpNext();
    }
  }, [activeMode, dismissUpNext, isDockOpen]);

  const upNextBackdropUrl = useMemo(() => {
    if (!upNextTarget) return "";
    const fromBackdrop = resolveBackdropUrl(
      upNextTarget.backdrop_url,
      upNextTarget.backdrop_path,
      "original",
      BASE_URL,
    );
    if (fromBackdrop) return fromBackdrop;
    return (
      resolvePosterUrl(
        upNextTarget.show_poster_url,
        upNextTarget.show_poster_path,
        "original",
        BASE_URL,
      ) ||
      resolvePosterUrl(upNextTarget.poster_url, upNextTarget.poster_path, "original", BASE_URL)
    );
  }, [upNextTarget]);

  return {
    upNextTarget,
    setUpNextTarget,
    upNextSecondsLeft,
    dismissUpNext,
    confirmUpNextNow,
    upNextBackdropUrl,
  };
}
