import { useIdentifyQueue } from "../contexts/IdentifyQueueContext";
import { identifyShow, searchSeries, type SeriesSearchResult } from "../api";
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
