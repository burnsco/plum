import type { IntroSkipMode, Library, MediaItem } from "../api";
import { getShowKey } from "./showGrouping";

export type SubtitleSize = "small" | "medium" | "large";
export type SubtitlePosition = "top" | "bottom";

export type SubtitleAppearance = {
  size: SubtitleSize;
  position: SubtitlePosition;
  color: string;
};

export type ResolvedLibraryPlaybackPreferences = {
  preferredAudioLanguage: string;
  preferredSubtitleLanguage: string;
  subtitlesEnabledByDefault: boolean;
  introSkipMode: IntroSkipMode;
};

export const subtitleAppearanceStorageKey = "plum:subtitle-appearance";
export const videoAutoplayStorageKey = "plum:video-autoplay-next";
export const videoAspectModeStorageKey = "plum:video-aspect-mode";
export const playerWebDefaultsStorageKey = "plum:player-web-defaults";
/** Per-series (library id + show key) audio/subtitle picks for TV/anime episodes. */
export const showTrackDefaultsStorageKey = "plum:player-show-track-defaults";

/** Must match Android [plum.tv.core.data.VideoAspectRatioMode.storageValue]. */
export type VideoAspectMode =
  | "auto"
  | "zoom"
  | "stretch"
  | "ratio-16-9"
  | "ratio-4-3"
  | "ratio-21-9";

export const defaultVideoAspectMode: VideoAspectMode = "auto";

export const videoAspectModeOptions: Array<{ value: VideoAspectMode; label: string }> = [
  { value: "auto", label: "Auto (stream)" },
  { value: "zoom", label: "Zoom (crop to fill)" },
  { value: "stretch", label: "Stretch to screen" },
  { value: "ratio-16-9", label: "16:9 frame" },
  { value: "ratio-4-3", label: "4:3 frame" },
  { value: "ratio-21-9", label: "21:9 frame" },
];

export function formatDetectedVideoAspectLabel(width: number, height: number): string | null {
  if (width <= 0 || height <= 0) return null;
  const dar = width / height;
  if (!Number.isFinite(dar) || dar <= 0) return null;
  const rounded = Math.round(dar * 100) / 100;
  const text = Number.isInteger(rounded) ? String(rounded) : rounded.toFixed(2).replace(/\.?0+$/, "");
  return `${text}:1`;
}

function normalizeVideoAspectMode(raw: string | undefined | null): VideoAspectMode {
  if (!raw || typeof raw !== "string") return defaultVideoAspectMode;
  const found = videoAspectModeOptions.find((o) => o.value === raw.trim());
  return found ? found.value : defaultVideoAspectMode;
}

const PLAYER_LOCAL_SETTINGS_EVENT = "plum-player-local-settings-changed";

/**
 * Stored in `defaultSubtitleLanguage` / `defaultAudioLanguage` when the user disables automatic
 * language-based track selection. See `PlayerWebDefaults` for all modes.
 */
export const PLAYER_WEB_TRACK_LANGUAGE_NONE = "__none__";

export type PlayerWebDefaults = {
  /** Overrides library preferred audio for automatic track selection unless {@link PLAYER_WEB_TRACK_LANGUAGE_NONE}. */
  defaultAudioLanguage: string;
  /** Overrides library preferred subtitle language for automatic track selection unless {@link PLAYER_WEB_TRACK_LANGUAGE_NONE}. */
  defaultSubtitleLanguage: string;
  /**
   * When non-empty, disambiguates multiple subtitle tracks with the same language (saved from the last
   * manual in-player subtitle choice). Cleared when changing subtitle language from Settings.
   */
  defaultSubtitleLabelHint: string;
  /** Limit the in-player subtitle menu to tracks that look English (eng, SDH, etc.). */
  subtitleMenuEnglishOnly: boolean;
};

const defaultPlayerWebDefaults: PlayerWebDefaults = {
  defaultAudioLanguage: "",
  defaultSubtitleLanguage: "",
  defaultSubtitleLabelHint: "",
  subtitleMenuEnglishOnly: false,
};

export type PlayerLocalSettingsSnapshot = {
  subtitleAppearance: SubtitleAppearance;
  webDefaults: PlayerWebDefaults;
  videoAutoplayEnabled: boolean;
  videoAspectMode: VideoAspectMode;
  /** Bumps when {@link showTrackDefaultsStorageKey} changes so players re-resolve per-show track prefs. */
  showTrackDefaultsRevision: number;
};

let cachedPlayerLocalSettingsSnapshot: PlayerLocalSettingsSnapshot | null = null;

let showTrackDefaultsRevision = 0;

export function getShowTrackDefaultsRevision(): number {
  return showTrackDefaultsRevision;
}

/** Optional fields: omitted keys inherit from global {@link PlayerWebDefaults} when resolving. */
export type ShowTrackDefaultsRecord = {
  defaultAudioLanguage?: string;
  defaultSubtitleLanguage?: string;
  defaultSubtitleLabelHint?: string;
};

type ShowTrackDefaultsMap = Record<string, ShowTrackDefaultsRecord>;

function readShowTrackDefaultsMap(): ShowTrackDefaultsMap {
  if (typeof window === "undefined") return {};
  try {
    const raw = window.localStorage.getItem(showTrackDefaultsStorageKey);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as unknown;
    if (parsed == null || typeof parsed !== "object" || Array.isArray(parsed)) return {};
    return parsed as ShowTrackDefaultsMap;
  } catch {
    return {};
  }
}

function writeShowTrackDefaultsMap(map: ShowTrackDefaultsMap) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(showTrackDefaultsStorageKey, JSON.stringify(map));
  showTrackDefaultsRevision += 1;
  bumpPlayerLocalSettings();
}

/**
 * Merges track-default fields for a TV/anime series (all seasons/episodes share one entry per
 * library + {@link getShowKey}).
 */
export function mergeShowTrackDefaultsForEpisode(
  item: MediaItem | null | undefined,
  patch: ShowTrackDefaultsRecord,
): void {
  if (!item || (item.type !== "tv" && item.type !== "anime")) return;
  const lib = item.library_id;
  if (lib == null || lib <= 0) return;
  const showKey = getShowKey(item);
  const composite = `${lib}:${showKey}`;
  const map = readShowTrackDefaultsMap();
  const prev = map[composite] ?? {};
  map[composite] = { ...prev, ...patch };
  writeShowTrackDefaultsMap(map);
}

export type WebTrackDefaultsSlice = Pick<
  PlayerWebDefaults,
  "defaultAudioLanguage" | "defaultSubtitleLanguage" | "defaultSubtitleLabelHint"
>;

/** Merges global web defaults with per-show overrides for the active episode, if any. */
export function resolveEffectiveWebTrackDefaults(
  item: MediaItem | null | undefined,
  globalWeb: PlayerWebDefaults,
): WebTrackDefaultsSlice {
  const base: WebTrackDefaultsSlice = {
    defaultAudioLanguage: globalWeb.defaultAudioLanguage,
    defaultSubtitleLanguage: globalWeb.defaultSubtitleLanguage,
    defaultSubtitleLabelHint: globalWeb.defaultSubtitleLabelHint,
  };
  if (!item || (item.type !== "tv" && item.type !== "anime")) return base;
  const lib = item.library_id;
  if (lib == null || lib <= 0) return base;
  const rec = readShowTrackDefaultsMap()[`${lib}:${getShowKey(item)}`];
  if (!rec) return base;
  return {
    defaultAudioLanguage:
      rec.defaultAudioLanguage !== undefined ? rec.defaultAudioLanguage : base.defaultAudioLanguage,
    defaultSubtitleLanguage:
      rec.defaultSubtitleLanguage !== undefined ? rec.defaultSubtitleLanguage : base.defaultSubtitleLanguage,
    defaultSubtitleLabelHint:
      rec.defaultSubtitleLabelHint !== undefined ? rec.defaultSubtitleLabelHint : base.defaultSubtitleLabelHint,
  };
}

function playerLocalSettingsSnapshotsContentEqual(
  a: PlayerLocalSettingsSnapshot,
  b: PlayerLocalSettingsSnapshot,
): boolean {
  return (
    a.videoAutoplayEnabled === b.videoAutoplayEnabled &&
    a.videoAspectMode === b.videoAspectMode &&
    a.subtitleAppearance.size === b.subtitleAppearance.size &&
    a.subtitleAppearance.position === b.subtitleAppearance.position &&
    a.subtitleAppearance.color === b.subtitleAppearance.color &&
    a.webDefaults.defaultAudioLanguage === b.webDefaults.defaultAudioLanguage &&
    a.webDefaults.defaultSubtitleLanguage === b.webDefaults.defaultSubtitleLanguage &&
    a.webDefaults.defaultSubtitleLabelHint === b.webDefaults.defaultSubtitleLabelHint &&
    a.webDefaults.subtitleMenuEnglishOnly === b.webDefaults.subtitleMenuEnglishOnly &&
    a.showTrackDefaultsRevision === b.showTrackDefaultsRevision
  );
}

function bumpPlayerLocalSettings() {
  cachedPlayerLocalSettingsSnapshot = null;
  if (typeof window === "undefined") return;
  window.dispatchEvent(new Event(PLAYER_LOCAL_SETTINGS_EVENT));
}

export function subscribePlayerLocalSettings(onStoreChange: () => void): () => void {
  if (typeof window === "undefined") {
    return () => {};
  }
  const handler = () => onStoreChange();
  window.addEventListener(PLAYER_LOCAL_SETTINGS_EVENT, handler);
  const onStorage = (event: StorageEvent) => {
    if (event.key === showTrackDefaultsStorageKey) {
      showTrackDefaultsRevision += 1;
    }
    if (
      event.key == null ||
      event.key === subtitleAppearanceStorageKey ||
      event.key === videoAutoplayStorageKey ||
      event.key === videoAspectModeStorageKey ||
      event.key === playerWebDefaultsStorageKey ||
      event.key === showTrackDefaultsStorageKey
    ) {
      cachedPlayerLocalSettingsSnapshot = null;
      onStoreChange();
    }
  };
  window.addEventListener("storage", onStorage);
  return () => {
    window.removeEventListener(PLAYER_LOCAL_SETTINGS_EVENT, handler);
    window.removeEventListener("storage", onStorage);
  };
}

export function readStoredPlayerWebDefaults(): PlayerWebDefaults {
  if (typeof window === "undefined") {
    return { ...defaultPlayerWebDefaults };
  }
  try {
    const raw = window.localStorage.getItem(playerWebDefaultsStorageKey);
    if (!raw) return { ...defaultPlayerWebDefaults };
    const parsed = JSON.parse(raw) as Partial<PlayerWebDefaults>;
    return {
      defaultAudioLanguage:
        typeof parsed.defaultAudioLanguage === "string" ? parsed.defaultAudioLanguage : "",
      defaultSubtitleLanguage:
        typeof parsed.defaultSubtitleLanguage === "string" ? parsed.defaultSubtitleLanguage : "",
      defaultSubtitleLabelHint:
        typeof parsed.defaultSubtitleLabelHint === "string" ? parsed.defaultSubtitleLabelHint : "",
      subtitleMenuEnglishOnly: parsed.subtitleMenuEnglishOnly === true,
    };
  } catch {
    return { ...defaultPlayerWebDefaults };
  }
}

export function writeStoredPlayerWebDefaults(preferences: PlayerWebDefaults) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(playerWebDefaultsStorageKey, JSON.stringify(preferences));
  bumpPlayerLocalSettings();
}

/** Heuristic: treat as English subtitles for the “English only” player menu filter. */
export function isEnglishSubtitleTrackForMenu(track: { srcLang: string; label: string }): boolean {
  const langNorm = normalizeLanguagePreference(track.srcLang);
  if (langNorm === "en") return true;
  const label = track.label.toLowerCase();
  const rawLang = track.srcLang.trim().toLowerCase();
  const blob = `${label} ${rawLang}`;
  if (/\b(en|eng|english)\b/i.test(blob)) return true;
  if ((langNorm === "" || rawLang === "und") && /^\s*sdh\s*$/i.test(track.label.trim())) return true;
  return false;
}

export const defaultSubtitleAppearance: SubtitleAppearance = {
  size: "medium",
  position: "bottom",
  color: "#ffffff",
};

export const defaultVideoAutoplayEnabled = true;

export const subtitleSizeOptions: Array<{ value: SubtitleSize; label: string }> = [
  { value: "small", label: "Small" },
  { value: "medium", label: "Medium" },
  { value: "large", label: "Large" },
];

export const subtitlePositionOptions: Array<{ value: SubtitlePosition; label: string }> = [
  { value: "bottom", label: "Bottom" },
  { value: "top", label: "Top" },
];

export const languagePreferenceOptions: Array<{ value: string; label: string }> = [
  { value: "en", label: "English" },
  { value: "ja", label: "Japanese" },
  { value: "es", label: "Spanish" },
  { value: "fr", label: "French" },
  { value: "de", label: "German" },
  { value: "it", label: "Italian" },
  { value: "pt", label: "Portuguese" },
  { value: "ko", label: "Korean" },
  { value: "zh", label: "Chinese" },
];

const languageAliases = new Map<string, string>([
  ["en", "en"],
  ["eng", "en"],
  ["english", "en"],
  ["english (us)", "en"],
  ["english us", "en"],
  ["english (uk)", "en"],
  ["ja", "ja"],
  ["jp", "ja"],
  ["jpn", "ja"],
  ["japanese", "ja"],
  ["es", "es"],
  ["spa", "es"],
  ["spanish", "es"],
  ["fr", "fr"],
  ["fre", "fr"],
  ["fra", "fr"],
  ["french", "fr"],
  ["de", "de"],
  ["deu", "de"],
  ["ger", "de"],
  ["german", "de"],
  ["it", "it"],
  ["ita", "it"],
  ["italian", "it"],
  ["pt", "pt"],
  ["por", "pt"],
  ["portuguese", "pt"],
  ["ko", "ko"],
  ["kor", "ko"],
  ["korean", "ko"],
  ["zh", "zh"],
  ["chi", "zh"],
  ["zho", "zh"],
  ["chinese", "zh"],
]);

function normalizeIntroSkipMode(raw: string | undefined | null): IntroSkipMode {
  if (raw === "off" || raw === "auto") {
    return raw;
  }
  return "manual";
}

function defaultLibraryPreferencesForType(type: Library["type"] | undefined): ResolvedLibraryPlaybackPreferences {
  if (type === "anime") {
    return {
      preferredAudioLanguage: "ja",
      preferredSubtitleLanguage: "en",
      subtitlesEnabledByDefault: true,
      introSkipMode: "manual",
    };
  }
  if (type === "movie" || type === "tv") {
    return {
      preferredAudioLanguage: "en",
      preferredSubtitleLanguage: "en",
      subtitlesEnabledByDefault: true,
      introSkipMode: "manual",
    };
  }
  return {
    preferredAudioLanguage: "",
    preferredSubtitleLanguage: "",
    subtitlesEnabledByDefault: false,
    introSkipMode: "manual",
  };
}

export function normalizeLanguagePreference(value: string | undefined | null): string {
  const normalized = value?.trim().toLowerCase() ?? "";
  if (!normalized) return "";
  return languageAliases.get(normalized) ?? normalized.split(/[\s_-]/)[0] ?? normalized;
}

export function languageMatchesPreference(
  value: string | undefined | null,
  preferredLanguage: string | undefined | null,
): boolean {
  const normalizedValue = normalizeLanguagePreference(value);
  const normalizedPreferred = normalizeLanguagePreference(preferredLanguage);
  if (!normalizedValue || !normalizedPreferred) return false;
  return normalizedValue === normalizedPreferred;
}

export function resolveLibraryPlaybackPreferences(
  library: Pick<
    Library,
    | "type"
    | "preferred_audio_language"
    | "preferred_subtitle_language"
    | "subtitles_enabled_by_default"
    | "intro_skip_mode"
  > | null
    | undefined,
): ResolvedLibraryPlaybackPreferences {
  const defaults = defaultLibraryPreferencesForType(library?.type);
  return {
    preferredAudioLanguage: normalizeLanguagePreference(
      library?.preferred_audio_language ?? defaults.preferredAudioLanguage,
    ),
    preferredSubtitleLanguage: normalizeLanguagePreference(
      library?.preferred_subtitle_language ?? defaults.preferredSubtitleLanguage,
    ),
    subtitlesEnabledByDefault:
      library?.subtitles_enabled_by_default ?? defaults.subtitlesEnabledByDefault,
    introSkipMode: normalizeIntroSkipMode(library?.intro_skip_mode ?? defaults.introSkipMode),
  };
}

export function readStoredSubtitleAppearance(): SubtitleAppearance {
  if (typeof window === "undefined") return defaultSubtitleAppearance;
  try {
    const raw = window.localStorage.getItem(subtitleAppearanceStorageKey);
    if (!raw) return defaultSubtitleAppearance;
    const parsed = JSON.parse(raw) as Partial<SubtitleAppearance>;
    const size = parsed.size === "small" || parsed.size === "large" ? parsed.size : "medium";
    const position = parsed.position === "top" ? "top" : "bottom";
    const color = typeof parsed.color === "string" && parsed.color.trim() ? parsed.color : "#ffffff";
    return { size, position, color };
  } catch {
    return defaultSubtitleAppearance;
  }
}

export function writeStoredSubtitleAppearance(preferences: SubtitleAppearance) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(subtitleAppearanceStorageKey, JSON.stringify(preferences));
  bumpPlayerLocalSettings();
}

export function readStoredVideoAutoplayEnabled(): boolean {
  if (typeof window === "undefined") return defaultVideoAutoplayEnabled;
  const stored = window.localStorage.getItem(videoAutoplayStorageKey);
  if (stored == null) return defaultVideoAutoplayEnabled;
  return stored !== "false";
}

export function writeStoredVideoAutoplayEnabled(enabled: boolean) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(videoAutoplayStorageKey, String(enabled));
  bumpPlayerLocalSettings();
}

export function readStoredVideoAspectMode(): VideoAspectMode {
  if (typeof window === "undefined") return defaultVideoAspectMode;
  try {
    const raw = window.localStorage.getItem(videoAspectModeStorageKey);
    return normalizeVideoAspectMode(raw);
  } catch {
    return defaultVideoAspectMode;
  }
}

export function writeStoredVideoAspectMode(mode: VideoAspectMode) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(videoAspectModeStorageKey, mode);
  bumpPlayerLocalSettings();
}

export function getPlayerLocalSettingsSnapshot(): PlayerLocalSettingsSnapshot {
  const next: PlayerLocalSettingsSnapshot = {
    subtitleAppearance: readStoredSubtitleAppearance(),
    webDefaults: readStoredPlayerWebDefaults(),
    videoAutoplayEnabled: readStoredVideoAutoplayEnabled(),
    videoAspectMode: readStoredVideoAspectMode(),
    showTrackDefaultsRevision: getShowTrackDefaultsRevision(),
  };
  if (
    cachedPlayerLocalSettingsSnapshot != null &&
    playerLocalSettingsSnapshotsContentEqual(cachedPlayerLocalSettingsSnapshot, next)
  ) {
    return cachedPlayerLocalSettingsSnapshot;
  }
  cachedPlayerLocalSettingsSnapshot = next;
  return next;
}

export function subtitleFontSizeValue(size: SubtitleSize): string {
  switch (size) {
    case "small":
      return "1.1rem";
    case "large":
      return "1.95rem";
    default:
      return "1.45rem";
  }
}
