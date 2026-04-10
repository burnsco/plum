import type { MediaItem } from "../api";

export type PlaybackKind = "video" | "music";
/** Video is always `window` (theater: fixed overlay filling the viewport). Display fullscreen uses the Fullscreen API separately. Music ignores layout and uses the in-page bar on the music library view. */
export type PlayerViewMode = "window";
export type MusicRepeatMode = "off" | "all" | "one";
export type MediaElementSlot = "audio" | "video";

export type PlaybackSession = {
  activeMode: PlaybackKind;
  isDockOpen: boolean;
  viewMode: PlayerViewMode;
  queue: MediaItem[];
  queueIndex: number;
  shuffle: boolean;
  repeatMode: MusicRepeatMode;
  /**
   * User already chose to resume from Home "Continue watching" — skip the in-player resume/restart
   * overlay and seek to [MediaItem.progress_seconds] immediately (including server seek for transcodes).
   */
  resumeIntent?: "continue_watching";
};
