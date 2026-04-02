import { identifyMovie, searchMovies, type MovieSearchResult } from "../api";
import { IdentifySearchDialog } from "./IdentifySearchDialog";

export interface IdentifyMovieDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  libraryId: number;
  mediaId: number;
  movieTitle: string;
  onSuccess: () => void;
}

export function IdentifyMovieDialog({
  open,
  onOpenChange,
  libraryId,
  mediaId,
  movieTitle,
  onSuccess,
}: IdentifyMovieDialogProps) {
  async function handleChoose(result: MovieSearchResult) {
    const response = await identifyMovie(libraryId, {
      mediaId,
      provider: result.Provider,
      externalId: result.ExternalID,
      tmdbId: result.Provider === "tmdb" ? Number.parseInt(result.ExternalID, 10) : undefined,
    });
    if (response.updated <= 0) {
      throw new Error("Identify failed");
    }
    onSuccess();
    onOpenChange(false);
  }

  return (
    <IdentifySearchDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Identify movie"
      description="Search for the correct movie and choose it to update this title's metadata."
      searchPlaceholder="Search for a movie..."
      initialQuery={movieTitle}
      search={searchMovies}
      choose={handleChoose}
    />
  );
}
