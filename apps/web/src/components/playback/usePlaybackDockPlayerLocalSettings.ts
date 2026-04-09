import { useCallback, useSyncExternalStore } from "react";
import {
  getPlayerLocalSettingsSnapshot,
  subscribePlayerLocalSettings,
  writeStoredSubtitleAppearance,
  writeStoredVideoAutoplayEnabled,
  writeStoredVideoAspectMode,
  type SubtitleAppearance,
  type VideoAspectMode,
} from "../../lib/playbackPreferences";

export function usePlaybackDockPlayerLocalSettings() {
  const playerLocalSettings = useSyncExternalStore(
    subscribePlayerLocalSettings,
    getPlayerLocalSettingsSnapshot,
    getPlayerLocalSettingsSnapshot,
  );
  const subtitleAppearance = playerLocalSettings.subtitleAppearance;
  const setSubtitleAppearance = useCallback((value: SubtitleAppearance) => {
    writeStoredSubtitleAppearance(value);
  }, []);
  const videoAutoplayEnabled = playerLocalSettings.videoAutoplayEnabled;
  const setVideoAutoplayEnabled = useCallback(
    (value: boolean | ((prev: boolean) => boolean)) => {
      const next =
        typeof value === "function"
          ? value(getPlayerLocalSettingsSnapshot().videoAutoplayEnabled)
          : value;
      writeStoredVideoAutoplayEnabled(next);
    },
    [],
  );
  const videoAspectMode = playerLocalSettings.videoAspectMode;
  const setVideoAspectMode = useCallback((value: VideoAspectMode) => {
    writeStoredVideoAspectMode(value);
  }, []);

  return {
    playerLocalSettings,
    subtitleAppearance,
    setSubtitleAppearance,
    videoAutoplayEnabled,
    setVideoAutoplayEnabled,
    videoAspectMode,
    setVideoAspectMode,
  };
}
