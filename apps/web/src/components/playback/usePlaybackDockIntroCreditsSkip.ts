import type { IntroSkipMode, MediaItem } from "../../api";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

const INTRO_END_MARGIN_SEC = 0.5;

export function usePlaybackDockIntroCreditsSkip(options: {
  activeItemId: number | null;
  activeItem: MediaItem | null;
  introSkipMode: IntroSkipMode;
  isVideo: boolean;
  seekTo: (seconds: number) => void;
  playbackCurrentTime: number;
}) {
  const { activeItemId, activeItem, introSkipMode, isVideo, seekTo, playbackCurrentTime } =
    options;

  const introEndSec = useMemo(() => {
    const end = activeItem?.intro_end_seconds;
    if (end == null || !Number.isFinite(end) || end <= 0) {
      return null;
    }
    return end;
  }, [activeItem?.intro_end_seconds]);

  const introStartSec = useMemo(() => {
    const s = activeItem?.intro_start_seconds;
    if (s != null && Number.isFinite(s) && s >= 0) {
      return s;
    }
    return 0;
  }, [activeItem?.intro_start_seconds]);

  const creditsEndSec = useMemo(() => {
    const end = activeItem?.credits_end_seconds;
    if (end == null || !Number.isFinite(end) || end <= 0) {
      return null;
    }
    return end;
  }, [activeItem?.credits_end_seconds]);

  const creditsStartSec = useMemo(() => {
    const s = activeItem?.credits_start_seconds;
    if (s != null && Number.isFinite(s) && s >= 0) {
      return s;
    }
    return null;
  }, [activeItem?.credits_start_seconds]);

  const creditsWindowOk =
    creditsStartSec != null &&
    creditsEndSec != null &&
    creditsEndSec > creditsStartSec;

  const introSkipStateRef = useRef({ consumedAuto: false, suppressed: false, lastTime: 0 });
  const [introButtonDismissed, setIntroButtonDismissed] = useState(false);

  const creditsSkipStateRef = useRef({ consumedAuto: false, suppressed: false, lastTime: 0 });
  const [creditsButtonDismissed, setCreditsButtonDismissed] = useState(false);

  useEffect(() => {
    introSkipStateRef.current = { consumedAuto: false, suppressed: false, lastTime: 0 };
    setIntroButtonDismissed(false);
    creditsSkipStateRef.current = { consumedAuto: false, suppressed: false, lastTime: 0 };
    setCreditsButtonDismissed(false);
  }, [activeItemId]);

  const handleSkipIntroClick = useCallback(() => {
    if (introEndSec == null) return;
    seekTo(introEndSec);
    setIntroButtonDismissed(true);
  }, [introEndSec, seekTo]);

  const handleSkipCreditsClick = useCallback(() => {
    if (creditsEndSec == null) return;
    seekTo(creditsEndSec);
    setCreditsButtonDismissed(true);
  }, [creditsEndSec, seekTo]);

  const processIntroSkip = useCallback(
    (video: HTMLVideoElement) => {
      const mode = introSkipMode;
      const end = introEndSec;
      if (mode === "off" || end == null || !isVideo) {
        return;
      }
      const st = introStartSec;
      const t = Number.isFinite(video.currentTime) ? video.currentTime : 0;
      const state = introSkipStateRef.current;
      if (t < state.lastTime - 1.0 && t < end) {
        state.consumedAuto = false;
        state.suppressed = false;
      }
      if (state.lastTime >= end && t < end - 0.25) {
        state.suppressed = true;
      }
      state.lastTime = t;
      if (t < st || t >= end - INTRO_END_MARGIN_SEC) {
        return;
      }
      if (mode === "auto" && !state.consumedAuto && !state.suppressed && video.readyState >= 2) {
        state.consumedAuto = true;
        seekTo(end);
      }
    },
    [introEndSec, introStartSec, isVideo, introSkipMode, seekTo],
  );

  const processCreditsSkip = useCallback(
    (video: HTMLVideoElement) => {
      const mode = introSkipMode;
      if (
        mode === "off" ||
        !creditsWindowOk ||
        !isVideo ||
        creditsStartSec == null ||
        creditsEndSec == null
      ) {
        return;
      }
      const st = creditsStartSec;
      const end = creditsEndSec;
      const t = Number.isFinite(video.currentTime) ? video.currentTime : 0;
      const state = creditsSkipStateRef.current;
      if (t < state.lastTime - 1.0 && t < end) {
        state.consumedAuto = false;
        state.suppressed = false;
      }
      if (state.lastTime >= end && t < end - 0.25) {
        state.suppressed = true;
      }
      state.lastTime = t;
      if (t < st || t >= end - INTRO_END_MARGIN_SEC) {
        return;
      }
      if (mode === "auto" && !state.consumedAuto && !state.suppressed && video.readyState >= 2) {
        state.consumedAuto = true;
        seekTo(end);
      }
    },
    [
      creditsEndSec,
      creditsStartSec,
      creditsWindowOk,
      isVideo,
      introSkipMode,
      seekTo,
    ],
  );

  const showSkipIntroControl =
    isVideo &&
    introEndSec != null &&
    introSkipMode !== "off" &&
    !introButtonDismissed &&
    playbackCurrentTime >= introStartSec &&
    playbackCurrentTime < introEndSec - INTRO_END_MARGIN_SEC;

  const showSkipCreditsControl =
    isVideo &&
    creditsWindowOk &&
    creditsStartSec != null &&
    creditsEndSec != null &&
    introSkipMode !== "off" &&
    !creditsButtonDismissed &&
    playbackCurrentTime >= creditsStartSec &&
    playbackCurrentTime < creditsEndSec - INTRO_END_MARGIN_SEC;

  return {
    processIntroSkip,
    processCreditsSkip,
    showSkipIntroControl,
    showSkipCreditsControl,
    handleSkipIntroClick,
    handleSkipCreditsClick,
  };
}
