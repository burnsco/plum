import { useCallback, useMemo } from "react";
import { embeddedSubtitleNeedsWebBurnIn } from "@plum/shared";
import type { Library, MediaItem } from "../api";
import {
  normalizeLanguagePreference,
  PLAYER_WEB_TRACK_LANGUAGE_NONE,
  readStoredPlayerWebDefaults,
  resolveEffectivePreferredAudioLanguage,
  resolveEffectiveWebTrackDefaults,
  resolveLibraryPlaybackPreferences,
  type ResolvedLibraryPlaybackPreferences,
} from "../lib/playbackPreferences";
import { preferredInitialAudioIndex } from "../lib/playback/playerQueue";
import {
  formatTrackLabel,
  getPreferredSubtitleKey,
  type SubtitleTrackOption,
} from "../lib/playback/playerMedia";

function buildInitialSubtitleTrackOptions(item: MediaItem): SubtitleTrackOption[] {
  const embedded =
    item.embeddedSubtitles?.map((subtitle, index) => {
      const requiresBurn = embeddedSubtitleNeedsWebBurnIn(subtitle);
      return {
        key: `emb-${subtitle.streamIndex}`,
        label: formatTrackLabel(
          subtitle.title,
          subtitle.language,
          `Embedded subtitle ${index + 1}`,
        ),
        src: "",
        srcLang: subtitle.language || "und",
        supported: subtitle.supported !== false,
        requiresBurn,
      };
    }) ?? [];
  return embedded;
}

/** Pure helper: initial PGS burn stream from library + stored web defaults + item tracks. */
export function resolveInitialBurnSubtitleStreamIndex(
  item: MediaItem,
  libraryPrefs: ResolvedLibraryPlaybackPreferences,
): number | null {
  const effectiveDefaults = resolveEffectiveWebTrackDefaults(item, readStoredPlayerWebDefaults());
  const subtitlesDisabledByClient =
    effectiveDefaults.defaultSubtitleLanguage.trim() === PLAYER_WEB_TRACK_LANGUAGE_NONE;
  const subtitlesEnabled = !subtitlesDisabledByClient && libraryPrefs.subtitlesEnabledByDefault;
  if (!subtitlesEnabled) return null;

  const preferredSubtitleLanguageRaw =
    effectiveDefaults.defaultSubtitleLanguage.trim() !== ""
      ? effectiveDefaults.defaultSubtitleLanguage
      : libraryPrefs.preferredSubtitleLanguage;
  const preferredSubtitleLanguage = normalizeLanguagePreference(preferredSubtitleLanguageRaw);
  if (preferredSubtitleLanguage === "") return null;

  const subtitleLabelHint =
    effectiveDefaults.defaultSubtitleLanguage.trim() !== ""
      ? effectiveDefaults.defaultSubtitleLabelHint.trim()
      : "";
  const preferredSubtitleKey = getPreferredSubtitleKey(
    buildInitialSubtitleTrackOptions(item),
    preferredSubtitleLanguage,
    true,
    subtitleLabelHint,
  );
  if (!preferredSubtitleKey.startsWith("emb-")) return null;
  const streamIndex = Number(preferredSubtitleKey.slice(4));
  if (!Number.isFinite(streamIndex)) return null;
  const selected = item.embeddedSubtitles?.find((track) => track.streamIndex === streamIndex);
  if (!selected || !embeddedSubtitleNeedsWebBurnIn(selected)) {
    return null;
  }
  return streamIndex;
}

export type PlaybackPreferencesApi = {
  libraryPrefsForItem: (item: MediaItem) => ResolvedLibraryPlaybackPreferences;
  effectivePreferredAudioLanguage: (item: MediaItem) => string;
  initialAudioStreamIndex: (item: MediaItem) => number;
  initialBurnEmbeddedSubtitleStreamIndex: (item: MediaItem) => number | null;
  /** Library-only preferred language for audio when recreating session after subtitle burn change. */
  audioIndexForSubtitleBurnChange: (
    item: MediaItem,
    session: { audioIndex: number } | null | undefined,
  ) => number;
};

export function usePlaybackPreferences(libraries: Library[]): PlaybackPreferencesApi {
  const libraryPrefsForItem = useCallback(
    (item: MediaItem): ResolvedLibraryPlaybackPreferences => {
      const activeLibrary = libraries.find((library) => library.id === item.library_id) ?? null;
      return resolveLibraryPlaybackPreferences(activeLibrary ?? { type: item.type });
    },
    [libraries],
  );

  const effectivePreferredAudioLanguage = useCallback(
    (item: MediaItem): string =>
      resolveEffectivePreferredAudioLanguage(item, libraryPrefsForItem(item)),
    [libraryPrefsForItem],
  );

  const initialAudioStreamIndex = useCallback(
    (item: MediaItem): number =>
      preferredInitialAudioIndex(item, effectivePreferredAudioLanguage(item)),
    [effectivePreferredAudioLanguage],
  );

  const initialBurnEmbeddedSubtitleStreamIndex = useCallback(
    (item: MediaItem): number | null =>
      resolveInitialBurnSubtitleStreamIndex(item, libraryPrefsForItem(item)),
    [libraryPrefsForItem],
  );

  const audioIndexForSubtitleBurnChange = useCallback(
    (item: MediaItem, session: { audioIndex: number } | null | undefined): number =>
      session != null
        ? session.audioIndex
        : preferredInitialAudioIndex(item, libraryPrefsForItem(item).preferredAudioLanguage),
    [libraryPrefsForItem],
  );

  return useMemo(
    () => ({
      libraryPrefsForItem,
      effectivePreferredAudioLanguage,
      initialAudioStreamIndex,
      initialBurnEmbeddedSubtitleStreamIndex,
      audioIndexForSubtitleBurnChange,
    }),
    [
      libraryPrefsForItem,
      effectivePreferredAudioLanguage,
      initialAudioStreamIndex,
      initialBurnEmbeddedSubtitleStreamIndex,
      audioIndexForSubtitleBurnChange,
    ],
  );
}
