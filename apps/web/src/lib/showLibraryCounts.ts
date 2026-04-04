/** Human-readable season/episode counts for library UI (aligned with Android TV home). */
export function formatSeasonEpisodeLibraryLine(seasons: number, episodes: number): string {
  if (seasons <= 0 && episodes <= 0) return "";
  const s =
    seasons <= 0 ? null : seasons === 1 ? "1 season" : `${seasons} seasons`;
  const e =
    episodes <= 0 ? null : episodes === 1 ? "1 episode" : `${episodes} episodes`;
  return [s, e].filter(Boolean).join(" · ");
}
