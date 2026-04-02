import type {
  DiscoverAcquisition,
  DiscoverBrowseCategory,
  DiscoverGenre,
  DiscoverItem,
  DiscoverLibraryMatch,
  DiscoverMediaType,
  DiscoverTitleDetails,
  DiscoverTitleVideo,
} from "@/api";

type DiscoverDateSource = Pick<DiscoverItem, "release_date" | "first_air_date">;

export const DISCOVER_MEDIA_FILTERS = ["all", "movie", "tv"] as const;

export type DiscoverMediaFilter = (typeof DISCOVER_MEDIA_FILTERS)[number];

export interface DiscoverCategoryOption {
  id: DiscoverBrowseCategory;
  label: string;
  defaultMediaType: DiscoverMediaType | "";
}

export const DISCOVER_CATEGORY_OPTIONS: DiscoverCategoryOption[] = [
  { id: "trending", label: "Trending Now", defaultMediaType: "" },
  { id: "popular-movies", label: "Popular Movies", defaultMediaType: "movie" },
  { id: "now-playing", label: "Now Playing", defaultMediaType: "movie" },
  { id: "upcoming", label: "Upcoming Movies", defaultMediaType: "movie" },
  { id: "popular-tv", label: "Popular TV", defaultMediaType: "tv" },
  { id: "on-the-air", label: "On The Air", defaultMediaType: "tv" },
  { id: "top-rated", label: "Top Rated Picks", defaultMediaType: "movie" },
];

export function discoverMediaLabel(mediaType: DiscoverMediaType): string {
  return mediaType === "movie" ? "Movie" : "TV";
}

export function discoverMediaFilterLabel(filter: DiscoverMediaFilter): string {
  switch (filter) {
    case "movie":
      return "Movies";
    case "tv":
      return "TV";
    case "all":
    default:
      return "All";
  }
}

export function discoverCategoryLabel(category: DiscoverBrowseCategory): string {
  return DISCOVER_CATEGORY_OPTIONS.find((option) => option.id === category)?.label ?? "Browse";
}

export function discoverVisibleItems(
  items: DiscoverItem[],
  mediaFilter: DiscoverMediaFilter,
): DiscoverItem[] {
  if (mediaFilter === "all") {
    return items;
  }
  return items.filter((item) => item.media_type === mediaFilter);
}

export function discoverGenresForFilter(
  movieGenres: DiscoverGenre[],
  tvGenres: DiscoverGenre[],
  mediaFilter: DiscoverMediaFilter,
): DiscoverGenre[] {
  if (mediaFilter === "movie") {
    return movieGenres;
  }
  if (mediaFilter === "tv") {
    return tvGenres;
  }
  const merged = new Map<string, DiscoverGenre>();
  for (const genre of [...movieGenres, ...tvGenres]) {
    if (!merged.has(genre.name)) {
      merged.set(genre.name, genre);
    }
  }
  return [...merged.values()];
}

export function discoverBrowseHref(options: {
  category?: DiscoverBrowseCategory | "";
  mediaType?: DiscoverMediaType | "";
  genreId?: number | null;
}): string {
  const params = new URLSearchParams();
  if (options.category) {
    params.set("category", options.category);
  }
  if (options.mediaType) {
    params.set("mediaType", options.mediaType);
  }
  if (options.genreId != null && options.genreId > 0) {
    params.set("genre", String(options.genreId));
  }
  const query = params.toString();
  return query ? `/discover/browse?${query}` : "/discover/browse";
}

export function discoverPrimaryDate(item: DiscoverDateSource): string {
  return item.release_date || item.first_air_date || "";
}

export function discoverYear(item: DiscoverDateSource): string {
  return discoverPrimaryDate(item).split("-")[0] || "";
}

export function discoverLibraryHref(match: DiscoverLibraryMatch): string {
  if (match.kind === "show" && match.show_key) {
    return `/library/${match.library_id}/show/${match.show_key}`;
  }
  return `/library/${match.library_id}`;
}

export function firstDiscoverMatch(
  matches?: DiscoverLibraryMatch[],
): DiscoverLibraryMatch | undefined {
  return matches?.[0];
}

export function discoverDetailMeta(details: DiscoverTitleDetails): string[] {
  const meta: string[] = [];
  const year = discoverYear(details);
  if (year) {
    meta.push(year);
  }
  if (details.status) {
    meta.push(details.status);
  }
  if (details.media_type === "movie" && details.runtime) {
    meta.push(`${details.runtime} min`);
  }
  if (details.media_type === "tv") {
    if (details.number_of_seasons) {
      meta.push(
        `${details.number_of_seasons} season${details.number_of_seasons === 1 ? "" : "s"}`,
      );
    }
    if (details.runtime) {
      meta.push(`${details.runtime} min episodes`);
    }
  }
  return meta;
}

export function discoverVideoUrl(video: DiscoverTitleVideo): string {
  if (video.site === "YouTube") {
    return `https://www.youtube.com/watch?v=${video.key}`;
  }
  if (video.site === "Vimeo") {
    return `https://vimeo.com/${video.key}`;
  }
  return "";
}

export function discoverAcquisitionLabel(
  acquisition: DiscoverAcquisition | undefined,
  pending = false,
): string {
  if (pending) {
    return "Adding...";
  }
  switch (acquisition?.state) {
    case "available":
      return "In Library";
    case "downloading":
      return "Downloading";
    case "added":
      return "Added";
    case "not_added":
    default:
      return acquisition?.is_configured === false ? "Unavailable" : "Add";
  }
}

export function discoverAcquisitionTone(
  acquisition: DiscoverAcquisition | undefined,
  pending = false,
): "default" | "success" | "muted" {
  if (pending) {
    return "default";
  }
  switch (acquisition?.state) {
    case "available":
    case "downloading":
    case "added":
      return "success";
    case "not_added":
    default:
      return acquisition?.is_configured === false ? "muted" : "default";
  }
}
