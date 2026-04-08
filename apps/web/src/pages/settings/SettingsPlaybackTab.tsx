import type { Library } from "@plum/contracts";
import { Button } from "@/components/ui/button";
import {
  languagePreferenceOptions,
  normalizeLanguagePreference,
} from "@/lib/playbackPreferences";
import type { UseQueryResult } from "@tanstack/react-query";
import { SettingsInputRow, SettingsSelectRow, SettingsToggleRow } from "./SettingsRows";
import {
  libraryNameRedundantWithType,
  libraryPreferencesEqual,
  libraryTypeLabel,
  type LibraryPlaybackPreferencesForm,
} from "./settingsTypes";

export function SettingsPlaybackTab({
  librariesQuery,
  libraries,
  libraryForms,
  librarySaveMessages,
  savingLibraryId,
  cloneLibraryPlaybackPreferences,
  setLibraryField,
  saveLibraryPreferences,
}: {
  librariesQuery: Pick<UseQueryResult<Library[]>, "isLoading" | "isError" | "error">;
  libraries: Library[];
  libraryForms: Record<number, LibraryPlaybackPreferencesForm>;
  librarySaveMessages: Record<number, string | null>;
  savingLibraryId: number | null;
  cloneLibraryPlaybackPreferences: (library: Library) => LibraryPlaybackPreferencesForm;
  setLibraryField: <K extends keyof LibraryPlaybackPreferencesForm>(
    libraryId: number,
    key: K,
    value: LibraryPlaybackPreferencesForm[K],
  ) => void;
  saveLibraryPreferences: (library: Library) => Promise<void>;
}) {
  return (
    <section className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4 shadow-[0_20px_45px_rgba(0,0,0,0.35)]">
      <div className="flex flex-col gap-2">
        <h2 className="text-xl font-semibold text-(--plum-text)">Playback defaults</h2>
        <p className="max-w-2xl text-sm text-(--plum-muted)">
          Choose the default playback behavior and scan automation for each library. Anime libraries
          default to Japanese audio with English subtitles; TV and movie libraries default to English
          for both when available.
        </p>
      </div>

      {librariesQuery.isLoading ? (
        <p className="mt-5 text-sm text-(--plum-muted)">Loading libraries…</p>
      ) : librariesQuery.isError ? (
        <p className="mt-5 text-sm text-red-300">
          {librariesQuery.error?.message || "Failed to load libraries."}
        </p>
      ) : libraries.length === 0 ? (
        <p className="mt-5 text-sm text-(--plum-muted)">
          Add a library to configure playback defaults and automation.
        </p>
      ) : (
        <div className="mt-6 grid gap-4">
          {libraries.map((library) => {
            const current = libraryForms[library.id] ?? cloneLibraryPlaybackPreferences(library);
            const saved = cloneLibraryPlaybackPreferences(library);
            const isDirty = !libraryPreferencesEqual(current, saved);
            const message = librarySaveMessages[library.id];
            const supportsPlaybackPreferences = library.type !== "music";
            const hideTypeSubtitle = libraryNameRedundantWithType(library.name, library.type);

            return (
              <article
                key={library.id}
                className="rounded-md border border-(--plum-border) bg-(--plum-panel-alt)/60 p-4"
              >
                <div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
                  <div>
                    <h3 className="text-base font-medium text-(--plum-text)">{library.name}</h3>
                    {hideTypeSubtitle ? null : (
                      <span className="mt-1.5 inline-block rounded-full border border-(--plum-border) px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">
                        {libraryTypeLabel(library.type)}
                      </span>
                    )}
                  </div>
                  <Button
                    onClick={() => void saveLibraryPreferences(library)}
                    disabled={!isDirty || savingLibraryId === library.id}
                  >
                    {savingLibraryId === library.id ? "Saving…" : "Save defaults"}
                  </Button>
                </div>

                {supportsPlaybackPreferences ? (
                  <div className="mt-5 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                    <SettingsSelectRow
                      id={`library-audio-${library.id}`}
                      label="Preferred audio"
                      description={false}
                    >
                      <select
                        id={`library-audio-${library.id}`}
                        value={current.preferred_audio_language}
                        onChange={(event) =>
                          setLibraryField(
                            library.id,
                            "preferred_audio_language",
                            normalizeLanguagePreference(event.target.value),
                          )
                        }
                        className="flex h-9 w-full rounded-md border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
                      >
                        {languagePreferenceOptions.map((option) => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </select>
                    </SettingsSelectRow>

                    <SettingsSelectRow
                      id={`library-subtitles-${library.id}`}
                      label="Preferred subtitles"
                      description={false}
                    >
                      <select
                        id={`library-subtitles-${library.id}`}
                        value={current.preferred_subtitle_language}
                        onChange={(event) =>
                          setLibraryField(
                            library.id,
                            "preferred_subtitle_language",
                            normalizeLanguagePreference(event.target.value),
                          )
                        }
                        className="flex h-9 w-full rounded-md border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
                      >
                        {languagePreferenceOptions.map((option) => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </select>
                    </SettingsSelectRow>

                    <SettingsToggleRow
                      label="Enable subtitles by default"
                      checked={current.subtitles_enabled_by_default}
                      onChange={(checked) =>
                        setLibraryField(library.id, "subtitles_enabled_by_default", checked)
                      }
                      description="If the preferred subtitle language exists, Plum will enable it automatically."
                    />

                  </div>
                ) : (
                  <p className="mt-5 text-sm text-(--plum-muted)">
                    Music libraries skip playback language defaults, but still support automated scan
                    behavior below.
                  </p>
                )}

                <div className="mt-5 grid gap-4 md:grid-cols-3">
                  <SettingsToggleRow
                    label="Enable filesystem watcher"
                    checked={current.watcher_enabled}
                    onChange={(checked) => setLibraryField(library.id, "watcher_enabled", checked)}
                    description="Automatically queue a scan when Plum sees filesystem changes for this library."
                  />

                  <SettingsSelectRow
                    id={`library-watcher-mode-${library.id}`}
                    label="Watcher mode"
                    description="Auto prefers native filesystem events and falls back to polling when needed."
                  >
                    <select
                      id={`library-watcher-mode-${library.id}`}
                      value={current.watcher_mode}
                      disabled={!current.watcher_enabled}
                      onChange={(event) =>
                        setLibraryField(
                          library.id,
                          "watcher_mode",
                          event.target.value === "poll" ? "poll" : "auto",
                        )
                      }
                      className="flex h-9 w-full rounded-md border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) disabled:cursor-not-allowed disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
                    >
                      <option value="auto">Auto</option>
                      <option value="poll">Poll</option>
                    </select>
                  </SettingsSelectRow>

                  <SettingsInputRow
                    id={`library-scan-interval-${library.id}`}
                    label="Scheduled scan interval"
                    description={
                      <>
                        Enter minutes between automatic scans. Use <code>0</code> to disable scheduled
                        scans.
                      </>
                    }
                    inputProps={{
                      type: "number",
                      min: 0,
                      step: 1,
                      value: current.scan_interval_minutes,
                      onChange: (event) =>
                        setLibraryField(
                          library.id,
                          "scan_interval_minutes",
                          Math.max(0, Number.parseInt(event.target.value || "0", 10) || 0),
                        ),
                    }}
                  />
                </div>

                <p
                  className={`mt-4 text-sm ${
                    message?.includes("saved")
                      ? "text-emerald-300"
                      : message
                        ? "text-red-300"
                        : "text-(--plum-muted)"
                  }`}
                >
                  {message ??
                    (isDirty
                      ? "Unsaved changes."
                      : "Defaults are active for new playback sessions and future automation runs.")}
                </p>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}
