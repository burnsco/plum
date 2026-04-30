import { useCallback, useState, type Dispatch, type RefObject, type SetStateAction } from "react";
import { rememberBlockedSubtitleKey } from "../../lib/playback/playbackDockSubtitleTracks";
import {
  buildSubtitleCues,
  consumeSubtitleResponseWithPartialUpdates,
  type SubtitleTrackOption,
} from "../../lib/playback/playerMedia";

/** Sidecar SRT/VTT etc. */
const SUBTITLE_LOAD_TIMEOUT_MS = 45_000;
/** Embedded tracks are transcoded server-side; allow longer before aborting. */
const EMBEDDED_SUBTITLE_LOAD_TIMEOUT_MS = 600_000;

export type LoadedSubtitleTrack = SubtitleTrackOption & {
  body: string;
};

export type SubtitleLoadState = "idle" | "loading" | "ready" | "blocked" | "timeout" | "error";
export type SubtitleLoadProgress =
  | "idle"
  | "requesting"
  | "receiving"
  | "extracting"
  | "parseable"
  | "ready"
  | "blocked"
  | "timeout"
  | "error";

export type SubtitleLoadDetail = {
  state: SubtitleLoadState;
  progress: SubtitleLoadProgress;
  bytesReceived: number;
  cueCount: number;
};

type UseSubtitleTransportParams = {
  activeMediaId: number | null;
  loadedSubtitleTracks: LoadedSubtitleTrack[];
  subtitleTrackRequests: SubtitleTrackOption[];
  subtitleLoadControllersRef: RefObject<Map<string, AbortController>>;
  blockedSubtitleRetryKeysRef: RefObject<Set<string>>;
  setPendingSubtitleKey: Dispatch<SetStateAction<string | null>>;
  setLoadedSubtitleTracks: Dispatch<SetStateAction<LoadedSubtitleTrack[]>>;
};

export function useSubtitleTransport({
  activeMediaId,
  loadedSubtitleTracks,
  subtitleTrackRequests,
  subtitleLoadControllersRef,
  blockedSubtitleRetryKeysRef,
  setPendingSubtitleKey,
  setLoadedSubtitleTracks,
}: UseSubtitleTransportParams) {
  const [subtitleLoadStateByKey, setSubtitleLoadStateByKey] = useState<
    Record<string, SubtitleLoadState>
  >({});
  const [subtitleLoadDetailByKey, setSubtitleLoadDetailByKey] = useState<
    Record<string, SubtitleLoadDetail>
  >({});

  const setTrackLoadDetail = useCallback((key: string, detail: SubtitleLoadDetail) => {
    setSubtitleLoadStateByKey((current) =>
      current[key] === detail.state ? current : { ...current, [key]: detail.state },
    );
    setSubtitleLoadDetailByKey((current) => {
      const previous = current[key];
      if (
        previous?.state === detail.state &&
        previous.progress === detail.progress &&
        previous.bytesReceived === detail.bytesReceived &&
        previous.cueCount === detail.cueCount
      ) {
        return current;
      }
      return { ...current, [key]: detail };
    });
  }, []);

  const setTrackLoadState = useCallback(
    (
      key: string,
      state: SubtitleLoadState,
      progress: SubtitleLoadProgress = state === "loading" ? "requesting" : state,
      bytesReceived = 0,
      cueCount = 0,
    ) => {
      setTrackLoadDetail(key, { state, progress, bytesReceived, cueCount });
    },
    [setTrackLoadDetail],
  );

  const clearSubtitleLoadState = useCallback(() => {
    setSubtitleLoadStateByKey({});
    setSubtitleLoadDetailByKey({});
  }, []);

  const ensureSubtitleTrackLoaded = useCallback(
    async (trackKey: string) => {
      if (trackKey === "off") return;
      if (loadedSubtitleTracks.some((track) => track.key === trackKey)) {
        setTrackLoadState(trackKey, "ready");
        return;
      }
      if (subtitleLoadControllersRef.current.has(trackKey)) return;
      if (blockedSubtitleRetryKeysRef.current.has(trackKey)) {
        setTrackLoadState(trackKey, "blocked");
        return;
      }
      const track = subtitleTrackRequests.find((candidate) => candidate.key === trackKey);
      if (!track) return;
      if (
        track.requiresBurn ||
        (track.assEligible && track.assSrc && track.preferredWebDeliveryMode === "ass")
      ) {
        setPendingSubtitleKey(null);
        setTrackLoadState(trackKey, "idle");
        return;
      }
      if (track.supported === false) {
        setPendingSubtitleKey(null);
        setTrackLoadState(trackKey, "error");
        return;
      }

      const controller = new AbortController();
      subtitleLoadControllersRef.current.set(trackKey, controller);
      let timedOut = false;
      const subtitleTimeoutMs = track.src.includes("/subtitles/embedded/")
        ? EMBEDDED_SUBTITLE_LOAD_TIMEOUT_MS
        : SUBTITLE_LOAD_TIMEOUT_MS;
      const timeoutId =
        typeof window === "undefined"
          ? null
          : window.setTimeout(() => {
              timedOut = true;
              controller.abort();
            }, subtitleTimeoutMs);

      try {
        setTrackLoadState(trackKey, "loading");
        const response = await fetch(track.src, {
          credentials: "include",
          signal: controller.signal,
        });
        if (!response.ok) {
          throw new Error(`Subtitle request failed: ${response.status}`);
        }
        let lastFlushedCueCount = 0;
        let lastFlushedBodyLen = 0;
        await consumeSubtitleResponseWithPartialUpdates(
          response,
          controller.signal,
          (bodyForState, streamDone) => {
            const cues = buildSubtitleCues(bodyForState);
            if (!streamDone) {
              if (cues.length === 0) {
                if (bodyForState.length > lastFlushedBodyLen) {
                  lastFlushedBodyLen = bodyForState.length;
                  setTrackLoadState(
                    trackKey,
                    "loading",
                    bodyForState.length > 0 ? "extracting" : "receiving",
                    bodyForState.length,
                    0,
                  );
                }
                return;
              }
              if (
                cues.length === lastFlushedCueCount &&
                bodyForState.length === lastFlushedBodyLen
              ) {
                return;
              }
              lastFlushedCueCount = cues.length;
              lastFlushedBodyLen = bodyForState.length;
            } else {
              lastFlushedCueCount = cues.length;
              lastFlushedBodyLen = bodyForState.length;
            }
            setLoadedSubtitleTracks((current) => {
              const rest = current.filter((candidate) => candidate.key !== track.key);
              return [...rest, { ...track, body: bodyForState }];
            });
            if (cues.length > 0) {
              setTrackLoadState(
                trackKey,
                streamDone ? "ready" : "loading",
                streamDone ? "ready" : "parseable",
                bodyForState.length,
                cues.length,
              );
            }
          },
        );
        blockedSubtitleRetryKeysRef.current.delete(track.key);
        setPendingSubtitleKey((current) => (current === track.key ? null : current));
        setTrackLoadState(trackKey, "ready", "ready", lastFlushedBodyLen, lastFlushedCueCount);
      } catch (error) {
        let loadError: unknown = error;
        if (
          (error instanceof DOMException && error.name === "AbortError") ||
          controller.signal.aborted
        ) {
          if (!timedOut) {
            return;
          }
          loadError = new Error("Subtitle request timed out");
        }
        console.error("[PlaybackDock] Subtitle load failed", {
          mediaId: activeMediaId,
          source: track.src,
          error: loadError,
        });
        setLoadedSubtitleTracks((current) =>
          current.filter((candidate) => candidate.key !== track.key),
        );
        rememberBlockedSubtitleKey(blockedSubtitleRetryKeysRef.current, track.key);
        setPendingSubtitleKey((current) => (current === track.key ? null : current));
        const timedOutError =
          loadError instanceof Error && loadError.message === "Subtitle request timed out";
        setTrackLoadState(trackKey, timedOutError ? "timeout" : "error");
      } finally {
        if (timeoutId != null) {
          window.clearTimeout(timeoutId);
        }
        subtitleLoadControllersRef.current.delete(trackKey);
      }
    },
    [
      activeMediaId,
      blockedSubtitleRetryKeysRef,
      loadedSubtitleTracks,
      setLoadedSubtitleTracks,
      setPendingSubtitleKey,
      setTrackLoadState,
      subtitleLoadControllersRef,
      subtitleTrackRequests,
    ],
  );

  return {
    clearSubtitleLoadState,
    ensureSubtitleTrackLoaded,
    setTrackLoadDetail,
    setTrackLoadState,
    subtitleLoadDetailByKey,
    subtitleLoadStateByKey,
  };
}
