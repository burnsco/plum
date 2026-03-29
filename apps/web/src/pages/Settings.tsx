import { useEffect, useState } from "react";
import type {
  HardwareEncodeFormat,
  Library,
  MetadataArtworkSettings as MetadataArtworkSettingsShape,
  TranscodingSettings as TranscodingSettingsShape,
  MetadataArtworkProviderStatus,
  TranscodingSettingsWarning,
  VaapiDecodeCodec,
} from "@plum/contracts";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useAuthState } from "@/contexts/AuthContext";
import {
  playerControlsAppearanceOptions,
  languagePreferenceOptions,
  normalizeLanguagePreference,
  readStoredPlayerControlsAppearance,
  resolveLibraryPlaybackPreferences,
  subscribeToPlayerControlsAppearance,
  writeStoredPlayerControlsAppearance,
  type PlayerControlsAppearance,
} from "@/lib/playbackPreferences";
import {
  useMetadataArtworkSettings,
  useLibraries,
  useUpdateMetadataArtworkSettings,
  useTranscodingSettings,
  useUpdateLibraryPlaybackPreferences,
  useUpdateTranscodingSettings,
} from "@/queries";

const decodeCodecOptions: Array<{
  key: VaapiDecodeCodec;
  label: string;
  description: string;
}> = [
  { key: "h264", label: "H.264", description: "Use VAAPI decode for 8-bit AVC video." },
  { key: "hevc", label: "HEVC", description: "Use VAAPI decode for standard HEVC streams." },
  { key: "mpeg2", label: "MPEG-2", description: "Use VAAPI decode for legacy MPEG-2 sources." },
  { key: "vc1", label: "VC-1", description: "Use VAAPI decode for VC-1 content when available." },
  { key: "vp8", label: "VP8", description: "Use VAAPI decode for VP8 sources." },
  { key: "vp9", label: "VP9", description: "Use VAAPI decode for standard VP9 streams." },
  { key: "av1", label: "AV1", description: "Use VAAPI decode for AV1 content." },
  {
    key: "hevc10bit",
    label: "HEVC 10-bit",
    description: "Allow VAAPI decode for 10-bit HEVC video.",
  },
  { key: "vp910bit", label: "VP9 10-bit", description: "Allow VAAPI decode for 10-bit VP9 video." },
];

const encodeFormatOptions: Array<{
  key: HardwareEncodeFormat;
  label: string;
  description: string;
}> = [
  { key: "h264", label: "H.264", description: "Best playback compatibility and safest default." },
  {
    key: "hevc",
    label: "HEVC",
    description: "Smaller output with newer client support requirements.",
  },
  {
    key: "av1",
    label: "AV1",
    description: "Highest efficiency, but hardware support varies widely.",
  },
];

const movieArtworkProviderOptions: Array<{
  key: keyof MetadataArtworkSettingsShape["movies"];
  label: string;
  description: string;
}> = [
  { key: "fanart", label: "Fanart", description: "Use fanart.tv artwork when it is available." },
  { key: "tmdb", label: "TMDB", description: "Use TMDB posters for movies and series." },
  { key: "tvdb", label: "TVDB", description: "Use TVDB posters for movies and series." },
];

const showArtworkProviderOptions: Array<{
  key: keyof MetadataArtworkSettingsShape["shows"];
  label: string;
  description: string;
}> = [
  { key: "fanart", label: "Fanart", description: "Use fanart.tv artwork when it is available." },
  { key: "tmdb", label: "TMDB", description: "Use TMDB posters for shows and seasons." },
  { key: "tvdb", label: "TVDB", description: "Use TVDB posters for shows and seasons." },
];

const episodeArtworkProviderOptions: Array<{
  key: keyof MetadataArtworkSettingsShape["episodes"];
  label: string;
  description: string;
}> = [
  { key: "tmdb", label: "TMDB", description: "Use TMDB stills and episode posters first." },
  { key: "tvdb", label: "TVDB", description: "Use TVDB episode artwork when available." },
  { key: "omdb", label: "OMDb", description: "Use OMDb when an episode IMDb ID is known." },
];

const settingsPanelClass =
  "rounded-[var(--radius-xl)] border border-[var(--plum-border)] bg-[var(--plum-panel)]/92 p-6 shadow-[0_18px_40px_rgba(3,8,20,0.14)]";
const settingsInsetClass =
  "rounded-[var(--radius-lg)] border border-[var(--plum-border)] bg-[var(--plum-panel-alt)]/65 p-5";
const settingsSelectClass =
  "flex h-10 w-full rounded-[var(--radius-md)] border border-[var(--plum-border)] bg-[var(--plum-panel-alt)]/88 px-3 py-2 text-sm text-[var(--plum-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--plum-ring)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--plum-bg)]";

function cloneSettings(settings: TranscodingSettingsShape): TranscodingSettingsShape {
  return {
    ...settings,
    decodeCodecs: { ...settings.decodeCodecs },
    encodeFormats: { ...settings.encodeFormats },
  };
}

function cloneMetadataArtworkSettings(
  settings: MetadataArtworkSettingsShape,
): MetadataArtworkSettingsShape {
  return {
    movies: { ...settings.movies },
    shows: { ...settings.shows },
    seasons: { ...settings.seasons },
    episodes: { ...settings.episodes },
  };
}

type LibraryPlaybackPreferencesForm = {
  preferred_audio_language: string;
  preferred_subtitle_language: string;
  subtitles_enabled_by_default: boolean;
  watcher_enabled: boolean;
  watcher_mode: "auto" | "poll";
  scan_interval_minutes: number;
};

function cloneLibraryPlaybackPreferences(library: Library): LibraryPlaybackPreferencesForm {
  const resolved = resolveLibraryPlaybackPreferences(library);
  return {
    preferred_audio_language: normalizeLanguagePreference(resolved.preferredAudioLanguage),
    preferred_subtitle_language: normalizeLanguagePreference(resolved.preferredSubtitleLanguage),
    subtitles_enabled_by_default: resolved.subtitlesEnabledByDefault,
    watcher_enabled: library.watcher_enabled ?? false,
    watcher_mode: library.watcher_mode === "poll" ? "poll" : "auto",
    scan_interval_minutes: library.scan_interval_minutes ?? 0,
  };
}

function libraryPreferencesEqual(
  left: LibraryPlaybackPreferencesForm,
  right: LibraryPlaybackPreferencesForm,
): boolean {
  return (
    left.preferred_audio_language === right.preferred_audio_language &&
    left.preferred_subtitle_language === right.preferred_subtitle_language &&
    left.subtitles_enabled_by_default === right.subtitles_enabled_by_default &&
    left.watcher_enabled === right.watcher_enabled &&
    left.watcher_mode === right.watcher_mode &&
    left.scan_interval_minutes === right.scan_interval_minutes
  );
}

export function Settings() {
  const { user } = useAuthState();
  const isAdmin = user?.is_admin ?? false;
  const librariesQuery = useLibraries();
  const settingsQuery = useTranscodingSettings({ enabled: isAdmin });
  const metadataArtworkQuery = useMetadataArtworkSettings({ enabled: isAdmin });
  const updateLibraryPreferences = useUpdateLibraryPlaybackPreferences();
  const updateSettings = useUpdateTranscodingSettings();
  const updateMetadataArtwork = useUpdateMetadataArtworkSettings();
  const [form, setForm] = useState<TranscodingSettingsShape | null>(null);
  const [metadataArtworkForm, setMetadataArtworkForm] =
    useState<MetadataArtworkSettingsShape | null>(null);
  const [libraryForms, setLibraryForms] = useState<Record<number, LibraryPlaybackPreferencesForm>>({});
  const [playerControlsAppearance, setPlayerControlsAppearance] = useState<PlayerControlsAppearance>(
    () => readStoredPlayerControlsAppearance(),
  );
  const [librarySaveMessages, setLibrarySaveMessages] = useState<Record<number, string | null>>({});
  const [savingLibraryId, setSavingLibraryId] = useState<number | null>(null);
  const [warnings, setWarnings] = useState<TranscodingSettingsWarning[]>([]);
  const [saveMessage, setSaveMessage] = useState<string | null>(null);
  const [dirty, setDirty] = useState(false);
  const [metadataArtworkSaveMessage, setMetadataArtworkSaveMessage] = useState<string | null>(null);
  const [metadataArtworkDirty, setMetadataArtworkDirty] = useState(false);

  useEffect(() => {
    if (!settingsQuery.data || dirty) return;
    setForm(cloneSettings(settingsQuery.data.settings));
    setWarnings(settingsQuery.data.warnings);
  }, [dirty, settingsQuery.data]);

  useEffect(() => {
    if (!metadataArtworkQuery.data || metadataArtworkDirty) return;
    setMetadataArtworkForm(cloneMetadataArtworkSettings(metadataArtworkQuery.data.settings));
  }, [metadataArtworkDirty, metadataArtworkQuery.data]);

  useEffect(() => {
    if (!librariesQuery.data) return;
    setLibraryForms((current) => {
      const next = { ...current };
      for (const library of librariesQuery.data) {
        const fallback = cloneLibraryPlaybackPreferences(library);
        const existing = current[library.id];
        const currentLibrary = cloneLibraryPlaybackPreferences(library);
        next[library.id] =
          existing && !libraryPreferencesEqual(existing, currentLibrary) ? existing : fallback;
      }
      return next;
    });
  }, [librariesQuery.data]);

  useEffect(() => {
    writeStoredPlayerControlsAppearance(playerControlsAppearance);
  }, [playerControlsAppearance]);

  useEffect(
    () =>
      subscribeToPlayerControlsAppearance((preference) => {
        setPlayerControlsAppearance((current) =>
          current === preference ? current : preference,
        );
      }),
    [],
  );

  const libraries = librariesQuery.data ?? [];
  const getLibraryFormFallback = (libraryId: number) => {
    const library = librariesQuery.data?.find((item) => item.id === libraryId);
    return library
      ? cloneLibraryPlaybackPreferences(library)
      : {
          preferred_audio_language: "en",
          preferred_subtitle_language: "en",
          subtitles_enabled_by_default: true,
          watcher_enabled: false,
          watcher_mode: "auto",
          scan_interval_minutes: 0,
        };
  };

  const setLibraryField = <K extends keyof LibraryPlaybackPreferencesForm>(
    libraryId: number,
    key: K,
    value: LibraryPlaybackPreferencesForm[K],
  ) => {
    setLibraryForms((current) => {
      const base = current[libraryId] ?? getLibraryFormFallback(libraryId);
      return {
        ...current,
        [libraryId]: { ...base, [key]: value },
      };
    });
    setLibrarySaveMessages((current) => ({ ...current, [libraryId]: null }));
  };

  const saveLibraryPreferences = async (library: Library) => {
    const payload = libraryForms[library.id] ?? cloneLibraryPlaybackPreferences(library);
    setSavingLibraryId(library.id);
    setLibrarySaveMessages((current) => ({ ...current, [library.id]: null }));
    try {
      const updated = await updateLibraryPreferences.mutateAsync({ libraryId: library.id, payload });
      setLibraryForms((current) => ({
        ...current,
        [library.id]: cloneLibraryPlaybackPreferences(updated),
      }));
      setLibrarySaveMessages((current) => ({
        ...current,
        [library.id]: "Playback defaults saved.",
      }));
    } catch (error) {
      setLibrarySaveMessages((current) => ({
        ...current,
        [library.id]:
          error instanceof Error ? error.message : "Failed to save playback defaults.",
      }));
    } finally {
      setSavingLibraryId(null);
    }
  };

  const playbackDefaultsSection = (
    <section className={settingsPanelClass}>
      <div className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold text-[var(--plum-text)]">Playback defaults</h1>
        <p className="max-w-2xl text-sm text-[var(--plum-muted)]">
          Choose the default playback behavior and scan automation for each library. Anime
          libraries default to Japanese audio with English subtitles; TV and movie libraries default
          to English for both when available.
        </p>
      </div>

      <div className={settingsInsetClass}>
        <div className="flex flex-col gap-2">
          <h2 className="text-lg font-medium text-[var(--plum-text)]">Player controls look</h2>
          <p className="text-sm text-[var(--plum-muted)]">
            Pick the default video-player controls style for docked and fullscreen playback. The
            quick switch inside the player updates this same preference.
          </p>
        </div>

        <div className="mt-4 grid gap-3 md:grid-cols-2">
          {playerControlsAppearanceOptions.map((option) => {
            const isActive = playerControlsAppearance === option.value;
            return (
              <button
                key={option.value}
                type="button"
                className={`rounded-[var(--radius-md)] border p-4 text-left transition-colors ${
                  isActive
                    ? "border-[var(--plum-accent-soft)] bg-[color-mix(in_srgb,var(--plum-accent)_18%,transparent)] text-[var(--plum-text)]"
                    : "border-[var(--plum-border)] bg-[var(--plum-panel)]/70 text-[var(--plum-text)] hover:border-[var(--plum-accent-soft)]"
                }`}
                onClick={() => setPlayerControlsAppearance(option.value)}
                aria-pressed={isActive}
              >
                <div className="flex items-center justify-between gap-3">
                  <span className="text-sm font-medium">{option.label}</span>
                  <span className="text-xs uppercase tracking-[0.16em] text-[var(--plum-muted)]">
                    {isActive ? "Active" : "Available"}
                  </span>
                </div>
                <p className="mt-2 text-sm text-[var(--plum-muted)]">{option.description}</p>
              </button>
            );
          })}
        </div>
      </div>

      {librariesQuery.isLoading ? (
        <p className="mt-5 text-sm text-[var(--plum-muted)]">Loading libraries…</p>
      ) : librariesQuery.isError ? (
        <p className="mt-5 text-sm text-red-300">
          {librariesQuery.error.message || "Failed to load libraries."}
        </p>
      ) : libraries.length === 0 ? (
        <p className="mt-5 text-sm text-[var(--plum-muted)]">
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

            return (
              <article
                key={library.id}
                className="rounded-[var(--radius-md)] border border-[var(--plum-border)] bg-[var(--plum-panel-alt)]/60 p-5"
              >
                <div className="flex flex-col gap-2 md:flex-row md:items-start md:justify-between">
                  <div>
                    <h2 className="text-lg font-medium text-[var(--plum-text)]">{library.name}</h2>
                    <p className="mt-1 text-xs uppercase tracking-[0.16em] text-[var(--plum-muted)]">
                      {library.type}
                    </p>
                  </div>
                  <Button
                    onClick={() => void saveLibraryPreferences(library)}
                    disabled={!isDirty || savingLibraryId === library.id}
                  >
                    {savingLibraryId === library.id ? "Saving…" : "Save defaults"}
                  </Button>
                </div>

                {supportsPlaybackPreferences ? (
                  <div className="mt-5 grid gap-4 md:grid-cols-3">
                    <div>
                      <label
                        className="mb-2 block text-sm font-medium text-[var(--plum-text)]"
                        htmlFor={`library-audio-${library.id}`}
                      >
                        Preferred audio
                      </label>
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
                        className={settingsSelectClass}
                      >
                        {languagePreferenceOptions.map((option) => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </select>
                    </div>

                    <div>
                      <label
                        className="mb-2 block text-sm font-medium text-[var(--plum-text)]"
                        htmlFor={`library-subtitles-${library.id}`}
                      >
                        Preferred subtitles
                      </label>
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
                        className={settingsSelectClass}
                      >
                        {languagePreferenceOptions.map((option) => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </select>
                    </div>

                    <div className="flex items-end">
                      <Toggle
                        label="Enable subtitles by default"
                        checked={current.subtitles_enabled_by_default}
                        onChange={(checked) =>
                          setLibraryField(library.id, "subtitles_enabled_by_default", checked)
                        }
                        description="If the preferred subtitle language exists, Plum will enable it automatically."
                      />
                    </div>
                  </div>
                ) : (
                  <p className="mt-5 text-sm text-[var(--plum-muted)]">
                    Music libraries skip playback language defaults, but still support automated
                    scan behavior below.
                  </p>
                )}

                <div className="mt-5 grid gap-4 md:grid-cols-3">
                  <div className="flex items-end">
                    <Toggle
                      label="Enable filesystem watcher"
                      checked={current.watcher_enabled}
                      onChange={(checked) => setLibraryField(library.id, "watcher_enabled", checked)}
                      description="Automatically queue a scan when Plum sees filesystem changes for this library."
                    />
                  </div>

                  <div>
                    <label
                      className="mb-2 block text-sm font-medium text-[var(--plum-text)]"
                      htmlFor={`library-watcher-mode-${library.id}`}
                    >
                      Watcher mode
                    </label>
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
                      className={`${settingsSelectClass} disabled:cursor-not-allowed disabled:opacity-60`}
                    >
                      <option value="auto">Auto</option>
                      <option value="poll">Poll</option>
                    </select>
                    <p className="mt-2 text-xs text-[var(--plum-muted)]">
                      Auto prefers native filesystem events and falls back to polling when needed.
                    </p>
                  </div>

                  <div>
                    <label
                      className="mb-2 block text-sm font-medium text-[var(--plum-text)]"
                      htmlFor={`library-scan-interval-${library.id}`}
                    >
                      Scheduled scan interval
                    </label>
                    <Input
                      id={`library-scan-interval-${library.id}`}
                      type="number"
                      min={0}
                      step={1}
                      value={current.scan_interval_minutes}
                      onChange={(event) =>
                        setLibraryField(
                          library.id,
                          "scan_interval_minutes",
                          Math.max(0, Number.parseInt(event.target.value || "0", 10) || 0),
                        )
                      }
                    />
                    <p className="mt-2 text-xs text-[var(--plum-muted)]">
                      Enter minutes between automatic scans. Use <code>0</code> to disable scheduled
                      scans.
                    </p>
                  </div>
                </div>

                <p
                  className={`mt-4 text-sm ${
                    message?.includes("saved")
                      ? "text-emerald-300"
                      : message
                        ? "text-red-300"
                        : isDirty
                          ? "text-[var(--plum-muted)]"
                          : "text-[var(--plum-muted)]"
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

  if (!isAdmin) {
    return (
      <div className="mx-auto flex max-w-6xl flex-col gap-6">
        {playbackDefaultsSection}
        <div className={settingsPanelClass}>
          <h2 className="text-xl font-semibold text-[var(--plum-text)]">Server settings</h2>
          <p className="mt-2 text-sm text-[var(--plum-muted)]">
            Server transcoding and metadata artwork settings are only available to admin accounts.
          </p>
        </div>
      </div>
    );
  }

  if (settingsQuery.isError || metadataArtworkQuery.isError) {
    return (
      <div className="mx-auto flex max-w-6xl flex-col gap-6">
        {playbackDefaultsSection}
        <div className={settingsPanelClass}>
          <h2 className="text-xl font-semibold text-[var(--plum-text)]">Server settings</h2>
          <p className="mt-2 text-sm text-red-300">
            {settingsQuery.error?.message ||
              metadataArtworkQuery.error?.message ||
              "Failed to load server settings."}
          </p>
        </div>
      </div>
    );
  }

  if (
    settingsQuery.isLoading ||
    metadataArtworkQuery.isLoading ||
    form == null ||
    metadataArtworkForm == null
  ) {
    return (
      <div className="mx-auto flex max-w-6xl flex-col gap-6">
        {playbackDefaultsSection}
        <div className={settingsPanelClass}>
          <h2 className="text-xl font-semibold text-[var(--plum-text)]">Server settings</h2>
          <p className="mt-2 text-sm text-[var(--plum-muted)]">Loading server settings…</p>
        </div>
      </div>
    );
  }

  function setField<K extends keyof TranscodingSettingsShape>(
    key: K,
    value: TranscodingSettingsShape[K],
  ) {
    setForm((current) => (current ? { ...current, [key]: value } : current));
    setDirty(true);
    setSaveMessage(null);
  }

  const setDecodeCodec = (key: VaapiDecodeCodec, checked: boolean) => {
    setForm((current) =>
      current
        ? {
            ...current,
            decodeCodecs: { ...current.decodeCodecs, [key]: checked },
          }
        : current,
    );
    setDirty(true);
    setSaveMessage(null);
  };

  const setEncodeFormat = (key: HardwareEncodeFormat, checked: boolean) => {
    setForm((current) => {
      if (!current) return current;
      const next = {
        ...current,
        encodeFormats: { ...current.encodeFormats, [key]: checked },
      };
      if (!next.encodeFormats[next.preferredHardwareEncodeFormat]) {
        const fallback =
          encodeFormatOptions.find((option) => next.encodeFormats[option.key])?.key ?? "h264";
        next.preferredHardwareEncodeFormat = fallback;
      }
      return next;
    });
    setDirty(true);
    setSaveMessage(null);
  };

  const handleSave = async () => {
    if (!form) return;
    setSaveMessage(null);
    try {
      const response = await updateSettings.mutateAsync(form);
      setForm(cloneSettings(response.settings));
      setWarnings(response.warnings);
      setDirty(false);
      setSaveMessage("Transcoding settings saved.");
    } catch (error) {
      setSaveMessage(
        error instanceof Error ? error.message : "Failed to save transcoding settings.",
      );
    }
  };

  function setArtworkField(
    section: keyof Pick<MetadataArtworkSettingsShape, "movies" | "shows" | "seasons">,
    key: keyof MetadataArtworkSettingsShape["shows"],
    checked: boolean,
  ) {
    setMetadataArtworkForm((current) =>
      current
        ? {
            ...current,
            [section]: { ...current[section], [key]: checked },
          }
        : current,
    );
    setMetadataArtworkDirty(true);
    setMetadataArtworkSaveMessage(null);
  }

  function setEpisodeArtworkField(
    key: keyof MetadataArtworkSettingsShape["episodes"],
    checked: boolean,
  ) {
    setMetadataArtworkForm((current) =>
      current
        ? {
            ...current,
            episodes: { ...current.episodes, [key]: checked },
          }
        : current,
    );
    setMetadataArtworkDirty(true);
    setMetadataArtworkSaveMessage(null);
  }

  const handleSaveMetadataArtwork = async () => {
    if (!metadataArtworkForm) return;
    setMetadataArtworkSaveMessage(null);
    try {
      const response = await updateMetadataArtwork.mutateAsync(metadataArtworkForm);
      setMetadataArtworkForm(cloneMetadataArtworkSettings(response.settings));
      setMetadataArtworkDirty(false);
      setMetadataArtworkSaveMessage("Metadata artwork settings saved.");
    } catch (error) {
      setMetadataArtworkSaveMessage(
        error instanceof Error ? error.message : "Failed to save metadata artwork settings.",
      );
    }
  };

  const metadataArtworkAvailabilityByProvider = new Map<string, MetadataArtworkProviderStatus>(
    (metadataArtworkQuery.data?.provider_availability ?? []).map((provider) => [
      provider.provider,
      provider,
    ]),
  );

  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-6">
      {playbackDefaultsSection}

      <section className={settingsPanelClass}>
        <div className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-[var(--plum-text)]">Metadata artwork</h1>
            <p className="mt-1 max-w-2xl text-sm text-[var(--plum-muted)]">
              Control which image fetchers Plum uses for movies, shows, seasons, and episodes.
              Provider order is fixed; these toggles only enable or disable each step.
            </p>
          </div>
          <Button onClick={handleSaveMetadataArtwork} disabled={updateMetadataArtwork.isPending}>
            {updateMetadataArtwork.isPending ? "Saving…" : "Save settings"}
          </Button>
        </div>
      </section>

      <section className="grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(18rem,1fr)]">
        <div className="flex flex-col gap-6">
          <div className={settingsPanelClass}>
            <h2 className="text-lg font-medium text-[var(--plum-text)]">Movies</h2>
            <p className="mt-1 text-sm text-[var(--plum-muted)]">
              Automatic order: Fanart, then TMDB, then TVDB.
            </p>
            <div className="mt-5 grid gap-3 md:grid-cols-2">
              {movieArtworkProviderOptions.map((option) => {
                const availability = metadataArtworkAvailabilityByProvider.get(option.key);
                return (
                  <CheckboxCard
                    key={`movies-${option.key}`}
                    checked={metadataArtworkForm.movies[option.key]}
                    label={option.label}
                    description={
                      availability && !availability.available && availability.reason
                        ? `${option.description} ${availability.reason}.`
                        : option.description
                    }
                    disabled={availability?.available === false}
                    onChange={(checked) => setArtworkField("movies", option.key, checked)}
                  />
                );
              })}
            </div>
          </div>

          <div className={settingsPanelClass}>
            <h2 className="text-lg font-medium text-[var(--plum-text)]">Shows</h2>
            <p className="mt-1 text-sm text-[var(--plum-muted)]">
              Automatic order: Fanart, then TMDB, then TVDB.
            </p>
            <div className="mt-5 grid gap-3 md:grid-cols-2">
              {showArtworkProviderOptions.map((option) => {
                const availability = metadataArtworkAvailabilityByProvider.get(option.key);
                return (
                  <CheckboxCard
                    key={`shows-${option.key}`}
                    checked={metadataArtworkForm.shows[option.key]}
                    label={option.label}
                    description={
                      availability && !availability.available && availability.reason
                        ? `${option.description} ${availability.reason}.`
                        : option.description
                    }
                    disabled={availability?.available === false}
                    onChange={(checked) => setArtworkField("shows", option.key, checked)}
                  />
                );
              })}
            </div>
          </div>

          <div className={settingsPanelClass}>
            <h2 className="text-lg font-medium text-[var(--plum-text)]">Seasons</h2>
            <p className="mt-1 text-sm text-[var(--plum-muted)]">
              Automatic order: Fanart, then TMDB, then TVDB.
            </p>
            <div className="mt-5 grid gap-3 md:grid-cols-2">
              {showArtworkProviderOptions.map((option) => {
                const availability = metadataArtworkAvailabilityByProvider.get(option.key);
                return (
                  <CheckboxCard
                    key={`seasons-${option.key}`}
                    checked={metadataArtworkForm.seasons[option.key]}
                    label={option.label}
                    description={
                      availability && !availability.available && availability.reason
                        ? `${option.description} ${availability.reason}.`
                        : option.description
                    }
                    disabled={availability?.available === false}
                    onChange={(checked) => setArtworkField("seasons", option.key, checked)}
                  />
                );
              })}
            </div>
          </div>

          <div className={settingsPanelClass}>
            <h2 className="text-lg font-medium text-[var(--plum-text)]">Episodes</h2>
            <p className="mt-1 text-sm text-[var(--plum-muted)]">
              Automatic order: TMDB, TVDB, then OMDb.
            </p>
            <div className="mt-5 grid gap-3 md:grid-cols-2">
              {episodeArtworkProviderOptions.map((option) => {
                const availability = metadataArtworkAvailabilityByProvider.get(option.key);
                return (
                  <CheckboxCard
                    key={`episodes-${option.key}`}
                    checked={metadataArtworkForm.episodes[option.key]}
                    label={option.label}
                    description={
                      availability && !availability.available && availability.reason
                        ? `${option.description} ${availability.reason}.`
                        : option.description
                    }
                    disabled={availability?.available === false}
                    onChange={(checked) => setEpisodeArtworkField(option.key, checked)}
                  />
                );
              })}
            </div>
          </div>
        </div>

        <aside className="flex flex-col gap-4">
          <div className={settingsPanelClass}>
            <h2 className="text-sm font-semibold uppercase tracking-[0.18em] text-[var(--plum-muted)]">
              Provider status
            </h2>
            <ul className="mt-3 space-y-3">
              {(metadataArtworkQuery.data?.provider_availability ?? []).map((provider) => (
                <li
                  key={provider.provider}
                  className={`rounded-[var(--radius-md)] border p-3 text-sm ${
                    provider.available
                      ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-100"
                      : "border-[var(--plum-border)] bg-[var(--plum-panel-alt)]/60 text-[var(--plum-muted)]"
                  }`}
                >
                  <div className="font-medium uppercase tracking-[0.12em]">{provider.provider}</div>
                  <div className="mt-1">
                    {provider.available ? "Available" : provider.reason || "Unavailable"}
                  </div>
                </li>
              ))}
            </ul>
          </div>

          <div className={settingsPanelClass}>
            <h2 className="text-sm font-semibold uppercase tracking-[0.18em] text-[var(--plum-muted)]">
              Save status
            </h2>
            <p
              className={`mt-3 text-sm ${
                metadataArtworkSaveMessage?.includes("saved")
                  ? "text-emerald-300"
                  : metadataArtworkSaveMessage
                    ? "text-red-300"
                    : "text-[var(--plum-muted)]"
              }`}
            >
              {metadataArtworkSaveMessage ??
                (metadataArtworkDirty
                  ? "Unsaved changes."
                  : "Saved settings are active for future metadata refreshes.")}
            </p>
          </div>
        </aside>
      </section>

      <section className={settingsPanelClass}>
        <div className="flex flex-col gap-2 md:flex-row md:items-end md:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-[var(--plum-text)]">Transcoding</h1>
            <p className="mt-1 max-w-2xl text-sm text-[var(--plum-muted)]">
              Configure server-wide VAAPI decode and hardware encode behavior for future transcode
              jobs.
            </p>
          </div>
          <Button onClick={handleSave} disabled={updateSettings.isPending}>
            {updateSettings.isPending ? "Saving…" : "Save settings"}
          </Button>
        </div>
      </section>

      <section className="grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(18rem,1fr)]">
        <div className="flex flex-col gap-6">
          <div className={settingsPanelClass}>
            <div className="flex items-start justify-between gap-4">
              <div>
                <h2 className="text-lg font-medium text-[var(--plum-text)]">
                  Video Acceleration API
                </h2>
                <p className="mt-1 text-sm text-[var(--plum-muted)]">
                  Enable VAAPI on the server and choose which source codecs are allowed to use it
                  for decode.
                </p>
              </div>
              <Toggle
                label="Enable VAAPI"
                checked={form.vaapiEnabled}
                onChange={(checked) => setField("vaapiEnabled", checked)}
              />
            </div>

            <div className="mt-5 space-y-5">
              <div>
                <label
                  className="mb-2 block text-sm font-medium text-[var(--plum-text)]"
                  htmlFor="vaapi-device"
                >
                  VAAPI device
                </label>
                <Input
                  id="vaapi-device"
                  value={form.vaapiDevicePath}
                  onChange={(event) => setField("vaapiDevicePath", event.target.value)}
                  placeholder="/dev/dri/renderD128"
                />
                <p className="mt-2 text-xs text-[var(--plum-muted)]">
                  Default render node for Intel/AMD VAAPI on Linux hosts.
                </p>
              </div>

              <div>
                <div className="mb-3">
                  <h3 className="text-sm font-medium text-[var(--plum-text)]">Decode codecs</h3>
                  <p className="mt-1 text-xs text-[var(--plum-muted)]">
                    Each codec can be enabled or disabled independently. Disabled codecs stay on
                    software decode.
                  </p>
                </div>
                <div className="grid gap-3 md:grid-cols-2">
                  {decodeCodecOptions.map((option) => (
                    <CheckboxCard
                      key={option.key}
                      checked={form.decodeCodecs[option.key]}
                      label={option.label}
                      description={option.description}
                      onChange={(checked) => setDecodeCodec(option.key, checked)}
                    />
                  ))}
                </div>
              </div>
            </div>
          </div>

          <div className={settingsPanelClass}>
            <div className="flex items-start justify-between gap-4">
              <div>
                <h2 className="text-lg font-medium text-[var(--plum-text)]">Hardware encoding</h2>
                <p className="mt-1 text-sm text-[var(--plum-muted)]">
                  Use VAAPI encoders when possible, with automatic software fallback if the hardware
                  path fails.
                </p>
              </div>
              <Toggle
                label="Enable hardware encoding"
                checked={form.hardwareEncodingEnabled}
                onChange={(checked) => setField("hardwareEncodingEnabled", checked)}
              />
            </div>

            <div className="mt-5 space-y-5">
              <div>
                <div className="mb-3">
                  <h3 className="text-sm font-medium text-[var(--plum-text)]">
                    Allowed output formats
                  </h3>
                  <p className="mt-1 text-xs text-[var(--plum-muted)]">
                    H.264 is enabled by default. HEVC and AV1 stay opt-in for compatibility and host
                    support reasons.
                  </p>
                </div>
                <div className="grid gap-3 md:grid-cols-3">
                  {encodeFormatOptions.map((option) => (
                    <CheckboxCard
                      key={option.key}
                      checked={form.encodeFormats[option.key]}
                      label={option.label}
                      description={option.description}
                      onChange={(checked) => setEncodeFormat(option.key, checked)}
                    />
                  ))}
                </div>
              </div>

              <div>
                <label
                  className="mb-2 block text-sm font-medium text-[var(--plum-text)]"
                  htmlFor="preferred-encode-format"
                >
                  Preferred hardware encode format
                </label>
                <select
                  id="preferred-encode-format"
                  value={form.preferredHardwareEncodeFormat}
                  onChange={(event) =>
                    setField(
                      "preferredHardwareEncodeFormat",
                      event.target.value as HardwareEncodeFormat,
                    )
                  }
                  className={settingsSelectClass}
                >
                  {encodeFormatOptions.map((option) => (
                    <option
                      key={option.key}
                      value={option.key}
                      disabled={!form.encodeFormats[option.key]}
                    >
                      {option.label}
                    </option>
                  ))}
                </select>
                <p className="mt-2 text-xs text-[var(--plum-muted)]">
                  Plum will try this hardware output format first, then retry in software if enabled
                  below.
                </p>
              </div>

              <Toggle
                label="Allow automatic software fallback"
                checked={form.allowSoftwareFallback}
                onChange={(checked) => setField("allowSoftwareFallback", checked)}
                description="When hardware transcoding fails, retry with software-safe FFmpeg settings."
              />
            </div>
          </div>
        </div>

        <aside className="flex flex-col gap-4">
          <div className={settingsPanelClass}>
            <h2 className="text-sm font-semibold uppercase tracking-[0.18em] text-[var(--plum-muted)]">
              Host warnings
            </h2>
            {warnings.length === 0 ? (
              <p className="mt-3 text-sm text-[var(--plum-muted)]">
                No capability warnings reported for the current server configuration.
              </p>
            ) : (
              <ul className="mt-3 space-y-3">
                {warnings.map((warning) => (
                  <li
                    key={warning.code}
                    className="rounded-[var(--radius-md)] border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-100"
                  >
                    {warning.message}
                  </li>
                ))}
              </ul>
            )}
          </div>

          <div className={settingsPanelClass}>
            <h2 className="text-sm font-semibold uppercase tracking-[0.18em] text-[var(--plum-muted)]">
              Save status
            </h2>
            <p
              className={`mt-3 text-sm ${
                saveMessage?.includes("saved")
                  ? "text-emerald-300"
                  : saveMessage
                    ? "text-red-300"
                    : "text-[var(--plum-muted)]"
              }`}
            >
              {saveMessage ??
                (dirty ? "Unsaved changes." : "Saved settings are active for future jobs.")}
            </p>
          </div>
        </aside>
      </section>
    </div>
  );
}

function Toggle({
  label,
  checked,
  onChange,
  description,
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  description?: string;
}) {
  return (
    <label className="inline-flex cursor-pointer items-start gap-3 text-sm text-[var(--plum-text)]">
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="mt-0.5 size-4 rounded border-[var(--plum-border)] bg-[var(--plum-panel-alt)] accent-[var(--plum-accent)]"
      />
      <span className="flex flex-col">
        <span>{label}</span>
        {description ? (
          <span className="text-xs text-[var(--plum-muted)]">{description}</span>
        ) : null}
      </span>
    </label>
  );
}

function CheckboxCard({
  checked,
  label,
  description,
  disabled,
  onChange,
}: {
  checked: boolean;
  label: string;
  description: string;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label
      className={`flex gap-3 rounded-[var(--radius-md)] border border-[var(--plum-border)] bg-[var(--plum-panel-alt)]/60 p-3 transition-colors ${
        disabled
          ? "cursor-not-allowed opacity-70"
          : "cursor-pointer hover:border-[var(--plum-accent-soft)]"
      }`}
    >
      <input
        type="checkbox"
        checked={checked}
        aria-label={label}
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
        className="mt-1 size-4 rounded border-[var(--plum-border)] bg-[var(--plum-panel-alt)] accent-[var(--plum-accent)]"
      />
      <span className="flex min-w-0 flex-col">
        <span className="text-sm font-medium text-[var(--plum-text)]">{label}</span>
        <span className="text-xs text-[var(--plum-muted)]">{description}</span>
      </span>
    </label>
  );
}
