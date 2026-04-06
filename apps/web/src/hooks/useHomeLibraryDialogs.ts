import { useState } from "react";
import type { ShowGroup } from "@/lib/showGrouping";

export type PosterPickerState =
  | { kind: "movie"; libraryId: number; mediaId: number; title: string }
  | { kind: "show"; libraryId: number; showKey: string; title: string };

export function useHomeLibraryDialogs() {
  const [identifyGroup, setIdentifyGroup] = useState<ShowGroup | null>(null);
  const [identifyMovieItem, setIdentifyMovieItem] = useState<{ id: number; title: string } | null>(null);
  const [posterPicker, setPosterPicker] = useState<PosterPickerState | null>(null);

  return {
    identifyGroup,
    setIdentifyGroup,
    identifyMovieItem,
    setIdentifyMovieItem,
    posterPicker,
    setPosterPicker,
  };
}
