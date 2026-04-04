export type RatingDisplay = {
  label?: "IMDb" | "TMDb";
  value?: number;
};

export function getPreferredMovieRating(item: {
  imdb_rating?: number;
  vote_average?: number;
}): RatingDisplay {
  if ((item.imdb_rating ?? 0) > 0) {
    return { label: "IMDb", value: item.imdb_rating };
  }
  if ((item.vote_average ?? 0) > 0) {
    return { label: "TMDb", value: item.vote_average };
  }
  return { label: undefined, value: undefined };
}

/** TV/anime browse rows carry series scores on `show_*` fields; per-episode ratings are omitted. */
export function getPreferredShowRatingFromBrowseEpisode(item: {
  show_imdb_rating?: number;
  show_vote_average?: number;
}): RatingDisplay {
  if ((item.show_imdb_rating ?? 0) > 0) {
    return { label: "IMDb", value: item.show_imdb_rating };
  }
  if ((item.show_vote_average ?? 0) > 0) {
    return { label: "TMDb", value: item.show_vote_average };
  }
  return { label: undefined, value: undefined };
}
