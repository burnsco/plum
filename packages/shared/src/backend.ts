export function ensureBaseUrl(raw: string | undefined | null): string {
  if (!raw) return "";
  return raw.endsWith("/") ? raw.slice(0, -1) : raw;
}

export function buildBackendUrl(base: string, path: string): string {
  const normalizedBase = ensureBaseUrl(base);
  if (!normalizedBase) return path;
  if (!path.startsWith("/")) return `${normalizedBase}/${path}`;
  return `${normalizedBase}${path}`;
}

export function mediaStreamUrl(base: string, mediaId: number): string {
  return buildBackendUrl(base, `/api/stream/${mediaId}`);
}

export function playbackSessionPlaylistUrl(
  base: string,
  sessionId: string,
  revision: number,
): string {
  return buildBackendUrl(
    base,
    `/api/playback/sessions/${sessionId}/revisions/${revision}/index.m3u8`,
  );
}

export function externalSubtitleUrl(base: string, subtitleId: number): string {
  return buildBackendUrl(base, `/api/subtitles/${subtitleId}`);
}

export function embeddedSubtitleUrl(base: string, mediaId: number, streamIndex: number): string {
  return buildBackendUrl(base, `/api/media/${mediaId}/subtitles/embedded/${streamIndex}`);
}

/**
 * Bitmap / image-based embedded subtitle codecs that Plum does not serve as WebVTT
 * (mirrors `EmbeddedSubtitleCodecLikelyBitmap` in the Go server).
 */
const EMBEDDED_SUBTITLE_CODECS_BLOCKING_WEBVTT = new Set([
  "hdmv_pgs_subtitle",
  "pgssub",
  "pgs",
  "dvd_subtitle",
  "dvdsub",
  "dvb_subtitle",
  "xsub",
  "dvb_teletext",
]);

/**
 * Web players cannot decode PGS; they must use a transcode with burn-in.
 * Browse payloads omit `vttEligible`, so we also infer from `codec` when the flag is absent.
 */
export function embeddedSubtitleNeedsWebBurnIn(subtitle: {
  supported?: boolean;
  vttEligible?: boolean;
  codec?: string;
}): boolean {
  if (subtitle.supported === false) return false;
  if (subtitle.vttEligible === true) return false;
  if (subtitle.vttEligible === false) return true;
  const c = subtitle.codec?.trim().toLowerCase() ?? "";
  if (c === "") return false;
  return EMBEDDED_SUBTITLE_CODECS_BLOCKING_WEBVTT.has(c);
}
