import {
  embeddedSubtitleAssUrl,
  embeddedSubtitleNeedsWebBurnIn,
  embeddedSubtitleUrl,
  externalSubtitleAssUrl,
  externalSubtitleUrl,
} from "@plum/shared";
import {
  BASE_URL,
  type EmbeddedAudioTrack,
  type EmbeddedSubtitle,
  type PlaybackTrackMetadata,
  type Subtitle,
} from "../../api";
import type { SubtitleTrackOption } from "./playerMedia";

export type PlaybackTrackMetadataInput = {
  subtitles?: readonly Subtitle[];
  embeddedSubtitles?: readonly EmbeddedSubtitle[];
  embeddedAudioTracks?: readonly EmbeddedAudioTrack[];
};

export type PlaybackTrackSource = PlaybackTrackMetadataInput & {
  mediaId: number;
};

/** Avoid unbounded growth of failed subtitle keys over a long session (oldest entries drop first). */
export const MAX_BLOCKED_SUBTITLE_RETRY_KEYS = 64;

export function rememberBlockedSubtitleKey(set: Set<string>, key: string) {
  set.add(key);
  while (set.size > MAX_BLOCKED_SUBTITLE_RETRY_KEYS) {
    const oldest = set.keys().next().value;
    if (oldest === undefined) break;
    set.delete(oldest);
  }
}

export function embeddedStreamIndexFromKey(key: string): number | null {
  if (!key.startsWith("emb-")) return null;
  const n = Number(key.slice(4));
  return Number.isFinite(n) ? n : null;
}

function isAssFormat(format: string): boolean {
  const f = format.trim().toLowerCase();
  return f === "ass" || f === "ssa";
}

export function buildSubtitleTrackRequests(
  source: PlaybackTrackSource | null,
): SubtitleTrackOption[] {
  if (source == null) return [];
  const external =
    source.subtitles?.map((subtitle, index) => {
      const assEligible = isAssFormat(subtitle.format ?? "");
      return {
        key: `ext-${subtitle.id}`,
        label: subtitle.title || subtitle.language || `Subtitle ${index + 1}`,
        src: externalSubtitleUrl(BASE_URL, subtitle.id),
        srcLang: subtitle.language || "und",
        supported: true,
        assEligible,
        assSrc: assEligible ? externalSubtitleAssUrl(BASE_URL, subtitle.id) : undefined,
      };
    }) ?? [];
  const embedded =
    source.embeddedSubtitles?.map((subtitle, index) => {
      const catalogOk = subtitle.supported !== false;
      const requiresBurn = catalogOk && embeddedSubtitleNeedsWebBurnIn(subtitle);
      const assEligible = catalogOk && !requiresBurn && subtitle.assEligible === true;
      const labelBase =
        subtitle.title || subtitle.language || `Embedded subtitle ${index + 1}`;
      const label = !catalogOk
        ? `${labelBase} (Unavailable)`
        : requiresBurn
          ? `${labelBase} (burn-in)`
          : labelBase;
      return {
        key: `emb-${subtitle.streamIndex}`,
        label,
        src: embeddedSubtitleUrl(BASE_URL, source.mediaId, subtitle.streamIndex),
        srcLang: subtitle.language || "und",
        supported: catalogOk,
        disabled: !catalogOk,
        requiresBurn,
        assEligible,
        assSrc: assEligible
          ? embeddedSubtitleAssUrl(BASE_URL, source.mediaId, subtitle.streamIndex)
          : undefined,
      };
    }) ?? [];
  return [...external, ...embedded];
}

export function clonePlaybackTrackMetadata(
  metadata: PlaybackTrackMetadataInput,
): PlaybackTrackMetadata {
  return {
    subtitles: metadata.subtitles?.map((subtitle) => ({ ...subtitle })),
    embeddedSubtitles: metadata.embeddedSubtitles?.map((subtitle) => ({ ...subtitle })),
    embeddedAudioTracks: metadata.embeddedAudioTracks?.map((track) => ({ ...track })),
  };
}
