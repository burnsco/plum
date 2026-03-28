import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  useMoviePosterCandidates,
  useResetMoviePosterSelection,
  useResetShowPosterSelection,
  useSetMoviePosterSelection,
  useSetShowPosterSelection,
  useShowPosterCandidates,
} from "@/queries";

type MoviePosterPickerDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  kind: "movie";
  libraryId: number;
  mediaId: number;
  title: string;
};

type ShowPosterPickerDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  kind: "show";
  libraryId: number;
  showKey: string;
  title: string;
};

type PosterPickerDialogProps = MoviePosterPickerDialogProps | ShowPosterPickerDialogProps;

export function PosterPickerDialog(props: PosterPickerDialogProps) {
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const movieCandidatesQuery = useMoviePosterCandidates(
    props.kind === "movie" ? props.libraryId : null,
    props.kind === "movie" ? props.mediaId : null,
    { enabled: props.open && props.kind === "movie" },
  );
  const showCandidatesQuery = useShowPosterCandidates(
    props.kind === "show" ? props.libraryId : null,
    props.kind === "show" ? props.showKey : null,
    { enabled: props.open && props.kind === "show" },
  );
  const setMoviePoster = useSetMoviePosterSelection();
  const resetMoviePoster = useResetMoviePosterSelection();
  const setShowPoster = useSetShowPosterSelection();
  const resetShowPoster = useResetShowPosterSelection();

  const data = props.kind === "movie" ? movieCandidatesQuery.data : showCandidatesQuery.data;
  const isLoading =
    props.kind === "movie" ? movieCandidatesQuery.isLoading : showCandidatesQuery.isLoading;
  const pending =
    setMoviePoster.isPending ||
    resetMoviePoster.isPending ||
    setShowPoster.isPending ||
    resetShowPoster.isPending;

  const unavailableProviders = useMemo(
    () =>
      (data?.provider_availability ?? []).filter((provider) => !provider.available && provider.reason),
    [data?.provider_availability],
  );

  async function handleSelect(sourceUrl: string) {
    setErrorMessage(null);
    try {
      if (props.kind === "movie") {
        await setMoviePoster.mutateAsync({
          libraryId: props.libraryId,
          mediaId: props.mediaId,
          sourceUrl,
        });
      } else {
        await setShowPoster.mutateAsync({
          libraryId: props.libraryId,
          showKey: props.showKey,
          sourceUrl,
        });
      }
      props.onOpenChange(false);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "Failed to change poster.");
    }
  }

  async function handleReset() {
    setErrorMessage(null);
    try {
      if (props.kind === "movie") {
        await resetMoviePoster.mutateAsync({
          libraryId: props.libraryId,
          mediaId: props.mediaId,
        });
      } else {
        await resetShowPoster.mutateAsync({
          libraryId: props.libraryId,
          showKey: props.showKey,
        });
      }
      props.onOpenChange(false);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "Failed to reset poster.");
    }
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className="max-w-4xl" onClose={() => props.onOpenChange(false)}>
        <DialogHeader>
          <DialogTitle>Change poster</DialogTitle>
          <DialogDescription>{props.title}</DialogDescription>
        </DialogHeader>

        {isLoading ? (
          <div className="py-10 text-sm text-[var(--plum-muted)]">Loading poster options…</div>
        ) : data == null ? (
          <div className="py-10 text-sm text-[var(--plum-muted)]">No poster options available.</div>
        ) : (
          <div className="space-y-4">
            <div className="flex items-center justify-between gap-3">
              <div className="text-sm text-[var(--plum-muted)]">
                {data.has_custom_selection
                  ? "A custom poster is pinned for this title."
                  : "Automatic poster selection is active."}
              </div>
              <Button
                type="button"
                variant="secondary"
                onClick={() => void handleReset()}
                disabled={pending}
              >
                Reset to automatic
              </Button>
            </div>

            {data.candidates.length === 0 ? (
              <div className="rounded-[var(--radius-md)] border border-dashed border-[var(--plum-border)] px-4 py-6 text-sm text-[var(--plum-muted)]">
                No poster candidates were found for this title.
              </div>
            ) : (
              <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
                {data.candidates.map((candidate) => (
                  <button
                    key={candidate.id}
                    type="button"
                    className={`group overflow-hidden rounded-[var(--radius-lg)] border text-left transition-colors ${
                      candidate.selected
                        ? "border-[var(--plum-accent)] bg-[var(--plum-accent-soft)]/30"
                        : "border-[var(--plum-border)] bg-[var(--plum-panel)] hover:border-[var(--plum-accent-soft)]"
                    }`}
                    onClick={() => void handleSelect(candidate.source_url)}
                    disabled={pending}
                  >
                    <div className="aspect-[2/3] bg-black/20">
                      <img
                        src={candidate.image_url}
                        alt={`${props.title} poster from ${candidate.label}`}
                        className="size-full object-cover"
                      />
                    </div>
                    <div className="space-y-1 px-3 py-2">
                      <div className="text-sm font-medium text-[var(--plum-text)]">
                        {candidate.label}
                      </div>
                      <div className="text-xs text-[var(--plum-muted)]">
                        {candidate.selected ? "Current selection" : "Use this poster"}
                      </div>
                    </div>
                  </button>
                ))}
              </div>
            )}

            {unavailableProviders.length > 0 ? (
              <div className="text-xs text-[var(--plum-muted)]">
                Unavailable:{" "}
                {unavailableProviders
                  .map((provider) => `${provider.provider} (${provider.reason})`)
                  .join(" • ")}
              </div>
            ) : null}

            {errorMessage ? <div className="text-sm text-rose-300">{errorMessage}</div> : null}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
