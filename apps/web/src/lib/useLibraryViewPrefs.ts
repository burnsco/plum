import { useCallback, useState } from "react";
import type { LayoutMode } from "@/components/LibraryViewControls";

const STORAGE_KEY_CARD_WIDTH = "plum:library:cardWidth";
const STORAGE_KEY_LAYOUT_MODE = "plum:library:layoutMode";
const DEFAULT_CARD_WIDTH = 180;
const DEFAULT_LAYOUT_MODE: LayoutMode = "grid";

function readStoredNumber(key: string, fallback: number): number {
  try {
    const raw = localStorage.getItem(key);
    if (raw === null) return fallback;
    const parsed = Number(raw);
    return Number.isFinite(parsed) ? parsed : fallback;
  } catch {
    return fallback;
  }
}

function readStoredLayoutMode(key: string, fallback: LayoutMode): LayoutMode {
  try {
    const raw = localStorage.getItem(key);
    if (raw === "grid" || raw === "detail" || raw === "table") return raw;
    return fallback;
  } catch {
    return fallback;
  }
}

/** Persists library card size and layout mode in localStorage */
export function useLibraryViewPrefs() {
  const [cardWidth, setCardWidthState] = useState<number>(() =>
    readStoredNumber(STORAGE_KEY_CARD_WIDTH, DEFAULT_CARD_WIDTH),
  );
  const [layoutMode, setLayoutModeState] = useState<LayoutMode>(() =>
    readStoredLayoutMode(STORAGE_KEY_LAYOUT_MODE, DEFAULT_LAYOUT_MODE),
  );

  const setCardWidth = useCallback((width: number) => {
    setCardWidthState(width);
    try {
      localStorage.setItem(STORAGE_KEY_CARD_WIDTH, String(width));
    } catch {
      // ignore storage errors
    }
  }, []);

  const setLayoutMode = useCallback((mode: LayoutMode) => {
    setLayoutModeState(mode);
    try {
      localStorage.setItem(STORAGE_KEY_LAYOUT_MODE, mode);
    } catch {
      // ignore storage errors
    }
  }, []);

  return { cardWidth, setCardWidth, layoutMode, setLayoutMode };
}
