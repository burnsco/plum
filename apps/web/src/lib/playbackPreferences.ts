import type { IntroSkipMode, Library } from "../api";

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
export const playerWebDefaultsStorageKey = "plum:player-web-defaults";

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
  /** Limit the in-player subtitle menu to tracks that look English (eng, SDH, etc.). */
  subtitleMenuEnglishOnly: boolean;
};

const defaultPlayerWebDefaults: PlayerWebDefaults = {
  defaultAudioLanguage: "",
  defaultSubtitleLanguage: "",
  subtitleMenuEnglishOnly: false,
};

function bumpPlayerLocalSettings() {
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
    if (
      event.key === subtitleAppearanceStorageKey ||
      event.key === videoAutoplayStorageKey ||
      event.key === playerWebDefaultsStorageKey
    ) {
      onStoreChange();
    }
  };
  window.addEventListener("storage", onStorage);
  return () => {
    window.removeEventListener(PLAYER_LOCAL_SETTINGS_EVENT, handler);
    window.removeEventListener("storage", onStorage);
  };
}

export type PlayerLocalSettingsSnapshot = {
  subtitleAppearance: SubtitleAppearance;
  webDefaults: PlayerWebDefaults;
  videoAutoplayEnabled: boolean;
};

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

export function getPlayerLocalSettingsSnapshot(): PlayerLocalSettingsSnapshot {
  return {
    subtitleAppearance: readStoredSubtitleAppearance(),
    webDefaults: readStoredPlayerWebDefaults(),
    videoAutoplayEnabled: readStoredVideoAutoplayEnabled(),
  };
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
