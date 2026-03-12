import type { MediaItem } from "../../api";
import { languageMatchesPreference } from "../playbackPreferences";

export function shuffleQueue(items: MediaItem[], currentId: number): MediaItem[] {
  const current = items.find((item) => item.id === currentId) ?? items[0];
  const rest = items.filter((item) => item.id !== current?.id);
  for (let index = rest.length - 1; index > 0; index -= 1) {
    const swapIndex = Math.floor(Math.random() * (index + 1));
    [rest[index], rest[swapIndex]] = [rest[swapIndex], rest[index]];
  }
  return current ? [current, ...rest] : rest;
}

export function indexOfQueueItem(items: MediaItem[], itemId: number): number {
  return items.findIndex((item) => item.id === itemId);
}

export function clampVolume(volume: number): number {
  return Math.max(0, Math.min(volume, 1));
}

export function preferredInitialAudioIndex(item: MediaItem, preferredLanguage: string): number {
  if (!preferredLanguage) return -1;
  return (
    item.embeddedAudioTracks?.find(
      (track) =>
        languageMatchesPreference(track.language, preferredLanguage) ||
        languageMatchesPreference(track.title, preferredLanguage),
    )?.streamIndex ?? -1
  );
}
