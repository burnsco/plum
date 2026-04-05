import type { MediaItem } from "@/api";

function hasProviderIdentity(item: MediaItem): boolean {
  return Boolean(item.tmdb_id && item.tmdb_id > 0) || Boolean(item.tvdb_id?.trim());
}

/** Aligns with server “tracked” identify rows: needs provider match or not marked identified. */
export function mediaItemNeedsIdentificationAttention(item: MediaItem): boolean {
  if (item.metadata_review_needed === true) return true;
  const identified = item.match_status === "identified";
  return !identified || !hasProviderIdentity(item);
}
