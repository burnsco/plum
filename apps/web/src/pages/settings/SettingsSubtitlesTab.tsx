import { useEffect, useMemo, useState } from "react";
import {
  languagePreferenceOptions,
  normalizeLanguagePreference,
  PLAYER_WEB_TRACK_LANGUAGE_NONE,
  readStoredPlayerWebDefaults,
  readStoredSubtitleAppearance,
  subscribePlayerLocalSettings,
  subtitlePositionOptions,
  subtitleSizeOptions,
  writeStoredPlayerWebDefaults,
  writeStoredSubtitleAppearance,
} from "@/lib/playbackPreferences";
import { Toggle } from "./settingsControls";

export function SettingsSubtitlesTab() {
  const [appearance, setAppearance] = useState(() => readStoredSubtitleAppearance());
  const [webDefaults, setWebDefaults] = useState(() => readStoredPlayerWebDefaults());

  useEffect(() => {
    return subscribePlayerLocalSettings(() => {
      setAppearance(readStoredSubtitleAppearance());
      setWebDefaults(readStoredPlayerWebDefaults());
    });
  }, []);

  const languageOptsWithDefault = useMemo(
    () => [
      { value: "", label: "Use library default" },
      { value: PLAYER_WEB_TRACK_LANGUAGE_NONE, label: "Don't auto-pick" },
      ...languagePreferenceOptions,
    ],
    [],
  );

  return (
    <section className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4 shadow-[0_20px_45px_rgba(0,0,0,0.35)]">
      <div className="flex flex-col gap-2">
        <h2 className="text-xl font-semibold text-(--plum-text)">Subtitles &amp; web player</h2>
        <p className="max-w-2xl text-sm text-(--plum-muted)">
          Subtitle appearance, automatic track picks, and the subtitle track list apply only in this
          browser. Per-library defaults on the Playback tab still control server-side behavior unless
          you override them here.
        </p>
      </div>

      <article className="mt-8 rounded-md border border-(--plum-border) bg-(--plum-panel-alt)/60 p-4">
        <h3 className="text-base font-medium text-(--plum-text)">Web player on this device</h3>
        <p className="mt-1 max-w-2xl text-sm text-(--plum-muted)">
          Tune how subtitles look and which tracks the player prefers when you start playback.
        </p>

        <div className="mt-5 grid gap-5 md:grid-cols-2 xl:grid-cols-3">
          <div>
            <span className="mb-2 block text-sm font-medium text-(--plum-text)">Subtitle size</span>
            <div className="flex flex-wrap gap-2">
              {subtitleSizeOptions.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  onClick={() => {
                    const next = { ...appearance, size: option.value };
                    setAppearance(next);
                    writeStoredSubtitleAppearance(next);
                  }}
                  className={`rounded-md border px-3 py-1.5 text-sm transition-colors ${
                    appearance.size === option.value
                      ? "border-(--plum-ring) bg-(--plum-panel) text-(--plum-text)"
                      : "border-(--plum-border) bg-(--plum-panel)/60 text-(--plum-muted) hover:text-(--plum-text)"
                  }`}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>

          <div>
            <span className="mb-2 block text-sm font-medium text-(--plum-text)">Subtitle position</span>
            <div className="flex flex-wrap gap-2">
              {subtitlePositionOptions.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  onClick={() => {
                    const next = { ...appearance, position: option.value };
                    setAppearance(next);
                    writeStoredSubtitleAppearance(next);
                  }}
                  className={`rounded-md border px-3 py-1.5 text-sm transition-colors ${
                    appearance.position === option.value
                      ? "border-(--plum-ring) bg-(--plum-panel) text-(--plum-text)"
                      : "border-(--plum-border) bg-(--plum-panel)/60 text-(--plum-muted) hover:text-(--plum-text)"
                  }`}
                >
                  {option.label}
                </button>
              ))}
            </div>
          </div>

          <div>
            <label
              className="mb-2 block text-sm font-medium text-(--plum-text)"
              htmlFor="web-player-subtitle-color"
            >
              Subtitle color
            </label>
            <input
              id="web-player-subtitle-color"
              type="color"
              value={appearance.color}
              onChange={(event) => {
                const next = { ...appearance, color: event.target.value };
                setAppearance(next);
                writeStoredSubtitleAppearance(next);
              }}
              className="h-9 w-full max-w-[8rem] cursor-pointer rounded border border-(--plum-border) bg-(--plum-panel)"
            />
          </div>

          <div>
            <label
              className="mb-2 block text-sm font-medium text-(--plum-text)"
              htmlFor="web-player-default-sub-lang"
            >
              Default subtitle language
            </label>
            <select
              id="web-player-default-sub-lang"
              value={webDefaults.defaultSubtitleLanguage}
              onChange={(event) => {
                const raw = event.target.value;
                const next = {
                  ...webDefaults,
                  defaultSubtitleLanguage:
                    raw === PLAYER_WEB_TRACK_LANGUAGE_NONE
                      ? PLAYER_WEB_TRACK_LANGUAGE_NONE
                      : normalizeLanguagePreference(raw),
                  defaultSubtitleLabelHint: "",
                };
                setWebDefaults(next);
                writeStoredPlayerWebDefaults(next);
              }}
              className="flex h-9 w-full rounded-md border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
            >
              {languageOptsWithDefault.map((option) => (
                <option key={`sub-${option.value || "library"}`} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <p className="mt-1.5 text-xs text-(--plum-muted)">
              Used when a new video starts. Library default follows server settings per library.
              Don&apos;t auto-pick keeps subtitles off until you choose one.
            </p>
          </div>

          <div>
            <label
              className="mb-2 block text-sm font-medium text-(--plum-text)"
              htmlFor="web-player-default-audio-lang"
            >
              Default audio language
            </label>
            <select
              id="web-player-default-audio-lang"
              value={webDefaults.defaultAudioLanguage}
              onChange={(event) => {
                const raw = event.target.value;
                const next = {
                  ...webDefaults,
                  defaultAudioLanguage:
                    raw === PLAYER_WEB_TRACK_LANGUAGE_NONE
                      ? PLAYER_WEB_TRACK_LANGUAGE_NONE
                      : normalizeLanguagePreference(raw),
                };
                setWebDefaults(next);
                writeStoredPlayerWebDefaults(next);
              }}
              className="flex h-9 w-full rounded-md border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
            >
              {languageOptsWithDefault.map((option) => (
                <option key={`a-${option.value || "library"}`} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <p className="mt-1.5 text-xs text-(--plum-muted)">
              Used when multiple audio tracks exist. Library default follows server settings. Don&apos;t
              auto-pick leaves the stream&apos;s default track until you switch manually.
            </p>
          </div>

          <div className="flex items-end md:col-span-2 xl:col-span-1">
            <Toggle
              label="English subtitles only in player menu"
              checked={webDefaults.subtitleMenuEnglishOnly}
              onChange={(checked) => {
                const next = { ...webDefaults, subtitleMenuEnglishOnly: checked };
                setWebDefaults(next);
                writeStoredPlayerWebDefaults(next);
              }}
              description="Hides non-English tracks in the fullscreen subtitle list (eng, English, SDH, etc.). Off and the current track stay available."
            />
          </div>
        </div>
      </article>
    </section>
  );
}
