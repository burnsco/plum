import { useQueryClient } from "@tanstack/react-query";
import { useIdentifyQueue } from "../contexts/IdentifyQueueContext";
import { identifyShow, searchSeries, type SeriesSearchResult } from "../api";
import { queryKeys } from "@/queries";
import { IdentifySearchDialog } from "./IdentifySearchDialog";

export interface IdentifyShowDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  libraryId: number;
  showKey: string;
  showTitle: string;
  onSuccess: () => void;
}

export function IdentifyShowDialog({
  open,
  onOpenChange,
  libraryId,
  showKey,
  showTitle,
  onSuccess,
}: IdentifyShowDialogProps) {
  const queryClient = useQueryClient();
  const { queueLibraryIdentify } = useIdentifyQueue();

  async function handleChoose(result: SeriesSearchResult) {
    const response = await identifyShow(libraryId, showKey, {
      provider: result.Provider,
      externalId: result.ExternalID,
    });
    if (response.updated <= 0) {
      throw new Error("Identify failed");
    }
    queueLibraryIdentify(libraryId, {
      abortActive: true,
      prioritize: true,
      resetState: true,
    });
    void queryClient.invalidateQueries({ queryKey: queryKeys.unidentifiedSummary });
    onSuccess();
    onOpenChange(false);
  }

  return (
    <IdentifySearchDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Identify show"
      description="Search for the correct series and choose it to update this show's metadata."
      searchPlaceholder="Search for a TV series..."
      initialQuery={showTitle}
      search={searchSeries}
      choose={handleChoose}
    />
  );
}
