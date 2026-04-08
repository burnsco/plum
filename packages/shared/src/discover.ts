/**
 * TMDB ISO 3166-1 alpha-2 origin filters (`origin_country` query).
 * Must match Go `parseDiscoverOriginCountry` / `normalizeDiscoverOrigin` and Kotlin `DiscoverOrigin.normalizeKey`.
 */
export function normalizeDiscoverOriginKey(raw: string | undefined | null): string {
  if (raw == null) {
    return "";
  }
  const s = String(raw).trim().toUpperCase();
  if (s.length !== 2) {
    return "";
  }
  for (let i = 0; i < 2; i++) {
    const c = s.charCodeAt(i);
    if (c < 65 || c > 90) {
      return "";
    }
  }
  return s;
}
