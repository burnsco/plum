import { useMemo } from "react";
import type Hls from "hls.js";
import type { EmbeddedSubtitleDeliveryMode } from "../../api";
import {
  findHlsSubtitleTrackIndexForLogicalId,
  type SubtitleTrackOption,
} from "../../lib/playback/playerMedia";
import type { SubtitleLoadState } from "./useSubtitleTransport";

export type WebSubtitleRenderer = "none" | "hls_native" | "manual_vtt" | "ass" | "burn_in";

export type NormalizedSubtitleSelection = {
  selectedTrack: SubtitleTrackOption | null;
  logicalId: string | null;
  origin: "external" | "embedded" | null;
  renderer: WebSubtitleRenderer;
  selectedDeliveryMode: EmbeddedSubtitleDeliveryMode | null;
  loadState: SubtitleLoadState;
  activeAssSource: string | null;
  manualTrackKey: string | null;
};

type ResolveWebSubtitleSelectionParams = {
  selectedSubtitleKey: string;
  subtitleTrackRequests: SubtitleTrackOption[];
  subtitleLoadStateByKey: Record<string, SubtitleLoadState>;
  burnEmbeddedSubtitleStreamIndex: number | null;
  videoSourceIsHls: boolean;
  hls: Hls | null;
  resolutionVersion: number;
};

export function resolveWebSubtitleSelection({
  selectedSubtitleKey,
  subtitleTrackRequests,
  subtitleLoadStateByKey,
  burnEmbeddedSubtitleStreamIndex,
  videoSourceIsHls,
  hls,
  resolutionVersion: _resolutionVersion,
}: ResolveWebSubtitleSelectionParams): NormalizedSubtitleSelection {
  if (selectedSubtitleKey === "off") {
    return {
      selectedTrack: null,
      logicalId: null,
      origin: null,
      renderer: "none",
      selectedDeliveryMode: null,
      loadState: "idle",
      activeAssSource: null,
      manualTrackKey: null,
    };
  }

  const selectedTrack =
    subtitleTrackRequests.find((track) => track.key === selectedSubtitleKey) ?? null;
  if (selectedTrack == null) {
    return {
      selectedTrack: null,
      logicalId: null,
      origin: null,
      renderer: "none",
      selectedDeliveryMode: null,
      loadState: "idle",
      activeAssSource: null,
      manualTrackKey: null,
    };
  }

  const burnKey =
    burnEmbeddedSubtitleStreamIndex != null
      ? `emb:${burnEmbeddedSubtitleStreamIndex}`
      : null;
  const loadState = subtitleLoadStateByKey[selectedTrack.key] ?? "idle";

  if (selectedTrack.requiresBurn === true || (burnKey != null && selectedTrack.key === burnKey)) {
    return {
      selectedTrack,
      logicalId: selectedTrack.logicalId ?? null,
      origin: selectedTrack.origin ?? null,
      renderer: "burn_in",
      selectedDeliveryMode: "burn_in",
      loadState,
      activeAssSource: null,
      manualTrackKey: null,
    };
  }

  if (
    videoSourceIsHls &&
    hls != null &&
    findHlsSubtitleTrackIndexForLogicalId(hls, selectedTrack.key) >= 0
  ) {
    return {
      selectedTrack,
      logicalId: selectedTrack.logicalId ?? null,
      origin: selectedTrack.origin ?? null,
      renderer: "hls_native",
      selectedDeliveryMode: "hls_vtt",
      loadState,
      activeAssSource: null,
      manualTrackKey: null,
    };
  }

  if (selectedTrack.assEligible && selectedTrack.assSrc) {
    return {
      selectedTrack,
      logicalId: selectedTrack.logicalId ?? null,
      origin: selectedTrack.origin ?? null,
      renderer: "ass",
      selectedDeliveryMode: "ass",
      loadState,
      activeAssSource: selectedTrack.assSrc,
      manualTrackKey: null,
    };
  }

  return {
    selectedTrack,
    logicalId: selectedTrack.logicalId ?? null,
    origin: selectedTrack.origin ?? null,
    renderer: "manual_vtt",
    selectedDeliveryMode:
      selectedTrack.preferredWebDeliveryMode === "burn_in" ||
      selectedTrack.preferredWebDeliveryMode === "hls_vtt"
        ? "direct_vtt"
        : (selectedTrack.preferredWebDeliveryMode ?? "direct_vtt"),
    loadState,
    activeAssSource: null,
    manualTrackKey: selectedTrack.key,
  };
}

type UseSubtitleControllerParams = ResolveWebSubtitleSelectionParams;

export function useSubtitleController({
  selectedSubtitleKey,
  subtitleTrackRequests,
  subtitleLoadStateByKey,
  burnEmbeddedSubtitleStreamIndex,
  videoSourceIsHls,
  hls,
  resolutionVersion,
}: UseSubtitleControllerParams): NormalizedSubtitleSelection {
  return useMemo(
    () =>
      resolveWebSubtitleSelection({
        selectedSubtitleKey,
        subtitleTrackRequests,
        subtitleLoadStateByKey,
        burnEmbeddedSubtitleStreamIndex,
        videoSourceIsHls,
        hls,
        resolutionVersion,
      }),
    [
      burnEmbeddedSubtitleStreamIndex,
      hls,
      resolutionVersion,
      selectedSubtitleKey,
      subtitleLoadStateByKey,
      subtitleTrackRequests,
      videoSourceIsHls,
    ],
  );
}
