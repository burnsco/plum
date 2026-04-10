import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ChangeEvent,
  type PointerEvent,
  type RefObject,
} from "react";

type UsePlaybackDockSeekControlsArgs = {
  activeItemId: number | null;
  playbackCurrentTime: number;
  playbackStreamOffsetSeconds: number;
  progressMax: number;
  seekTo: (seconds: number) => void;
  videoRef: RefObject<HTMLVideoElement | null>;
  resetHideTimer: () => void;
};

export function usePlaybackDockSeekControls({
  activeItemId,
  playbackCurrentTime,
  playbackStreamOffsetSeconds,
  progressMax,
  seekTo,
  videoRef,
  resetHideTimer,
}: UsePlaybackDockSeekControlsArgs) {
  const [seekPreviewSec, setSeekPreviewSec] = useState<number | null>(null);
  const seekScrubActiveRef = useRef(false);
  const seekPreviewValueRef = useRef<number | null>(null);
  const seekSliderRef = useRef<HTMLInputElement | null>(null);
  const scrubWindowListenersRef = useRef<(() => void) | null>(null);

  const removeScrubWindowListeners = useCallback(() => {
    scrubWindowListenersRef.current?.();
    scrubWindowListenersRef.current = null;
  }, []);

  useEffect(
    () => () => {
      scrubWindowListenersRef.current?.();
      scrubWindowListenersRef.current = null;
    },
    [],
  );

  const seekToRef = useRef(seekTo);
  useEffect(() => {
    seekToRef.current = seekTo;
  }, [seekTo]);

  const finishSeekScrub = useCallback(
    (input: HTMLInputElement | null) => {
      if (!seekScrubActiveRef.current) return;
      seekScrubActiveRef.current = false;
      removeScrubWindowListeners();
      const el = input ?? seekSliderRef.current;
      const parsed = el ? Number(el.value) : NaN;
      const preview = seekPreviewValueRef.current;
      seekPreviewValueRef.current = null;
      setSeekPreviewSec(null);
      const v = Number.isFinite(parsed)
        ? parsed
        : preview != null && Number.isFinite(preview)
          ? preview
          : null;
      if (v != null && Number.isFinite(v)) {
        seekToRef.current(v);
      }
    },
    [removeScrubWindowListeners],
  );

  const handleSeekSliderPointerDown = useCallback(
    (e: PointerEvent<HTMLInputElement>) => {
      try {
        e.currentTarget.setPointerCapture(e.pointerId);
      } catch {
        /* ignore */
      }
      removeScrubWindowListeners();

      seekScrubActiveRef.current = true;
      const pointerId = e.pointerId;

      const onWindowPointerEnd = (ev: Event) => {
        if (!(ev instanceof PointerEvent) || ev.pointerId !== pointerId) return;
        removeScrubWindowListeners();
        queueMicrotask(() => {
          finishSeekScrub(seekSliderRef.current);
        });
      };

      window.addEventListener("pointerup", onWindowPointerEnd);
      window.addEventListener("pointercancel", onWindowPointerEnd);
      scrubWindowListenersRef.current = () => {
        window.removeEventListener("pointerup", onWindowPointerEnd);
        window.removeEventListener("pointercancel", onWindowPointerEnd);
      };
    },
    [finishSeekScrub, removeScrubWindowListeners],
  );

  const handleSeekSliderChange = useCallback(
    (
      event:
        | ChangeEvent<HTMLInputElement>
        | { currentTarget: HTMLInputElement },
    ) => {
      const next = Number(event.currentTarget.value);
      if (!Number.isFinite(next)) return;
      seekPreviewValueRef.current = next;
      setSeekPreviewSec(next);
      if (!seekScrubActiveRef.current) {
        seekToRef.current(next);
      }
    },
    [],
  );

  const seekRelativeSeconds = useCallback(
    (delta: number) => {
      removeScrubWindowListeners();
      seekScrubActiveRef.current = false;
      seekPreviewValueRef.current = null;
      setSeekPreviewSec(null);
      const cap =
        progressMax > 0 && Number.isFinite(progressMax)
          ? progressMax
          : Number.POSITIVE_INFINITY;
      const el = videoRef.current;
      const t =
        el != null && Number.isFinite(el.currentTime)
          ? el.currentTime
            + playbackStreamOffsetSeconds
          : playbackCurrentTime;
      seekTo(Math.max(0, Math.min(cap, t + delta)));
      resetHideTimer();
    },
    [
      playbackCurrentTime,
      playbackStreamOffsetSeconds,
      progressMax,
      removeScrubWindowListeners,
      resetHideTimer,
      seekTo,
      videoRef,
    ],
  );

  useEffect(() => {
    removeScrubWindowListeners();
    seekScrubActiveRef.current = false;
    seekPreviewValueRef.current = null;
    setSeekPreviewSec(null);
  }, [activeItemId, removeScrubWindowListeners]);

  const seekSliderDisplayValue = Math.min(
    seekPreviewSec !== null ? seekPreviewSec : playbackCurrentTime,
    progressMax || 0,
  );
  const seekTimeLabelSec =
    seekPreviewSec !== null ? seekPreviewSec : playbackCurrentTime;

  return {
    seekSliderRef,
    seekSliderDisplayValue,
    seekTimeLabelSec,
    handleSeekSliderPointerDown,
    handleSeekSliderChange,
    seekRelativeSeconds,
  };
}
