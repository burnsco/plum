import { useEffect, type Dispatch, type RefObject, type SetStateAction } from "react";
import Hls from "hls.js";
import { ignorePromise } from "@/lib/ignorePromise";
import {
  findHlsSubtitleTrackIndexForPlumKey,
  formatHlsErrorMessage,
} from "@/lib/playback/playerMedia";
import type { HlsErrorData, SubtitleTrackOption } from "@/lib/playback/playerMedia";

type LoadedSubtitleTrack = SubtitleTrackOption & { body: string };

export type UseHlsAttachmentParams = {
  isVideo: boolean;
  activeItemId: number | null;
  videoSourceUrl: string;
  videoSourceIsHls: boolean;
  videoAttachmentVersion: number;
  videoRef: RefObject<HTMLVideoElement | null>;
  hlsRef: RefObject<Hls | null>;
  seekToAfterReloadRef: RefObject<number | null>;
  setHlsStatusMessage: (message: string) => void;
  markSubtitleReady: () => void;
  maybeRecoverInitialBufferGap: (video: HTMLVideoElement | null) => boolean;
  mediaRecoveryAttemptsRef: RefObject<number>;
  networkRecoveryAttemptsRef: RefObject<number>;
  burnEmbeddedSubtitleStreamIndex: number | null;
  selectedSubtitleKey: string;
  subtitleTrackRequests: SubtitleTrackOption[];
  subtitleReadyVersion: number;
  subtitleLoadControllersRef: RefObject<Map<string, AbortController>>;
  setLoadedSubtitleTracks: Dispatch<SetStateAction<LoadedSubtitleTrack[]>>;
};

/**
 * Attaches Hls.js to the session video element, handles fatal/non-fatal errors,
 * and syncs native HLS subtitle track selection with Plum track keys.
 */
export function useHlsAttachment({
  isVideo,
  activeItemId,
  videoSourceUrl,
  videoSourceIsHls,
  videoAttachmentVersion,
  videoRef,
  hlsRef,
  seekToAfterReloadRef,
  setHlsStatusMessage,
  markSubtitleReady,
  maybeRecoverInitialBufferGap,
  mediaRecoveryAttemptsRef,
  networkRecoveryAttemptsRef,
  burnEmbeddedSubtitleStreamIndex,
  selectedSubtitleKey,
  subtitleTrackRequests,
  subtitleReadyVersion,
  subtitleLoadControllersRef,
  setLoadedSubtitleTracks,
}: UseHlsAttachmentParams): void {
  useEffect(() => {
    if (hlsRef.current != null) {
      hlsRef.current.destroy();
      hlsRef.current = null;
    }

    const video = videoRef.current;
    if (!isVideo || !video) return;

    if (!videoSourceUrl) {
      video.removeAttribute("src");
      video.load();
      return;
    }

    if (!videoSourceIsHls || !Hls.isSupported()) {
      video.src = videoSourceUrl;
      return;
    }

    const hls = new Hls({
      enableWorker: true,
      backBufferLength: 90,
      maxBufferLength: 60,
      maxMaxBufferLength: 120,
      startFragPrefetch: true,
      startPosition: seekToAfterReloadRef.current !== null ? seekToAfterReloadRef.current : -1,
      xhrSetup: (xhr) => {
        xhr.withCredentials = true;
      },
    });
    hlsRef.current = hls;
    hls.on(Hls.Events.MANIFEST_PARSED, () => {
      setHlsStatusMessage("");
      mediaRecoveryAttemptsRef.current = 0;
      networkRecoveryAttemptsRef.current = 0;
      markSubtitleReady();
    });
    hls.on(Hls.Events.ERROR, (_event, data: HlsErrorData) => {
      const formattedError = formatHlsErrorMessage(data);
      const isRecoverableGapError =
        !data.fatal &&
        (data.details === "bufferStalledError" || data.details === "bufferSeekOverHole");
      if (!isRecoverableGapError) {
        console.error("[PlaybackDock] HLS error", {
          mediaId: activeItemId,
          source: videoSourceUrl,
          fatal: data.fatal,
          type: data.type,
          details: data.details,
          error: data.error,
        });
      }

      if (!data.fatal) {
        if (data.details === "bufferStalledError") {
          const el = videoRef.current;
          if (maybeRecoverInitialBufferGap(el)) {
            setHlsStatusMessage("Resyncing playback...");
            if (el) ignorePromise(el.play(), "useHlsAttachment:hlsBufferStallResync");
          }
        }
        return;
      }

      if (data.type === Hls.ErrorTypes.NETWORK_ERROR && networkRecoveryAttemptsRef.current < 2) {
        networkRecoveryAttemptsRef.current += 1;
        setHlsStatusMessage("Reconnecting stream...");
        hls.startLoad();
        return;
      }

      if (data.type === Hls.ErrorTypes.MEDIA_ERROR && mediaRecoveryAttemptsRef.current < 2) {
        mediaRecoveryAttemptsRef.current += 1;
        setHlsStatusMessage("Recovering playback...");
        hls.recoverMediaError();
        return;
      }

      setHlsStatusMessage(`Stream error: ${formattedError}`);
    });
    hls.loadSource(videoSourceUrl);
    hls.attachMedia(video);

    return () => {
      hls.destroy();
      if (hlsRef.current === hls) {
        hlsRef.current = null;
      }
    };
  }, [
    activeItemId,
    hlsRef,
    isVideo,
    markSubtitleReady,
    maybeRecoverInitialBufferGap,
    mediaRecoveryAttemptsRef,
    networkRecoveryAttemptsRef,
    seekToAfterReloadRef,
    setHlsStatusMessage,
    videoAttachmentVersion,
    videoRef,
    videoSourceIsHls,
    videoSourceUrl,
  ]);

  useEffect(() => {
    if (!videoSourceIsHls) return;
    const hls = hlsRef.current;
    if (!hls) return;
    const burnIdx = burnEmbeddedSubtitleStreamIndex;
    if (burnIdx != null && selectedSubtitleKey === `emb-${burnIdx}`) {
      hls.subtitleTrack = -1;
      hls.subtitleDisplay = false;
      return;
    }
    if (selectedSubtitleKey === "off") {
      hls.subtitleTrack = -1;
      return;
    }
    const selectedReq = subtitleTrackRequests.find((t) => t.key === selectedSubtitleKey);
    if (selectedReq?.assEligible && selectedReq.assSrc) {
      // ASS is rendered by JASSUB; the same logical track may still appear as HLS WebVTT.
      hls.subtitleTrack = -1;
      hls.subtitleDisplay = false;
      return;
    }
    const idx = findHlsSubtitleTrackIndexForPlumKey(hls, selectedSubtitleKey);
    if (idx >= 0) {
      hls.subtitleTrack = idx;
      hls.subtitleDisplay = true;
    } else {
      hls.subtitleTrack = -1;
    }
  }, [
    burnEmbeddedSubtitleStreamIndex,
    hlsRef,
    selectedSubtitleKey,
    subtitleReadyVersion,
    subtitleTrackRequests,
    videoAttachmentVersion,
    videoSourceIsHls,
  ]);

  useEffect(() => {
    if (!videoSourceIsHls || selectedSubtitleKey === "off") return;
    const hls = hlsRef.current;
    if (!hls) return;
    const idx = findHlsSubtitleTrackIndexForPlumKey(hls, selectedSubtitleKey);
    if (idx < 0) return;
    const controller = subtitleLoadControllersRef.current.get(selectedSubtitleKey);
    if (controller) {
      controller.abort();
      subtitleLoadControllersRef.current.delete(selectedSubtitleKey);
    }
    setLoadedSubtitleTracks((current) =>
      current.filter((candidate) => candidate.key !== selectedSubtitleKey),
    );
  }, [
    hlsRef,
    selectedSubtitleKey,
    setLoadedSubtitleTracks,
    subtitleLoadControllersRef,
    subtitleReadyVersion,
    videoSourceIsHls,
  ]);
}
