import type { MediaItem } from "@/api";

const mediaItemScalarDefaults: Pick<
  MediaItem,
  "library_id" | "tmdb_id" | "overview" | "poster_path" | "backdrop_path" | "release_date" | "vote_average"
> = {
  library_id: 0,
  tmdb_id: 0,
  overview: "",
  poster_path: "",
  backdrop_path: "",
  release_date: "",
  vote_average: 0,
};

type MediaItemScalars = typeof mediaItemScalarDefaults;

/** Test/builder helper: fills scalar fields the server always sends on full `MediaItem` JSON. */
export function makeMediaItem(
  item: Omit<MediaItem, keyof MediaItemScalars> & Partial<MediaItemScalars>,
): MediaItem {
  return { ...mediaItemScalarDefaults, ...item };
}
