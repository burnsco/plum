import { createContext, useContext, type ReactNode } from "react";
import type { PlaybackPreferencesApi } from "./usePlaybackPreferences";

export type PlaybackPreferencesContextValue = {
  api: PlaybackPreferencesApi;
  librariesFetched: boolean;
};

const PlaybackPreferencesContext = createContext<PlaybackPreferencesContextValue | null>(
  null,
);

export function PlaybackPreferencesProvider({
  children,
  value,
}: {
  children: ReactNode;
  value: PlaybackPreferencesContextValue;
}) {
  return (
    <PlaybackPreferencesContext.Provider value={value}>
      {children}
    </PlaybackPreferencesContext.Provider>
  );
}

export function usePlayerPlaybackPreferences(): PlaybackPreferencesContextValue {
  const ctx = useContext(PlaybackPreferencesContext);
  if (!ctx) {
    throw new Error(
      "usePlayerPlaybackPreferences must be used within PlaybackPreferencesProvider",
    );
  }
  return ctx;
}
