import {
  buildBackendUrl,
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
  type EmbeddedSubtitleDeliveryMode,
  type EmbeddedSubtitleDeliveryOption,
  type MediaAttachment,
  type PlaybackTrackMetadata,
  type Subtitle,
} from "../../api";
import {
  embeddedStreamIndexFromLogicalId,
  formatSubtitleTrackLabel,
  sortSubtitleTrackOptions,
  type SubtitleTrackOption,
} from "./playerMedia";

export type PlaybackTrackMetadataInput = {
  subtitles?: readonly Subtitle[];
  embeddedSubtitles?: readonly EmbeddedSubtitle[];
  embeddedAudioTracks?: readonly EmbeddedAudioTrack[];
  mediaAttachments?: readonly MediaAttachment[];
};

export type PlaybackTrackSource = PlaybackTrackMetadataInput & {
  mediaId: number;
};

/** Avoid unbounded growth of failed subtitle keys over a long session (oldest entries drop first). */
export const MAX_BLOCKED_SUBTITLE_RETRY_KEYS = 64;

const ASS_FONT_MIME_TYPES = new Set([
  "application/vnd.ms-opentype",
  "application/x-truetype-font",
  "font/otf",
  "font/ttf",
  "font/woff",
  "font/woff2",
]);

const ASS_FONT_EXTENSIONS = new Set([".otf", ".ttf", ".woff", ".woff2"]);

export function rememberBlockedSubtitleKey(set: Set<string>, key: string) {
  set.add(key);
  while (set.size > MAX_BLOCKED_SUBTITLE_RETRY_KEYS) {
    const oldest = set.keys().next().value;
    if (oldest === undefined) break;
    set.delete(oldest);
  }
}

export function embeddedStreamIndexFromKey(key: string): number | null {
  return embeddedStreamIndexFromLogicalId(key);
}

function isAssFormat(format: string): boolean {
  const f = format.trim().toLowerCase();
  return f === "ass" || f === "ssa";
}

function mediaAttachmentExtension(fileName: string | undefined): string {
  const clean = (fileName ?? "").split(/[?#]/, 1)[0] ?? "";
  const dot = clean.lastIndexOf(".");
  if (dot < 0) return "";
  return clean.slice(dot).toLowerCase();
}

function isAssFontAttachment(attachment: MediaAttachment): boolean {
  const mime = attachment.mimeType?.trim().toLowerCase() ?? "";
  if (mime !== "") return ASS_FONT_MIME_TYPES.has(mime);
  return ASS_FONT_EXTENSIONS.has(mediaAttachmentExtension(attachment.fileName));
}

export function buildAssFontUrls(
  baseUrl: string,
  attachments: readonly MediaAttachment[] | undefined,
): string[] {
  return (
    attachments
      ?.filter(isAssFontAttachment)
      .map((attachment) => buildBackendUrl(baseUrl, attachment.deliveryUrl)) ?? []
  );
}

function inferredEmbeddedDeliveryModes(
  subtitle: EmbeddedSubtitle,
  requiresBurn: boolean,
  assEligible: boolean,
): EmbeddedSubtitleDeliveryOption[] | undefined {
  if (subtitle.deliveryModes?.length) {
    return subtitle.deliveryModes.map((mode) => ({ ...mode }));
  }
  if (subtitle.supported === false) {
    return undefined;
  }
  if (requiresBurn) {
    return [{ mode: "burn_in", requiresReload: true }];
  }
  const modes: EmbeddedSubtitleDeliveryOption[] = [{ mode: "direct_vtt", requiresReload: false }];
  if (assEligible) {
    modes.push({ mode: "ass", requiresReload: false });
  }
  return modes;
}

function inferredPreferredWebDeliveryMode(
  subtitle: EmbeddedSubtitle,
  requiresBurn: boolean,
  assEligible: boolean,
): EmbeddedSubtitleDeliveryMode | undefined {
  if (subtitle.preferredWebDeliveryMode != null) {
    return subtitle.preferredWebDeliveryMode;
  }
  if (subtitle.supported === false) {
    return undefined;
  }
  if (requiresBurn) {
    return "burn_in";
  }
  if (assEligible) {
    return "ass";
  }
  return "direct_vtt";
}

export function buildSubtitleTrackRequests(
  source: PlaybackTrackSource | null,
): SubtitleTrackOption[] {
  if (source == null) return [];
  const external =
    source.subtitles?.map((subtitle, index) => {
      const assEligible = isAssFormat(subtitle.format ?? "");
      const deliveryModes: ReadonlyArray<EmbeddedSubtitleDeliveryOption> = assEligible
        ? [
            { mode: "direct_vtt", requiresReload: false },
            { mode: "ass", requiresReload: false },
          ]
        : [{ mode: "direct_vtt", requiresReload: false }];
      const preferredWebDeliveryMode: EmbeddedSubtitleDeliveryMode = assEligible
        ? "ass"
        : "direct_vtt";
      const logicalId = subtitle.logicalId || `ext:${subtitle.id}`;
      return {
        key: logicalId,
        logicalId,
        origin: "external" as const,
        label: formatSubtitleTrackLabel(subtitle.title, subtitle.language, `Subtitle ${index + 1}`),
        src: externalSubtitleUrl(BASE_URL, subtitle.id),
        srcLang: subtitle.language || "und",
        supported: true,
        forced: subtitle.forced === true,
        default: subtitle.default === true,
        hearingImpaired: subtitle.hearingImpaired === true,
        deliveryModes,
        preferredWebDeliveryMode,
        assEligible,
        assSrc: assEligible ? externalSubtitleAssUrl(BASE_URL, subtitle.id) : undefined,
      };
    }) ?? [];
  const embedded =
    source.embeddedSubtitles?.map((subtitle, index) => {
      const catalogOk = subtitle.supported !== false;
      const requiresBurn = catalogOk && embeddedSubtitleNeedsWebBurnIn(subtitle);
      const assEligible = catalogOk && !requiresBurn && subtitle.assEligible === true;
      const deliveryModes = inferredEmbeddedDeliveryModes(subtitle, requiresBurn, assEligible);
      const preferredWebDeliveryMode = inferredPreferredWebDeliveryMode(
        subtitle,
        requiresBurn,
        assEligible,
      );
      const labelBase = formatSubtitleTrackLabel(
        subtitle.title,
        subtitle.language,
        `Embedded subtitle ${index + 1}`,
      );
      const label = !catalogOk
        ? `${labelBase} (Unavailable)`
        : requiresBurn
          ? `${labelBase} (burn-in)`
          : labelBase;
      const logicalId = subtitle.logicalId || `emb:${subtitle.streamIndex}`;
      return {
        key: logicalId,
        logicalId,
        origin: "embedded" as const,
        label,
        src: embeddedSubtitleUrl(BASE_URL, source.mediaId, subtitle.streamIndex),
        srcLang: subtitle.language || "und",
        supported: catalogOk,
        forced: subtitle.forced === true,
        default: subtitle.default === true,
        hearingImpaired: subtitle.hearingImpaired === true,
        disabled: !catalogOk,
        deliveryModes,
        preferredWebDeliveryMode,
        requiresBurn,
        assEligible,
        assSrc: assEligible
          ? embeddedSubtitleAssUrl(BASE_URL, source.mediaId, subtitle.streamIndex)
          : undefined,
      };
    }) ?? [];
  return sortSubtitleTrackOptions([...external, ...embedded]);
}

export function clonePlaybackTrackMetadata(
  metadata: PlaybackTrackMetadataInput,
): PlaybackTrackMetadata {
  return {
    subtitles: metadata.subtitles?.map((subtitle) => ({ ...subtitle })),
    embeddedSubtitles: metadata.embeddedSubtitles?.map((subtitle) => ({
      ...subtitle,
    })),
    embeddedAudioTracks: metadata.embeddedAudioTracks?.map((track) => ({
      ...track,
    })),
    mediaAttachments: metadata.mediaAttachments?.map((attachment) => ({
      ...attachment,
    })),
  };
}
