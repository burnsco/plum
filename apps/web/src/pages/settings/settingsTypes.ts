import type { Library } from "@plum/contracts";

export type LibraryPlaybackPreferencesForm = {
  preferred_audio_language: string;
  preferred_subtitle_language: string;
  subtitles_enabled_by_default: boolean;
  watcher_enabled: boolean;
  watcher_mode: "auto" | "poll";
  scan_interval_minutes: number;
};

export function libraryPreferencesEqual(
  left: LibraryPlaybackPreferencesForm,
  right: LibraryPlaybackPreferencesForm,
): boolean {
  return (
    left.preferred_audio_language === right.preferred_audio_language &&
    left.preferred_subtitle_language === right.preferred_subtitle_language &&
    left.subtitles_enabled_by_default === right.subtitles_enabled_by_default &&
    left.watcher_enabled === right.watcher_enabled &&
    left.watcher_mode === right.watcher_mode &&
    left.scan_interval_minutes === right.scan_interval_minutes
  );
}

export type SettingsTab =
  | "general"
  | "playback"
  | "subtitles"
  | "server-env"
  | "admin"
  | "media-stack"
  | "arr-profiles"
  | "metadata"
  | "transcoding";

export function libraryTypeLabel(type: Library["type"]): string {
  switch (type) {
    case "movie":
      return "Movie";
    case "tv":
      return "TV";
    case "anime":
      return "Anime";
    case "music":
      return "Music";
    default:
      return type;
  }
}

/** True when the library name is just the type (e.g. "Movies" + movie) so we skip a duplicate subtitle. */
export function libraryNameRedundantWithType(name: string, type: Library["type"]): boolean {
  const n = name.trim().toLowerCase();
  switch (type) {
    case "movie":
      return n === "movie" || n === "movies";
    case "tv":
      return n === "tv" || n === "tvs" || n === "television";
    case "anime":
      return n === "anime";
    case "music":
      return n === "music";
    default:
      return false;
  }
}
