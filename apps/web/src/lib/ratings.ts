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
