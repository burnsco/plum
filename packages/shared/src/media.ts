import { buildBackendUrl } from "./backend"

export function tmdbPosterUrl(path: string | undefined, size: 'w200' | 'w500' | 'original' = 'w200'): string {
  if (!path) return ''
  if (path.startsWith('http://') || path.startsWith('https://')) return path
  return `https://image.tmdb.org/t/p/${size}${path}`
}

export function tmdbBackdropUrl(path: string | undefined, size: 'w300' | 'w500' | 'w780' | 'original' = 'w500'): string {
  if (!path) return ''
  if (path.startsWith('http://') || path.startsWith('https://')) return path
  return `https://image.tmdb.org/t/p/${size}${path}`
}

export function resolvePosterUrl(
  posterUrl: string | undefined,
  posterPath: string | undefined,
  size: 'w200' | 'w500' | 'original' = 'w200',
  backendBaseUrl = '',
): string {
  if (posterUrl) {
    if (posterUrl.startsWith('http://') || posterUrl.startsWith('https://')) return posterUrl
    if (posterUrl.startsWith('/')) return buildBackendUrl(backendBaseUrl, posterUrl)
    return posterUrl
  }
  return tmdbPosterUrl(posterPath, size)
}

export function resolveBackdropUrl(
  backdropUrl: string | undefined,
  backdropPath: string | undefined,
  size: 'w300' | 'w500' | 'w780' | 'original' = 'w500',
  backendBaseUrl = '',
): string {
  if (backdropUrl) {
    if (backdropUrl.startsWith('http://') || backdropUrl.startsWith('https://')) return backdropUrl
    if (backdropUrl.startsWith('/')) return buildBackendUrl(backendBaseUrl, backdropUrl)
    return backdropUrl
  }
  return tmdbBackdropUrl(backdropPath, size)
}

/** TMDb person profile stills use the same CDN as posters; typical sizes: w45, w185, h632, original. */
export function tmdbProfileUrl(
  path: string | undefined,
  size: 'w45' | 'w185' | 'h632' | 'original' = 'w185',
): string {
  if (!path) return ''
  if (path.startsWith('http://') || path.startsWith('https://')) return path
  return `https://image.tmdb.org/t/p/${size}${path}`
}

export function resolveCastProfileUrl(
  profileUrl: string | undefined,
  profilePath: string | undefined,
  size: 'w45' | 'w185' | 'h632' | 'original' = 'w185',
  backendBaseUrl = '',
): string {
  if (profileUrl) {
    if (profileUrl.startsWith('http://') || profileUrl.startsWith('https://')) return profileUrl
    if (profileUrl.startsWith('/')) return buildBackendUrl(backendBaseUrl, profileUrl)
    return profileUrl
  }
  return tmdbProfileUrl(profilePath, size)
}
