import type { Library, RecentlyAddedEntry } from "@/api";

export function recentlyAddedEntryKey(entry: RecentlyAddedEntry): string {
  if (entry.kind === "show" && entry.show_key?.trim().length) {
    return `show:${entry.show_key}`;
  }
  if (entry.kind === "episode") {
    return `episode:${entry.media.id}`;
  }
  return `${entry.kind}:${entry.media.id}`;
}

export function buildRecentlyAddedToastLabel(entry: RecentlyAddedEntry): string {
  if (entry.kind === "show" || entry.kind === "episode") {
    const showTitle = entry.show_title?.trim() || entry.media.title.trim();
    const episodeLabel = entry.episode_label?.trim() || "";
    if (episodeLabel.length > 0) {
      return `${showTitle} (${episodeLabel})`;
    }
    return showTitle;
  }
  return entry.media.title.trim() || "Unknown title";
}

export function buildRecentlyAddedToastMessage(
  entry: RecentlyAddedEntry,
  libraries: Library[],
): string {
  const label = buildRecentlyAddedToastLabel(entry);
  const libraryName = libraries.find((library) => library.id === entry.media.library_id)?.name.trim();
  if (libraryName && libraryName.length > 0) {
    return `Ready to play in ${libraryName}: ${label}`;
  }
  return `Ready to play: ${label}`;
}
