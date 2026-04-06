import Hls from "hls.js";
import type { ClientPlaybackCapabilities, MediaItem } from "../../api";
import { ignorePromise } from "../ignorePromise";
import { languageMatchesPreference, type SubtitleAppearance } from "../playbackPreferences";

export type BrowserAudioTrack = {
  enabled: boolean;
};

export type BrowserAudioTrackList = {
  length: number;
  [index: number]: BrowserAudioTrack | undefined;
};

export type HlsErrorData = {
  fatal: boolean;
  type?: string;
  details?: string;
  error?: Error;
};

type ParsedVttCueBlock = {
  startTime: number;
  endTime: number;
  text: string;
  settings: string[];
};

export type TrackMenuOption = {
  key: string;
  label: string;
  disabled?: boolean;
};

export type SubtitleTrackOption = TrackMenuOption & {
  src: string;
  srcLang: string;
  supported?: boolean;
  /** PGS-style track: selecting it restarts playback with server burn-in transcode. */
  requiresBurn?: boolean;
  /** ASS/SSA track: use JASSUB renderer instead of the HTML5 TextTrack API. */
  assEligible?: boolean;
  /** Raw ASS/SSA URL served by the /ass endpoint; only set when assEligible is true. */
  assSrc?: string;
};

export type AudioTrackOption = TrackMenuOption & {
  streamIndex: number;
  language: string;
};

function canPlay(element: HTMLVideoElement, value: string): boolean {
  const result = element.canPlayType(value);
  return result === "probably" || result === "maybe";
}

export function detectClientPlaybackCapabilities(): ClientPlaybackCapabilities {
  if (typeof document === "undefined") {
    return {
      supportsNativeHls: false,
      supportsMseHls: false,
      videoCodecs: [],
      audioCodecs: [],
      containers: [],
    };
  }

  const video = document.createElement("video");
  const containers = [
    canPlay(video, "video/mp4") ? "mp4" : null,
    canPlay(video, "video/webm") ? "webm" : null,
    canPlay(video, "video/ogg") ? "ogg" : null,
  ].filter((value): value is string => value != null);

  const videoCodecs = [
    canPlay(video, 'video/mp4; codecs="avc1.42E01E"') ? "h264" : null,
    canPlay(video, 'video/mp4; codecs="hvc1.1.6.L93.B0"') ? "hevc" : null,
    canPlay(video, 'video/mp4; codecs="av01.0.05M.08"') ? "av1" : null,
    canPlay(video, 'video/webm; codecs="vp9"') ? "vp9" : null,
    canPlay(video, 'video/webm; codecs="vp8"') ? "vp8" : null,
  ].filter((value): value is string => value != null);

  const audioCodecs = [
    canPlay(video, 'audio/mp4; codecs="mp4a.40.2"') ? "aac" : null,
    canPlay(video, 'audio/mpeg; codecs="mp3"') ? "mp3" : null,
    canPlay(video, 'audio/webm; codecs="opus"') ? "opus" : null,
    canPlay(video, 'audio/webm; codecs="vorbis"') ? "vorbis" : null,
    canPlay(video, 'audio/mp4; codecs="ac-3"') ? "ac3" : null,
    canPlay(video, 'audio/mp4; codecs="ec-3"') ? "eac3" : null,
    canPlay(video, 'audio/ogg; codecs="flac"') ? "flac" : null,
  ].filter((value): value is string => value != null);

  return {
    supportsNativeHls: canPlay(video, "application/vnd.apple.mpegurl"),
    supportsMseHls: Hls.isSupported(),
    videoCodecs,
    audioCodecs,
    containers,
  };
}

export function getBrowserAudioTracks(
  element: HTMLVideoElement | null,
): BrowserAudioTrackList | null {
  if (!element) return null;
  const audioTracks = (element as HTMLVideoElement & { audioTracks?: BrowserAudioTrackList })
    .audioTracks;
  return audioTracks && typeof audioTracks.length === "number" ? audioTracks : null;
}

export function formatTrackLabel(
  title: string | undefined,
  language: string | undefined,
  fallback: string,
): string {
  const normalizedTitle = title?.trim();
  const normalizedLanguage = language?.trim();
  if (
    normalizedTitle &&
    normalizedLanguage &&
    normalizedTitle.localeCompare(normalizedLanguage, undefined, { sensitivity: "accent" }) !== 0
  ) {
    return `${normalizedTitle} • ${normalizedLanguage}`;
  }
  return normalizedTitle || normalizedLanguage || fallback;
}

export function formatClock(totalSeconds: number): string {
  if (!Number.isFinite(totalSeconds) || totalSeconds <= 0) return "0:00";
  const wholeSeconds = Math.floor(totalSeconds);
  const hours = Math.floor(wholeSeconds / 3600);
  const minutes = Math.floor((wholeSeconds % 3600) / 60);
  const seconds = wholeSeconds % 60;
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
  }
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}

export function formatHlsErrorMessage(data: HlsErrorData): string {
  return data.details || data.type || data.error?.message || "Playback stream failed";
}

export function resolvedVideoDuration(
  authoritativeDuration: number,
  itemDuration: number,
  elementDuration: number,
): number {
  if (Number.isFinite(authoritativeDuration) && authoritativeDuration > 0) {
    return authoritativeDuration;
  }
  if (Number.isFinite(itemDuration) && itemDuration > 0) {
    return itemDuration;
  }
  return Number.isFinite(elementDuration) && elementDuration > 0 ? elementDuration : 0;
}

/** Plum transcode/remux HLS uses growing `EVENT` playlists; direct file streams are full seekable range. */
export type PlumVideoDelivery = "direct" | "remux" | "transcode";

/** Avoid seeking exactly to the reported end; some pipelines stall instead of ending cleanly. */
const SEEK_END_EPSILON_SEC = 0.05;

function mediaSeekableEndSeconds(video: HTMLMediaElement): number {
  try {
    if (video.seekable.length <= 0) return 0;
    const end = video.seekable.end(video.seekable.length - 1);
    return Number.isFinite(end) && end > 0 ? end : 0;
  } catch {
    return 0;
  }
}

/**
 * Latest time (seconds) it is safe to seek to. For remux/transcode HLS, uses the minimum of catalog
 * duration, `video.duration`, and the seekable range end so we never jump past encoded segments.
 * For direct play, uses the longer of catalog vs media so short browser metadata does not shrink range.
 */
export function seekUpperBoundSeconds(
  video: HTMLMediaElement,
  authoritativeSeconds: number,
  itemDurationSeconds: number,
  delivery: PlumVideoDelivery,
): number {
  const catalog = resolvedVideoDuration(authoritativeSeconds, itemDurationSeconds, 0);
  const elDur =
    Number.isFinite(video.duration) && video.duration > 0 ? video.duration : 0;
  const seekableEnd = mediaSeekableEndSeconds(video);

  if (delivery === "remux" || delivery === "transcode") {
    const parts: number[] = [];
    if (catalog > 0) parts.push(catalog);
    if (elDur > 0) parts.push(elDur);
    if (seekableEnd > 0) parts.push(seekableEnd);
    if (parts.length === 0) return 0;
    return Math.min(...parts);
  }

  const fromMedia = Math.max(elDur, seekableEnd);
  if (catalog > 0 && fromMedia > 0) {
    return Math.max(catalog, fromMedia);
  }
  if (catalog > 0) return catalog;
  if (fromMedia > 0) return fromMedia;
  return 0;
}

export function clampVideoSeekSeconds(
  video: HTMLMediaElement,
  targetSeconds: number,
  authoritativeSeconds: number,
  itemDurationSeconds: number,
  delivery: PlumVideoDelivery,
): number {
  const upper = seekUpperBoundSeconds(
    video,
    authoritativeSeconds,
    itemDurationSeconds,
    delivery,
  );
  const t = Math.max(0, targetSeconds);
  if (upper <= 0 || !Number.isFinite(upper)) return t;
  return Math.min(t, Math.max(0, upper - SEEK_END_EPSILON_SEC));
}

export function nudgeVideoIntoBufferedRange(video: HTMLVideoElement | null): boolean {
  if (!video || video.buffered.length === 0) {
    return false;
  }

  const currentTime = Number.isFinite(video.currentTime) ? video.currentTime : 0;
  for (let index = 0; index < video.buffered.length; index += 1) {
    const start = video.buffered.start(index);
    const end = video.buffered.end(index);
    if (currentTime + 0.05 < start) {
      video.currentTime = start + 0.01;
      return true;
    }
    if (currentTime >= start && currentTime <= end) {
      return false;
    }
  }

  const lastIndex = video.buffered.length - 1;
  if (lastIndex >= 0) {
    const start = video.buffered.start(lastIndex);
    if (currentTime < start) {
      video.currentTime = start + 0.01;
      return true;
    }
  }

  return false;
}

export function bufferedRangeStartsNearZero(video: HTMLVideoElement | null): boolean {
  if (!video || video.buffered.length === 0) {
    return false;
  }
  return video.buffered.start(0) <= 0.05;
}

function parseVttTimestamp(value: string): number | null {
  const match = value.trim().match(/^(?:(\d+):)?(\d{2}):(\d{2})\.(\d{3})$/);
  if (!match) {
    return null;
  }

  const hours = Number(match[1] ?? 0);
  const minutes = Number(match[2]);
  const seconds = Number(match[3]);
  const milliseconds = Number(match[4]);
  return hours * 3600 + minutes * 60 + seconds + milliseconds / 1000;
}

/**
 * Normalizes bytes received so far for {@link buildSubtitleCues} / {@link parseVttCueBlocks}.
 *
 * While streaming we must **not** truncate to the last `\n\n`: ffmpeg often emits the first cue
 * as `WEBVTT`, blank line, timing line, then cue text **without** a trailing blank line for a long
 * time. Truncating to the header-only prefix yields zero cues and leaves the UI stuck on “Loading”.
 * {@link parseVttCueBlocks} already treats an in-progress last cue (no closing blank line) as a
 * valid block once the timing line is complete.
 */
export function streamingVttPrefixForParse(accum: string, streamDone: boolean): string {
  const n = accum.replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n");
  if (streamDone) {
    return n;
  }
  return n;
}

function normalizeVttInput(raw: string): string {
  return raw.replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n");
}

const defaultSubtitleStreamFlushMs = 280;

/**
 * Reads a fetch Response body incrementally and invokes onPartial with a VTT prefix safe for
 * {@link buildSubtitleCues} while bytes are still arriving. Lets subtitles appear before the
 * server finishes the full embedded extract.
 */
export async function consumeSubtitleResponseWithPartialUpdates(
  response: Response,
  signal: AbortSignal,
  onPartial: (bodyForState: string, streamDone: boolean) => void,
  flushMs: number = defaultSubtitleStreamFlushMs,
): Promise<string> {
  const reader = response.body?.getReader();
  if (!reader) {
    const text = await response.text();
    const n = normalizeVttInput(text);
    onPartial(n, true);
    return n;
  }

  const decoder = new TextDecoder();
  let accum = "";
  let lastFlushAt = 0;

  const flush = (streamDone: boolean) => {
    const bodyForState = streamingVttPrefixForParse(accum, streamDone);
    if (streamDone) {
      const full = normalizeVttInput(accum);
      onPartial(full, true);
    } else {
      onPartial(bodyForState, false);
    }
  };

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (signal.aborted) {
        ignorePromise(reader.cancel(), "playerMedia:readerCancelAborted");
        throw new DOMException("Aborted", "AbortError");
      }
      if (value) {
        accum += decoder.decode(value, { stream: true });
      }
      const now = Date.now();
      if (done) {
        accum += decoder.decode();
        flush(true);
        return normalizeVttInput(accum);
      }
      if (now - lastFlushAt >= flushMs) {
        lastFlushAt = now;
        flush(false);
      }
    }
  } catch (error) {
    if (signal.aborted || (error instanceof DOMException && error.name === "AbortError")) {
      ignorePromise(reader.cancel(), "playerMedia:readerCancelCatch");
      throw new DOMException("Aborted", "AbortError");
    }
    throw error;
  }
}

export function parseVttCueBlocks(body: string): ParsedVttCueBlock[] {
  const normalized = body.replace(/^\uFEFF/, "").replace(/\r\n?/g, "\n");
  const lines = normalized.split("\n");
  const cues: ParsedVttCueBlock[] = [];

  let index = 0;
  if (lines[0]?.startsWith("WEBVTT")) {
    index += 1;
    while (index < lines.length && lines[index]?.trim() !== "") {
      index += 1;
    }
  }

  while (index < lines.length) {
    while (index < lines.length && lines[index]?.trim() === "") {
      index += 1;
    }
    if (index >= lines.length) {
      break;
    }

    const blockStart = lines[index]?.trim() ?? "";
    if (
      blockStart.startsWith("NOTE") ||
      blockStart.startsWith("STYLE") ||
      blockStart.startsWith("REGION")
    ) {
      while (index < lines.length && lines[index]?.trim() !== "") {
        index += 1;
      }
      continue;
    }

    let timingLine = blockStart;
    if (!timingLine.includes("-->")) {
      index += 1;
      timingLine = lines[index]?.trim() ?? "";
    }
    if (!timingLine.includes("-->")) {
      while (index < lines.length && lines[index]?.trim() !== "") {
        index += 1;
      }
      continue;
    }

    const [startToken, endTokenWithSettings] = timingLine.split("-->");
    const timingParts = endTokenWithSettings.trim().split(/\s+/);
    const startTime = parseVttTimestamp(startToken);
    const endTime = parseVttTimestamp(timingParts[0] ?? "");
    if (startTime == null || endTime == null) {
      while (index < lines.length && lines[index]?.trim() !== "") {
        index += 1;
      }
      continue;
    }

    index += 1;
    const textLines: string[] = [];
    while (index < lines.length && lines[index]?.trim() !== "") {
      textLines.push(lines[index] ?? "");
      index += 1;
    }

    cues.push({
      startTime,
      endTime: Math.max(endTime, startTime + 0.001),
      text: textLines.join("\n"),
      settings: timingParts.slice(1),
    });
  }

  return cues;
}

function subtitleLabelMatchesHint(trackLabel: string, hint: string): boolean {
  const a = trackLabel.trim().toLowerCase();
  const b = hint.trim().toLowerCase();
  if (!b) return true;
  return a === b || a.includes(b) || b.includes(a);
}

export function getPreferredSubtitleKey(
  subtitleTracks: SubtitleTrackOption[],
  preferredLanguage: string,
  subtitlesEnabled: boolean,
  subtitleLabelHint?: string,
): string {
  if (!subtitlesEnabled || !preferredLanguage) return "off";
  const hint = subtitleLabelHint?.trim() ?? "";
  const langMatch = (track: SubtitleTrackOption) =>
    track.supported !== false &&
    (languageMatchesPreference(track.srcLang, preferredLanguage) ||
      languageMatchesPreference(track.label, preferredLanguage));
  const noBurn = (track: SubtitleTrackOption) => !track.requiresBurn;

  if (hint) {
    const hintMatches = subtitleTracks.filter(
      (track) => langMatch(track) && subtitleLabelMatchesHint(track.label, hint),
    );
    const bestHint = hintMatches.find(noBurn) ?? hintMatches[0];
    if (bestHint) return bestHint.key;
  }

  const allLangMatches = subtitleTracks.filter(langMatch);
  const best = allLangMatches.find(noBurn) ?? allLangMatches[0];
  return best?.key ?? "off";
}

export function getPreferredAudioKey(
  audioTracks: AudioTrackOption[],
  preferredLanguage: string,
): string {
  if (!preferredLanguage) return "";
  return (
    audioTracks.find(
      (track) =>
        languageMatchesPreference(track.language, preferredLanguage) ||
        languageMatchesPreference(track.label, preferredLanguage),
    )?.key ?? ""
  );
}

export function applyCueLineSetting(cue: TextTrackCue, position: SubtitleAppearance["position"]) {
  const cueWithLine = cue as TextTrackCue & { line?: number | string };
  if (!("line" in cueWithLine)) return;
  cueWithLine.line = position === "top" ? 8 : "auto";
}

export function applyVttCueSettings(cue: TextTrackCue, settings: string[]) {
  const vttCue = cue as TextTrackCue & {
    align?: "start" | "center" | "end" | "left" | "right";
    line?: number | string;
    position?: number | string;
    size?: number;
    vertical?: string;
  };

  for (const setting of settings) {
    const [key, value] = setting.split(":", 2);
    if (!key || !value) continue;
    switch (key) {
      case "align":
        if (["start", "center", "end", "left", "right"].includes(value)) {
          vttCue.align = value as "start" | "center" | "end" | "left" | "right";
        }
        break;
      case "line":
        vttCue.line = value === "auto" ? "auto" : Number(value.replace("%", ""));
        break;
      case "position":
        vttCue.position = Number(value.replace("%", ""));
        break;
      case "size":
        vttCue.size = Number(value.replace("%", ""));
        break;
      case "vertical":
        vttCue.vertical = value;
        break;
    }
  }
}

export function clearTextTrackCues(track: TextTrack | null) {
  if (!track) return;
  // track.cues is null when mode === "disabled"; switch to "hidden" so we can access the list.
  const wasDisabled = track.mode === "disabled";
  if (wasDisabled) track.mode = "hidden";
  const cues = track.cues;
  if (!cues) {
    if (wasDisabled) track.mode = "disabled";
    return;
  }
  while (cues.length > 0) {
    const cue = cues[0];
    if (!cue) break;
    track.removeCue(cue);
  }
  if (wasDisabled) track.mode = "disabled";
}

export function buildSubtitleCues(body: string): TextTrackCue[] {
  const CueConstructor =
    typeof window !== "undefined" ? (window.VTTCue ?? window.TextTrackCue) : undefined;
  if (!CueConstructor) {
    return [];
  }

  return parseVttCueBlocks(body)
    .map((cueBlock) => {
      const cue = new CueConstructor(
        cueBlock.startTime,
        cueBlock.endTime,
        cueBlock.text,
      ) as TextTrackCue;
      applyVttCueSettings(cue, cueBlock.settings);
      return cue;
    })
    .filter(Boolean);
}

export function hasTextTrack(video: HTMLVideoElement, track: TextTrack | null): boolean {
  if (!track) {
    return false;
  }
  for (let index = 0; index < video.textTracks.length; index += 1) {
    if (video.textTracks[index] === track) {
      return true;
    }
  }
  return false;
}

/** Virtual media playlist filename served under the playback revision (matches server `plum_subs_*` names). */
export function plumHlsSubtitlePlaylistFileForTrackKey(trackKey: string): string | null {
  if (trackKey === "off") return null;
  const match = /^(emb|ext)-(\d+)$/.exec(trackKey);
  if (!match) return null;
  return `plum_subs_${match[1]}_${match[2]}.m3u8`;
}

/** Resolves hls.js subtitle track index for a Plum menu key when the master lists our virtual playlists. */
export function findHlsSubtitleTrackIndexForPlumKey(hls: Hls, trackKey: string): number {
  const wantFile = plumHlsSubtitlePlaylistFileForTrackKey(trackKey);
  if (!wantFile) return -1;
  const tracks = hls.subtitleTracks ?? [];
  for (let i = 0; i < tracks.length; i++) {
    const url = tracks[i]?.url ?? "";
    if (url.includes(wantFile)) return i;
  }
  return -1;
}

export function getSeasonEpisodeLabel(item: MediaItem): string | null {
  const season = item.season ?? 0;
  const episode = item.episode ?? 0;
  if (season <= 0 && episode <= 0) return null;
  return `S${String(season).padStart(2, "0")}E${String(episode).padStart(2, "0")}`;
}

export function getVideoMetadata(item: MediaItem): string {
  const bits = [item.type === "movie" ? "Movie" : item.type === "anime" ? "Anime" : "TV"];
  const seasonEpisode = getSeasonEpisodeLabel(item);
  const releaseYear =
    item.release_date?.split("-")[0] ||
    (item.type === "movie" ? item.title.match(/\((\d{4})\)$/)?.[1] : undefined);

  if (seasonEpisode) bits.push(seasonEpisode);
  if (releaseYear) bits.push(releaseYear);
  if (item.duration > 0) bits.push(formatClock(item.duration));
  return bits.join(" • ");
}

export function getMusicMetadata(item: MediaItem, queueIndex: number, queueSize: number): string {
  const bits = [item.artist || "Unknown Artist"];
  if (item.album) bits.push(item.album);
  if (queueSize > 0) bits.push(`${queueIndex + 1}/${queueSize}`);
  return bits.join(" • ");
}
