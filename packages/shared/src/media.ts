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
