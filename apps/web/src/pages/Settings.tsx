import { useEffect, useMemo, useRef, useState } from "react";
import type {
  HardwareEncodeFormat,
  IntroSkipMode,
  Library,
  MetadataArtworkSettings as MetadataArtworkSettingsShape,
  MediaStackServiceValidationResult,
  MediaStackSettings as MediaStackSettingsShape,
  OpenCLToneMapAlgorithm,
  TranscodingSettings as TranscodingSettingsShape,
  MetadataArtworkProviderStatus,
  TranscodingSettingsWarning,
  VaapiDecodeCodec,
} from "@plum/contracts";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useAuthState } from "@/contexts/AuthContext";
import {
  languagePreferenceOptions,
  normalizeLanguagePreference,
  PLAYER_WEB_TRACK_LANGUAGE_NONE,
  readStoredPlayerWebDefaults,
  readStoredSubtitleAppearance,
  resolveLibraryPlaybackPreferences,
  subscribePlayerLocalSettings,
  subtitlePositionOptions,
  subtitleSizeOptions,
  writeStoredPlayerWebDefaults,
  writeStoredSubtitleAppearance,
} from "@/lib/playbackPreferences";
import {
  useMetadataArtworkSettings,
  useMediaStackSettings,
  useLibraries,
  useUpdateMediaStackSettings,
  useUpdateMetadataArtworkSettings,
  useTranscodingSettings,
  useValidateMediaStackSettings,
  useUpdateLibraryPlaybackPreferences,
  useUpdateTranscodingSettings,
} from "@/queries";
import { createQuickConnectCode } from "@/api";
import { cn } from "@/lib/utils";
import { Cpu, Image, Link2, ListTree, MonitorPlay, Server, Volume2 } from "lucide-react";
import { ServerEnvSettingsTab } from "@/pages/ServerEnvSettingsTab";

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

const openclTonemapAlgorithmOptions: Array<{
  value: OpenCLToneMapAlgorithm;
  label: string;
  description: string;
}> = [
  {
    value: "hable",
    label: "Hable",
    description: "Filmic curve; a common default for HDR fiction and games.",
  },
  {
    value: "reinhard",
    label: "Reinhard",
    description: "Smooth rolloff; can look softer on very bright highlights.",
  },
  {
    value: "mobius",
    label: "Mobius",
    description: "Preserves highlights with a gentle knee.",
  },
  {
    value: "linear",
    label: "Linear",
    description: "Simple linear stretch; can clip or look harsh on strong HDR.",
  },
  {
    value: "gamma",
    label: "Gamma",
    description: "Power-law compression; fast but less perceptually tuned.",
  },
  {
    value: "clip",
    label: "Clip",
    description: "Hard clip to SDR range; mostly useful as a baseline comparison.",
  },
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

function cloneMediaStackSettings(settings: MediaStackSettingsShape): MediaStackSettingsShape {
  return {
    radarr: { ...settings.radarr },
    sonarrTv: { ...settings.sonarrTv },
  };
}

const preferredMediaStackQualityProfiles: Record<keyof MediaStackSettingsShape, string> = {
  radarr: "UHD Bluray + Web",
  sonarrTv: "WEB-2160p",
};

function normalizeQualityProfileName(name: string): string {
  return name.trim().toLowerCase();
}

function resolvePreferredQualityProfileId(
  service: keyof MediaStackSettingsShape,
  validation: MediaStackServiceValidationResult,
): number {
  const preferredName = normalizeQualityProfileName(preferredMediaStackQualityProfiles[service]);
  return (
    validation.qualityProfiles.find(
      (profile) => normalizeQualityProfileName(profile.name) === preferredName,
    )?.id ?? 0
  );
}

function applyMediaStackValidationDefaults(
  settings: MediaStackSettingsShape,
  result: {
    radarr: MediaStackServiceValidationResult;
    sonarrTv: MediaStackServiceValidationResult;
  },
): { next: MediaStackSettingsShape; changed: boolean } {
  const next = cloneMediaStackSettings(settings);
  let changed = false;

  const applyDefaults = (
    service: keyof MediaStackSettingsShape,
    validation: MediaStackServiceValidationResult,
  ) => {
    if (!validation.reachable) return;
    const currentService = next[service];
    const nextRootFolder = validation.rootFolders.some(
      (folder) => folder.path === currentService.rootFolderPath,
    )
      ? currentService.rootFolderPath
      : (validation.rootFolders[0]?.path ?? "");
    if (currentService.rootFolderPath !== nextRootFolder) {
      currentService.rootFolderPath = nextRootFolder;
      changed = true;
    }

    const currentQualityProfileIsValid = validation.qualityProfiles.some(
      (profile) => profile.id === currentService.qualityProfileId,
    );
    const nextQualityProfileId = currentQualityProfileIsValid
      ? currentService.qualityProfileId
      : resolvePreferredQualityProfileId(service, validation) ||
        (validation.qualityProfiles[0]?.id ?? 0);
    if (currentService.qualityProfileId !== nextQualityProfileId) {
      currentService.qualityProfileId = nextQualityProfileId;
      changed = true;
    }
  };

  applyDefaults("radarr", result.radarr);
  applyDefaults("sonarrTv", result.sonarrTv);

  return { next, changed };
}

function summarizeMediaStackProfileListRefresh(result: {
  radarr: MediaStackServiceValidationResult;
  sonarrTv: MediaStackServiceValidationResult;
}): { message: string; tone: "success" | "warning" } {
  const statuses = [
    { label: "Radarr", validation: result.radarr },
    { label: "Sonarr TV", validation: result.sonarrTv },
  ];
  const reachable = statuses.filter((entry) => entry.validation.reachable);
  const unreachable = statuses.filter(
    (entry) => entry.validation.configured && !entry.validation.reachable,
  );

  if (reachable.length === 0) {
    return {
      message:
        "Could not load quality profiles. Check the connection details on the Media stack tab.",
      tone: "warning",
    };
  }
  if (unreachable.length > 0) {
    return {
      message: `Loaded profiles for ${reachable.map((e) => e.label).join(" and ")}. ${unreachable.map((e) => `${e.label} is unreachable`).join(" and ")}.`,
      tone: "warning",
    };
  }
  return {
    message: "Quality profile lists refreshed from Radarr and Sonarr TV.",
    tone: "success",
  };
}

function summarizeMediaStackValidation(result: {
  radarr: MediaStackServiceValidationResult;
  sonarrTv: MediaStackServiceValidationResult;
}): { message: string; tone: "success" | "warning" } {
  const statuses = [
    { label: "Radarr", validation: result.radarr },
    { label: "Sonarr TV", validation: result.sonarrTv },
  ];
  const reachable = statuses.filter((entry) => entry.validation.reachable);
  const unreachable = statuses.filter(
    (entry) => entry.validation.configured && !entry.validation.reachable,
  );

  if (reachable.length === 0) {
    return {
      message:
        "Unable to reach the configured media stack services. Check the connection details below.",
      tone: "warning",
    };
  }
  if (unreachable.length > 0) {
    return {
      message: `Defaults refreshed for ${reachable.map((entry) => entry.label).join(" and ")}. ${unreachable.map((entry) => `${entry.label} still needs attention`).join(" and ")}.`,
      tone: "warning",
    };
  }
  return {
    message: "Connection validated. Defaults refreshed from each reachable service.",
    tone: "success",
  };
}

type LibraryPlaybackPreferencesForm = {
  preferred_audio_language: string;
  preferred_subtitle_language: string;
  subtitles_enabled_by_default: boolean;
  intro_skip_mode: IntroSkipMode;
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
    intro_skip_mode: resolved.introSkipMode,
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
    left.intro_skip_mode === right.intro_skip_mode &&
    left.watcher_enabled === right.watcher_enabled &&
    left.watcher_mode === right.watcher_mode &&
    left.scan_interval_minutes === right.scan_interval_minutes
  );
}

function libraryTypeLabel(type: Library["type"]): string {
  switch (type) {
    case "movie":
      return "Movie";
    case "tv":
      return "TV";
    case "anime":
      return "Anime";
    case "music":
      return "Music";
    default:
      return type;
  }
}

/** True when the library name is just the type (e.g. "Movies" + movie) so we skip a duplicate subtitle. */
function libraryNameRedundantWithType(name: string, type: Library["type"]): boolean {
  const n = name.trim().toLowerCase();
  switch (type) {
    case "movie":
      return n === "movie" || n === "movies";
    case "tv":
      return n === "tv" || n === "tvs" || n === "television";
    case "anime":
      return n === "anime";
    case "music":
      return n === "music";
    default:
      return false;
  }
}

type SettingsTab =
  | "playback"
  | "server-env"
  | "media-stack"
  | "arr-profiles"
  | "metadata"
  | "transcoding";

function PlaybackWebDefaultsSection() {
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
    <article className="mt-8 rounded-md border border-(--plum-border) bg-(--plum-panel-alt)/60 p-4">
      <h3 className="text-base font-medium text-(--plum-text)">Web player on this device</h3>
      <p className="mt-1 max-w-2xl text-sm text-(--plum-muted)">
        Subtitle look, automatic track picks, and the subtitle track list apply only in this browser.
        Per-library defaults on this page still control server-side behavior unless you override them
        here.
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
  );
}

export function Settings() {
  const { user } = useAuthState();
  const isAdmin = user?.is_admin ?? false;
  const librariesQuery = useLibraries();
  const settingsQuery = useTranscodingSettings({ enabled: isAdmin });
  const metadataArtworkQuery = useMetadataArtworkSettings({ enabled: isAdmin });
  const mediaStackQuery = useMediaStackSettings({ enabled: isAdmin });
  const updateLibraryPreferences = useUpdateLibraryPlaybackPreferences();
  const updateMediaStack = useUpdateMediaStackSettings();
  const updateSettings = useUpdateTranscodingSettings();
  const updateMetadataArtwork = useUpdateMetadataArtworkSettings();
  const validateMediaStack = useValidateMediaStackSettings();
  const [form, setForm] = useState<TranscodingSettingsShape | null>(null);
  const [metadataArtworkForm, setMetadataArtworkForm] =
    useState<MetadataArtworkSettingsShape | null>(null);
  const [mediaStackForm, setMediaStackForm] = useState<MediaStackSettingsShape | null>(null);
  const [libraryForms, setLibraryForms] = useState<Record<number, LibraryPlaybackPreferencesForm>>(
    {},
  );
  const [librarySaveMessages, setLibrarySaveMessages] = useState<Record<number, string | null>>({});
  const [savingLibraryId, setSavingLibraryId] = useState<number | null>(null);
  const [warnings, setWarnings] = useState<TranscodingSettingsWarning[]>([]);
  const [saveMessage, setSaveMessage] = useState<string | null>(null);
  const [dirty, setDirty] = useState(false);
  const [mediaStackValidation, setMediaStackValidation] = useState<{
    radarr: MediaStackServiceValidationResult;
    sonarrTv: MediaStackServiceValidationResult;
  } | null>(null);
  const [mediaStackSaveMessage, setMediaStackSaveMessage] = useState<string | null>(null);
  const [mediaStackSaveTone, setMediaStackSaveTone] = useState<"success" | "warning" | "error">(
    "success",
  );
  const [mediaStackDirty, setMediaStackDirty] = useState(false);
  const [metadataArtworkSaveMessage, setMetadataArtworkSaveMessage] = useState<string | null>(null);
  const [metadataArtworkDirty, setMetadataArtworkDirty] = useState(false);
  const [activeTab, setActiveTab] = useState<SettingsTab>("playback");
  const [quickConnect, setQuickConnect] = useState<{ code: string; expiresAt: string } | null>(null);
  const [quickConnectBusy, setQuickConnectBusy] = useState(false);
  const [quickConnectErr, setQuickConnectErr] = useState<string | null>(null);
  const arrProfilesAutoRefreshPendingRef = useRef(false);

  useEffect(() => {
    if (activeTab === "arr-profiles") {
      arrProfilesAutoRefreshPendingRef.current = true;
    } else {
      arrProfilesAutoRefreshPendingRef.current = false;
    }
  }, [activeTab]);

  useEffect(() => {
    if (activeTab !== "arr-profiles" || !isAdmin) return;
    if (!arrProfilesAutoRefreshPendingRef.current) return;
    if (!mediaStackForm || validateMediaStack.isPending) return;
    arrProfilesAutoRefreshPendingRef.current = false;
    setMediaStackSaveMessage(null);
    setMediaStackSaveTone("success");
    void validateMediaStack
      .mutateAsync(mediaStackForm)
      .then((result) => {
        setMediaStackValidation(result);
        const feedback = summarizeMediaStackProfileListRefresh(result);
        setMediaStackSaveMessage(feedback.message);
        setMediaStackSaveTone(feedback.tone);
      })
      .catch((error) => {
        setMediaStackSaveMessage(
          error instanceof Error ? error.message : "Failed to validate media stack settings.",
        );
        setMediaStackSaveTone("error");
      });
  }, [activeTab, isAdmin, mediaStackForm, validateMediaStack]);

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
    if (!mediaStackQuery.data || mediaStackDirty) return;
    setMediaStackForm(cloneMediaStackSettings(mediaStackQuery.data));
  }, [mediaStackDirty, mediaStackQuery.data]);

  useEffect(() => {
    if (!isAdmin && activeTab !== "playback") {
      setActiveTab("playback");
    }
  }, [isAdmin, activeTab]);

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

  const libraries = librariesQuery.data ?? [];
  const getLibraryFormFallback = (libraryId: number) => {
    const library = librariesQuery.data?.find((item) => item.id === libraryId);
    return library
      ? cloneLibraryPlaybackPreferences(library)
      : {
          preferred_audio_language: "en",
          preferred_subtitle_language: "en",
          subtitles_enabled_by_default: true,
          intro_skip_mode: "manual",
          watcher_enabled: false,
          watcher_mode: "auto" as const,
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
      const updated = await updateLibraryPreferences.mutateAsync({
        libraryId: library.id,
        payload,
      });
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
        [library.id]: error instanceof Error ? error.message : "Failed to save playback defaults.",
      }));
    } finally {
      setSavingLibraryId(null);
    }
  };

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

  function setMediaStackServiceField<
    S extends keyof MediaStackSettingsShape,
    K extends keyof MediaStackSettingsShape[S],
  >(service: S, key: K, value: MediaStackSettingsShape[S][K]) {
    setMediaStackForm((current) =>
      current
        ? {
            ...current,
            [service]: {
              ...current[service],
              [key]: value,
            },
          }
        : current,
    );
    setMediaStackDirty(true);
    setMediaStackSaveMessage(null);
    setMediaStackSaveTone("success");
  }

  async function handleValidateMediaStack(options: { applyDefaults: boolean }) {
    if (!mediaStackForm) return;
    setMediaStackSaveMessage(null);
    setMediaStackSaveTone("success");
    try {
      const result = await validateMediaStack.mutateAsync(mediaStackForm);
      setMediaStackValidation(result);
      if (options.applyDefaults) {
        const { next, changed } = applyMediaStackValidationDefaults(mediaStackForm, result);
        const feedback = summarizeMediaStackValidation(result);
        setMediaStackForm(next);
        setMediaStackDirty((current) => current || changed);
        setMediaStackSaveMessage(feedback.message);
        setMediaStackSaveTone(feedback.tone);
      } else {
        const feedback = summarizeMediaStackProfileListRefresh(result);
        setMediaStackSaveMessage(feedback.message);
        setMediaStackSaveTone(feedback.tone);
      }
    } catch (error) {
      setMediaStackSaveMessage(
        error instanceof Error ? error.message : "Failed to validate media stack settings.",
      );
      setMediaStackSaveTone("error");
    }
  }

  async function handleSaveMediaStack() {
    if (!mediaStackForm) return;
    setMediaStackSaveMessage(null);
    setMediaStackSaveTone("success");
    try {
      const saved = await updateMediaStack.mutateAsync(mediaStackForm);
      setMediaStackForm(cloneMediaStackSettings(saved));
      setMediaStackDirty(false);
      setMediaStackSaveMessage("Media stack settings saved.");
      setMediaStackSaveTone("success");
    } catch (error) {
      setMediaStackSaveMessage(
        error instanceof Error ? error.message : "Failed to save media stack settings.",
      );
      setMediaStackSaveTone("error");
    }
  }

  const metadataArtworkAvailabilityByProvider = new Map<string, MetadataArtworkProviderStatus>(
    (metadataArtworkQuery.data?.provider_availability ?? []).map((provider) => [
      provider.provider,
      provider,
    ]),
  );

  // ── Tab: Playback ──────────────────────────────────────────────────────────

  const playbackTabContent = (
    <section className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4 shadow-[0_20px_45px_rgba(0,0,0,0.35)]">
      <div className="flex flex-col gap-2">
        <h2 className="text-xl font-semibold text-(--plum-text)">Playback defaults</h2>
        <p className="max-w-2xl text-sm text-(--plum-muted)">
          Choose the default playback behavior and scan automation for each library. Anime libraries
          default to Japanese audio with English subtitles; TV and movie libraries default to
          English for both when available.
        </p>
      </div>

      {librariesQuery.isLoading ? (
        <p className="mt-5 text-sm text-(--plum-muted)">Loading libraries…</p>
      ) : librariesQuery.isError ? (
        <p className="mt-5 text-sm text-red-300">
          {librariesQuery.error.message || "Failed to load libraries."}
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
                    <div>
                      <label
                        className="mb-2 block text-sm font-medium text-(--plum-text)"
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
                        className="flex h-9 w-full rounded-md border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
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
                        className="mb-2 block text-sm font-medium text-(--plum-text)"
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
                        className="flex h-9 w-full rounded-md border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
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

                    <div>
                      <label
                        className="mb-2 block text-sm font-medium text-(--plum-text)"
                        htmlFor={`library-intro-skip-${library.id}`}
                      >
                        Intro skip
                      </label>
                      <select
                        id={`library-intro-skip-${library.id}`}
                        value={current.intro_skip_mode}
                        onChange={(event) =>
                          setLibraryField(
                            library.id,
                            "intro_skip_mode",
                            event.target.value as IntroSkipMode,
                          )
                        }
                        className="flex h-9 w-full rounded-md border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
                      >
                        <option value="off">Off</option>
                        <option value="manual">Show skip button</option>
                        <option value="auto">Auto-skip</option>
                      </select>
                      <p className="mt-1.5 text-xs leading-snug text-(--plum-muted)">
                        When the video file has a chapter titled Intro, Opening, or similar, Plum can
                        jump past it. Rescan the library after adding chapters so timestamps are
                        picked up.
                      </p>
                    </div>
                  </div>
                ) : (
                  <p className="mt-5 text-sm text-(--plum-muted)">
                    Music libraries skip playback language defaults, but still support automated
                    scan behavior below.
                  </p>
                )}

                <div className="mt-5 grid gap-4 md:grid-cols-3">
                  <div className="flex items-end">
                    <Toggle
                      label="Enable filesystem watcher"
                      checked={current.watcher_enabled}
                      onChange={(checked) =>
                        setLibraryField(library.id, "watcher_enabled", checked)
                      }
                      description="Automatically queue a scan when Plum sees filesystem changes for this library."
                    />
                  </div>

                  <div>
                    <label
                      className="mb-2 block text-sm font-medium text-(--plum-text)"
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
                      className="flex h-9 w-full rounded-md border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) disabled:cursor-not-allowed disabled:opacity-60 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
                    >
                      <option value="auto">Auto</option>
                      <option value="poll">Poll</option>
                    </select>
                    <p className="mt-2 text-xs text-(--plum-muted)">
                      Auto prefers native filesystem events and falls back to polling when needed.
                    </p>
                  </div>

                  <div>
                    <label
                      className="mb-2 block text-sm font-medium text-(--plum-text)"
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
                    <p className="mt-2 text-xs text-(--plum-muted)">
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
      <PlaybackWebDefaultsSection />
    </section>
  );

  // ── Tab: Media Stack ───────────────────────────────────────────────────────

  const mediaStackTabContent = (
    <section className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4 shadow-[0_20px_45px_rgba(0,0,0,0.35)]">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-xl font-semibold text-(--plum-text)">Media stack</h2>
          <p className="mt-1 max-w-2xl text-sm text-(--plum-muted)">
            Connect Radarr and Sonarr TV so Discover can add titles directly and Downloads can show
            live queue progress.
          </p>
        </div>
        <div className="flex flex-wrap gap-3">
          <Button
            variant="outline"
            onClick={() => void handleValidateMediaStack({ applyDefaults: true })}
            disabled={mediaStackForm == null || validateMediaStack.isPending}
          >
            {validateMediaStack.isPending ? "Validating..." : "Validate & load defaults"}
          </Button>
          <Button
            onClick={handleSaveMediaStack}
            disabled={mediaStackForm == null || !mediaStackDirty || updateMediaStack.isPending}
          >
            {updateMediaStack.isPending ? "Saving..." : "Save settings"}
          </Button>
        </div>
      </div>

      {mediaStackQuery.isLoading || mediaStackForm == null ? (
        <p className="mt-5 text-sm text-(--plum-muted)">Loading media stack settings...</p>
      ) : mediaStackQuery.isError ? (
        <p className="mt-5 text-sm text-red-300">
          {mediaStackQuery.error.message || "Failed to load media stack settings."}
        </p>
      ) : (
        <div className="mt-6 grid gap-4 xl:grid-cols-2">
          <MediaStackServiceCard
            title="Radarr"
            description="Movie adds always route here in v1."
            service={mediaStackForm.radarr}
            validation={mediaStackValidation?.radarr ?? null}
            onChange={(key, value) => setMediaStackServiceField("radarr", key, value)}
          />
          <MediaStackServiceCard
            title="Sonarr TV"
            description="TV show adds always route here in v1."
            service={mediaStackForm.sonarrTv}
            validation={mediaStackValidation?.sonarrTv ?? null}
            onChange={(key, value) => setMediaStackServiceField("sonarrTv", key, value)}
          />
        </div>
      )}

      <p
        className={`mt-4 text-sm ${
          mediaStackSaveMessage == null
            ? "text-(--plum-muted)"
            : mediaStackSaveTone === "success"
              ? "text-emerald-300"
              : mediaStackSaveTone === "warning"
                ? "text-amber-200"
                : mediaStackSaveMessage
                  ? "text-red-300"
                  : "text-(--plum-muted)"
        }`}
      >
        {mediaStackSaveMessage ??
          (mediaStackDirty
            ? "Unsaved changes."
            : "Direct adds always search immediately after Plum hands the title to Radarr or Sonarr TV.")}
      </p>
    </section>
  );

  const arrProfilesTabContent = (
    <section className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4 shadow-[0_20px_45px_rgba(0,0,0,0.35)]">
      <div className="flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
        <div>
          <h2 className="text-xl font-semibold text-(--plum-text)">Sonarr / Radarr profiles</h2>
          <p className="mt-1 max-w-2xl text-sm text-(--plum-muted)">
            Pick the default quality profiles used when Discover adds a movie to Radarr or a series to
            Sonarr TV. Set the base URL and API key on the{" "}
            <span className="text-(--plum-text-secondary)">Media stack</span> tab first. Lists load
            automatically when you open this tab; use Refresh if you changed Arr outside Plum.{" "}
            <span className="text-(--plum-muted)">
              Refreshing only updates the dropdowns and does not replace your saved choices.
            </span>
          </p>
        </div>
        <div className="flex flex-wrap gap-3">
          <Button
            variant="outline"
            onClick={() => void handleValidateMediaStack({ applyDefaults: false })}
            disabled={mediaStackForm == null || validateMediaStack.isPending}
          >
            {validateMediaStack.isPending ? "Refreshing…" : "Refresh profile lists"}
          </Button>
          <Button
            onClick={() => void handleSaveMediaStack()}
            disabled={mediaStackForm == null || !mediaStackDirty || updateMediaStack.isPending}
          >
            {updateMediaStack.isPending ? "Saving…" : "Save profile defaults"}
          </Button>
        </div>
      </div>

      {mediaStackQuery.isLoading || mediaStackForm == null ? (
        <p className="mt-5 text-sm text-(--plum-muted)">Loading media stack settings...</p>
      ) : mediaStackQuery.isError ? (
        <p className="mt-5 text-sm text-red-300">
          {mediaStackQuery.error.message || "Failed to load media stack settings."}
        </p>
      ) : (
        <div className="mt-6 grid gap-4 xl:grid-cols-2">
          <MediaStackArrQualityProfileCard
            title="Radarr (movies)"
            description="Default quality profile for new movie adds."
            service={mediaStackForm.radarr}
            validation={mediaStackValidation?.radarr ?? null}
            idPrefix="arr-profiles-radarr"
            qualityLabel="Default movie quality profile (Radarr)"
            onChange={(profileId) => setMediaStackServiceField("radarr", "qualityProfileId", profileId)}
          />
          <MediaStackArrQualityProfileCard
            title="Sonarr TV (shows)"
            description="Default quality profile for new series adds."
            service={mediaStackForm.sonarrTv}
            validation={mediaStackValidation?.sonarrTv ?? null}
            idPrefix="arr-profiles-sonarr"
            qualityLabel="Default TV quality profile (Sonarr)"
            onChange={(profileId) =>
              setMediaStackServiceField("sonarrTv", "qualityProfileId", profileId)
            }
          />
        </div>
      )}

      <p
        className={`mt-4 text-sm ${
          mediaStackSaveMessage == null
            ? "text-(--plum-muted)"
            : mediaStackSaveTone === "success"
              ? "text-emerald-300"
              : mediaStackSaveTone === "warning"
                ? "text-amber-200"
                : mediaStackSaveMessage
                  ? "text-red-300"
                  : "text-(--plum-muted)"
        }`}
      >
        {mediaStackSaveMessage ??
          (mediaStackDirty
            ? "Unsaved changes."
            : "Saved defaults are used the next time a title is sent to Radarr or Sonarr TV.")}
      </p>
    </section>
  );

  // ── Tab: Metadata ──────────────────────────────────────────────────────────

  const metadataTabContent = (() => {
    if (metadataArtworkQuery.isLoading || metadataArtworkForm == null) {
      return (
        <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4">
          <p className="text-sm text-(--plum-muted)">Loading metadata artwork settings…</p>
        </div>
      );
    }
    if (metadataArtworkQuery.isError) {
      return (
        <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4">
          <p className="text-sm text-red-300">
            {metadataArtworkQuery.error.message || "Failed to load metadata artwork settings."}
          </p>
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-6">
        <div className="flex flex-col gap-3 rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4 shadow-[0_20px_45px_rgba(0,0,0,0.35)] md:flex-row md:items-end md:justify-between">
          <div>
            <h2 className="text-xl font-semibold text-(--plum-text)">Metadata artwork</h2>
            <p className="mt-1 max-w-2xl text-sm text-(--plum-muted)">
              Control which image fetchers Plum uses for movies, shows, seasons, and episodes.
              Provider order is fixed; these toggles only enable or disable each step.
            </p>
          </div>
          <Button onClick={handleSaveMetadataArtwork} disabled={updateMetadataArtwork.isPending}>
            {updateMetadataArtwork.isPending ? "Saving…" : "Save settings"}
          </Button>
        </div>

        <div className="grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(18rem,1fr)]">
          <div className="flex flex-col gap-6">
            <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4">
              <h3 className="text-base font-medium text-(--plum-text)">Movies</h3>
              <p className="mt-1 text-sm text-(--plum-muted)">
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

            <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4">
              <h3 className="text-base font-medium text-(--plum-text)">Shows</h3>
              <p className="mt-1 text-sm text-(--plum-muted)">
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

            <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4">
              <h3 className="text-base font-medium text-(--plum-text)">Seasons</h3>
              <p className="mt-1 text-sm text-(--plum-muted)">
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

            <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-4">
              <h3 className="text-base font-medium text-(--plum-text)">Episodes</h3>
              <p className="mt-1 text-sm text-(--plum-muted)">
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
            <div className="rounded-lg border border-(--plum-border) bg-(--plum-panel)/80 p-5">
              <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">
                Provider status
              </h3>
              <ul className="mt-3 space-y-3">
                {(metadataArtworkQuery.data?.provider_availability ?? []).map((provider) => (
                  <li
                    key={provider.provider}
                    className={`rounded-md border p-3 text-sm ${
                      provider.available
                        ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-100"
                        : "border-(--plum-border) bg-(--plum-panel-alt)/60 text-(--plum-muted)"
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

            <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-5">
              <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">
                Save status
              </h3>
              <p
                className={`mt-3 text-sm ${
                  metadataArtworkSaveMessage?.includes("saved")
                    ? "text-emerald-300"
                    : metadataArtworkSaveMessage
                      ? "text-red-300"
                      : "text-(--plum-muted)"
                }`}
              >
                {metadataArtworkSaveMessage ??
                  (metadataArtworkDirty
                    ? "Unsaved changes."
                    : "Saved settings are active for future metadata refreshes.")}
              </p>
            </div>
          </aside>
        </div>
      </div>
    );
  })();

  // ── Tab: Transcoding ───────────────────────────────────────────────────────

  const transcodingTabContent = (() => {
    if (settingsQuery.isLoading || form == null) {
      return (
        <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-4">
          <p className="text-sm text-(--plum-muted)">Loading transcoding settings…</p>
        </div>
      );
    }
    if (settingsQuery.isError) {
      return (
        <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-4">
          <p className="text-sm text-red-300">
            {settingsQuery.error.message || "Failed to load transcoding settings."}
          </p>
        </div>
      );
    }
    return (
      <div className="flex flex-col gap-6">
        <div className="flex flex-col gap-2 rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-4 shadow-[0_20px_45px_rgba(0,0,0,0.35)] md:flex-row md:items-end md:justify-between">
          <div>
            <h2 className="text-xl font-semibold text-(--plum-text)">Transcoding</h2>
            <p className="mt-1 max-w-2xl text-sm text-(--plum-muted)">
              Configure server-wide VAAPI decode and hardware encode behavior for future transcode
              jobs.
            </p>
          </div>
          <Button onClick={handleSave} disabled={updateSettings.isPending}>
            {updateSettings.isPending ? "Saving…" : "Save settings"}
          </Button>
        </div>

        <div className="grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(18rem,1fr)]">
          <div className="flex flex-col gap-6">
            <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-4">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <h3 className="text-base font-medium text-(--plum-text)">
                    Video Acceleration API
                  </h3>
                  <p className="mt-1 text-sm text-(--plum-muted)">
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
                    className="mb-2 block text-sm font-medium text-(--plum-text)"
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
                  <p className="mt-2 text-xs text-(--plum-muted)">
                    Default render node for Intel/AMD VAAPI on Linux hosts.
                  </p>
                </div>

                <div>
                  <div className="mb-3">
                    <h4 className="text-sm font-medium text-(--plum-text)">Decode codecs</h4>
                    <p className="mt-1 text-xs text-(--plum-muted)">
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

            <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-4">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <h3 className="text-base font-medium text-(--plum-text)">Hardware encoding</h3>
                  <p className="mt-1 text-sm text-(--plum-muted)">
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
                    <h4 className="text-sm font-medium text-(--plum-text)">
                      Allowed output formats
                    </h4>
                    <p className="mt-1 text-xs text-(--plum-muted)">
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
                    className="mb-2 block text-sm font-medium text-(--plum-text)"
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
                    className="flex h-9 w-full rounded-(--radius-md) border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
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
                  <p className="mt-2 text-xs text-(--plum-muted)">
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

            <div className="rounded-(--radius-lg) border border-amber-500/25 bg-amber-500/[0.06] p-4">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <h3 className="text-base font-medium text-(--plum-text)">
                    Experimental: OpenCL tone mapping
                  </h3>
                  <p className="mt-1 text-sm text-(--plum-muted)">
                    When enabled, Plum may insert FFmpeg{" "}
                    <code className="rounded bg-black/30 px-1 py-0.5 text-xs">tonemap_opencl</code> for
                    sources that look HDR (PQ / HLG transfer, or 10-bit BT.2020). Requires OpenCL-capable
                    drivers and a matching FFmpeg build. Does not apply when burning in PGS subtitles.
                  </p>
                </div>
                <Toggle
                  label="Enable OpenCL tone map"
                  checked={form.openclToneMappingEnabled}
                  onChange={(checked) => setField("openclToneMappingEnabled", checked)}
                />
              </div>

              <div
                className={`mt-5 space-y-5 ${form.openclToneMappingEnabled ? "" : "pointer-events-none opacity-50"}`}
              >
                <div>
                  <label
                    className="mb-2 block text-sm font-medium text-(--plum-text)"
                    htmlFor="opencl-tonemap-algorithm"
                  >
                    Tonemap curve
                  </label>
                  <select
                    id="opencl-tonemap-algorithm"
                    value={form.openclToneMapAlgorithm}
                    onChange={(event) =>
                      setField("openclToneMapAlgorithm", event.target.value as OpenCLToneMapAlgorithm)
                    }
                    className="flex h-9 w-full rounded-(--radius-md) border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
                  >
                    {openclTonemapAlgorithmOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                  <p className="mt-2 text-xs text-(--plum-muted)">
                    {
                      openclTonemapAlgorithmOptions.find((o) => o.value === form.openclToneMapAlgorithm)
                        ?.description
                    }
                  </p>
                </div>

                <div>
                  <label
                    className="mb-2 block text-sm font-medium text-(--plum-text)"
                    htmlFor="opencl-tonemap-desat"
                  >
                    Highlight desaturation
                  </label>
                  <Input
                    id="opencl-tonemap-desat"
                    type="number"
                    min={0}
                    max={4}
                    step={0.05}
                    value={form.openclToneMapDesat}
                    onChange={(event) => {
                      const n = Number.parseFloat(event.target.value);
                      if (Number.isFinite(n)) {
                        setField("openclToneMapDesat", n);
                      }
                    }}
                  />
                  <p className="mt-2 text-xs text-(--plum-muted)">
                    Passed to FFmpeg as{" "}
                    <code className="rounded bg-black/30 px-1 py-0.5 text-xs">desat</code> (0–4). Try
                    around 0.5 unless you want a more saturated HDR look.
                  </p>
                </div>
              </div>
            </div>
          </div>

          <aside className="flex flex-col gap-4">
            <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-5">
              <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">
                Host warnings
              </h3>
              {warnings.length === 0 ? (
                <p className="mt-3 text-sm text-(--plum-muted)">
                  No capability warnings reported for the current server configuration.
                </p>
              ) : (
                <ul className="mt-3 space-y-3">
                  {warnings.map((warning) => (
                    <li
                      key={warning.code}
                      className="rounded-(--radius-md) border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-100"
                    >
                      {warning.message}
                    </li>
                  ))}
                </ul>
              )}
            </div>

            <div className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel)/80 p-5">
              <h3 className="text-sm font-semibold uppercase tracking-[0.18em] text-(--plum-muted)">
                Save status
              </h3>
              <p
                className={`mt-3 text-sm ${
                  saveMessage?.includes("saved")
                    ? "text-emerald-300"
                    : saveMessage
                      ? "text-red-300"
                      : "text-(--plum-muted)"
                }`}
              >
                {saveMessage ??
                  (dirty ? "Unsaved changes." : "Saved settings are active for future jobs.")}
              </p>
            </div>
          </aside>
        </div>
      </div>
    );
  })();

  // ── Tab definitions ────────────────────────────────────────────────────────

  const navItemBase =
    "relative flex items-center gap-2 px-2.5 py-1.5 text-sm font-medium rounded-md transition-all cursor-pointer select-none w-full";
  const navItemActive =
    "text-(--plum-text) bg-[rgba(181,123,255,0.1)] before:absolute before:left-0 before:top-1/2 before:-translate-y-1/2 before:h-5 before:w-[3px] before:rounded-r-full before:bg-(--plum-accent) before:content-[''] shadow-[0_0_20px_rgba(139,92,246,0.08)]";
  const navItemInactive =
    "text-(--plum-muted) hover:text-(--plum-text) hover:bg-[rgba(181,123,255,0.06)]";

  type TabItem = { id: SettingsTab; label: string; icon: React.ReactNode };
  const adminTabSections: { heading: string; tabs: TabItem[] }[] = [
    {
      heading: "Host",
      tabs: [{ id: "server-env", label: "Environment", icon: <Server className="size-4 shrink-0" /> }],
    },
    {
      heading: "Integrations",
      tabs: [
        { id: "media-stack", label: "Media stack", icon: <Link2 className="size-4 shrink-0" /> },
        { id: "arr-profiles", label: "Arr profiles", icon: <ListTree className="size-4 shrink-0" /> },
      ],
    },
    {
      heading: "Processing",
      tabs: [
        { id: "metadata", label: "Metadata", icon: <Image className="size-4 shrink-0" /> },
        { id: "transcoding", label: "Transcoding", icon: <Cpu className="size-4 shrink-0" /> },
      ],
    },
  ];

  const tabContent: Record<SettingsTab, React.ReactNode> = {
    playback: playbackTabContent,
    "server-env": <ServerEnvSettingsTab />,
    "media-stack": mediaStackTabContent,
    "arr-profiles": arrProfilesTabContent,
    metadata: metadataTabContent,
    transcoding: transcodingTabContent,
  };

  const generateQuickConnect = async () => {
    setQuickConnectBusy(true);
    setQuickConnectErr(null);
    try {
      const res = await createQuickConnectCode();
      setQuickConnect({ code: res.code, expiresAt: res.expiresAt });
    } catch (e) {
      setQuickConnectErr(e instanceof Error ? e.message : "Could not generate code.");
    } finally {
      setQuickConnectBusy(false);
    }
  };

  // ── Render ─────────────────────────────────────────────────────────────────

  return (
    <div className="mx-auto max-w-6xl">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:gap-5">
        <nav
          className="w-full shrink-0 rounded-lg border border-(--plum-border) bg-(--plum-panel)/85 p-2 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)] lg:sticky lg:top-0 lg:w-52 lg:self-start"
          aria-label="Settings sections"
        >
          <p className="mb-2 px-2 text-[10px] font-semibold uppercase tracking-[0.2em] text-(--plum-muted)">
            Library
          </p>
          <div className="flex flex-col gap-0.5">
            <button
              type="button"
              onClick={() => setActiveTab("playback")}
              className={cn(
                navItemBase,
                activeTab === "playback" ? navItemActive : navItemInactive,
              )}
              aria-current={activeTab === "playback" ? "page" : undefined}
            >
              <Volume2 className="size-4 shrink-0" />
              <span className="truncate">Playback</span>
            </button>
          </div>
          {isAdmin ? <div className="my-3 border-t border-(--plum-border)" /> : null}
          {isAdmin
            ? adminTabSections.map((section, idx) => (
                <div key={section.heading} className={cn(idx === 0 ? "" : "mt-4")}>
                  <p className="mb-2 px-2 text-[10px] font-semibold uppercase tracking-[0.2em] text-(--plum-muted)">
                    {section.heading}
                  </p>
                  <div className="flex flex-col gap-0.5">
                    {section.tabs.map((tab) => (
                      <button
                        key={tab.id}
                        type="button"
                        onClick={() => setActiveTab(tab.id)}
                        className={cn(
                          navItemBase,
                          activeTab === tab.id ? navItemActive : navItemInactive,
                        )}
                        aria-current={activeTab === tab.id ? "page" : undefined}
                      >
                        {tab.icon}
                        <span className="truncate">{tab.label}</span>
                      </button>
                    ))}
                  </div>
                </div>
              ))
            : null}

          {user ? (
            <div className="mt-4 border-t border-(--plum-border) pt-3">
              <h2 className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-(--plum-muted)">
                <MonitorPlay className="size-3.5 shrink-0" aria-hidden />
                Quick connect
              </h2>
              <p className="mt-1.5 text-xs leading-snug text-(--plum-muted)">
                TV sign-in for <span className="text-(--plum-text-secondary)">{user.email}</span>. On the TV,
                use this server&apos;s URL and &quot;Sign in with TV code&quot;. Code expires in 15 minutes,
                one use.
              </p>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                className="mt-2 w-full"
                disabled={quickConnectBusy}
                onClick={() => void generateQuickConnect()}
              >
                {quickConnectBusy ? "Generating…" : quickConnect ? "New code" : "Generate code"}
              </Button>
              {quickConnectErr ? (
                <p className="mt-2 text-xs text-red-400">{quickConnectErr}</p>
              ) : null}
              {quickConnect ? (
                <div className="mt-2 rounded-md border border-(--plum-border) bg-(--plum-panel) p-2">
                  <p className="text-[10px] font-medium uppercase tracking-wider text-(--plum-muted)">Code</p>
                  <p className="mt-0.5 font-mono text-xl font-semibold tracking-[0.25em] text-(--plum-text)">
                    {quickConnect.code}
                  </p>
                  <p className="mt-1 text-[11px] text-(--plum-muted)">
                    Expires {new Date(quickConnect.expiresAt).toLocaleString()} · once
                  </p>
                </div>
              ) : null}
            </div>
          ) : null}
        </nav>

        <div className="min-w-0 flex-1 pb-2">{tabContent[activeTab]}</div>
      </div>
    </div>
  );
}

function MediaStackArrQualityProfileCard({
  title,
  description,
  service,
  validation,
  qualityLabel,
  idPrefix,
  onChange,
}: {
  title: string;
  description: string;
  service: MediaStackSettingsShape["radarr"];
  validation: MediaStackServiceValidationResult | null;
  qualityLabel: string;
  idPrefix: string;
  onChange: (qualityProfileId: number) => void;
}) {
  const reachable = validation?.reachable === true;

  return (
    <article className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel-alt)/60 p-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h3 className="text-base font-medium text-(--plum-text)">{title}</h3>
          <p className="mt-1 text-sm text-(--plum-muted)">{description}</p>
        </div>
        <span
          className={`rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.16em] ${
            reachable
              ? "bg-emerald-500/12 text-emerald-200"
              : validation?.configured
                ? "bg-amber-500/12 text-amber-200"
                : "bg-(--plum-panel) text-(--plum-muted)"
          }`}
        >
          {reachable ? "Connected" : validation?.configured ? "Needs attention" : "Not configured"}
        </span>
      </div>

      <div className="mt-5">
        <label
          htmlFor={`${idPrefix}-quality-profile`}
          className="mb-2 block text-sm font-medium text-(--plum-text)"
        >
          {qualityLabel}
        </label>
        <select
          id={`${idPrefix}-quality-profile`}
          value={service.qualityProfileId}
          onChange={(event) =>
            onChange(Number.parseInt(event.target.value, 10) || 0)
          }
          className="flex h-9 w-full rounded-(--radius-md) border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
        >
          <option value={0}>Select a quality profile</option>
          {(validation?.qualityProfiles ?? []).map((profile) => (
            <option key={profile.id} value={profile.id}>
              {profile.name}
            </option>
          ))}
        </select>
      </div>

      <p
        className={`mt-4 text-sm ${
          validation?.errorMessage ? "text-amber-200" : "text-(--plum-muted)"
        }`}
      >
        {validation?.errorMessage ??
          "Profile names load when you open this tab (after URL and API key are set on Media stack). Use Refresh profile lists to update."}
      </p>
    </article>
  );
}

function MediaStackServiceCard({
  title,
  description,
  service,
  validation,
  onChange,
}: {
  title: string;
  description: string;
  service: MediaStackSettingsShape["radarr"];
  validation: MediaStackServiceValidationResult | null;
  onChange: <K extends keyof MediaStackSettingsShape["radarr"]>(
    key: K,
    value: MediaStackSettingsShape["radarr"][K],
  ) => void;
}) {
  const reachable = validation?.reachable === true;
  const serviceId = title.toLowerCase().replace(/[^a-z0-9]+/g, "-");

  return (
    <article className="rounded-(--radius-lg) border border-(--plum-border) bg-(--plum-panel-alt)/60 p-5">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h3 className="text-base font-medium text-(--plum-text)">{title}</h3>
          <p className="mt-1 text-sm text-(--plum-muted)">{description}</p>
        </div>
        <span
          className={`rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.16em] ${
            reachable
              ? "bg-emerald-500/12 text-emerald-200"
              : validation?.configured
                ? "bg-amber-500/12 text-amber-200"
                : "bg-(--plum-panel) text-(--plum-muted)"
          }`}
        >
          {reachable ? "Connected" : validation?.configured ? "Needs attention" : "Not configured"}
        </span>
      </div>

      <div className="mt-5 grid gap-4">
        <div>
          <label
            htmlFor={`${serviceId}-base-url`}
            className="mb-2 block text-sm font-medium text-(--plum-text)"
            title="Root URL you use in the browser for this app (include http/https, no trailing path)."
          >
            Base URL
          </label>
          <Input
            id={`${serviceId}-base-url`}
            value={service.baseUrl}
            onChange={(event) => onChange("baseUrl", event.target.value)}
            placeholder="http://127.0.0.1:7878"
          />
        </div>

        <div>
          <label
            htmlFor={`${serviceId}-api-key`}
            className="mb-2 block text-sm font-medium text-(--plum-text)"
            title="From Settings → General → Security → API key in Radarr or Sonarr."
          >
            API key
          </label>
          <Input
            id={`${serviceId}-api-key`}
            type="password"
            value={service.apiKey}
            onChange={(event) => onChange("apiKey", event.target.value)}
            placeholder="Paste the service API key"
          />
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <div>
            <label
              htmlFor={`${serviceId}-root-folder`}
              className="mb-2 block text-sm font-medium text-(--plum-text)"
              title="Library path on the Arr host where new downloads are imported."
            >
              Root folder
            </label>
            <select
              id={`${serviceId}-root-folder`}
              value={service.rootFolderPath}
              onChange={(event) => onChange("rootFolderPath", event.target.value)}
              className="flex h-9 w-full rounded-(--radius-md) border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
            >
              <option value="">Select a root folder</option>
              {(validation?.rootFolders ?? []).map((folder) => (
                <option key={folder.path} value={folder.path}>
                  {folder.path}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label
              htmlFor={`${serviceId}-quality-profile`}
              className="mb-2 block text-sm font-medium text-(--plum-text)"
              title="Default profile when Discover adds a title. Refresh lists after validating the connection."
            >
              Quality profile
            </label>
            <select
              id={`${serviceId}-quality-profile`}
              value={service.qualityProfileId}
              onChange={(event) =>
                onChange("qualityProfileId", Number.parseInt(event.target.value, 10) || 0)
              }
              className="flex h-9 w-full rounded-(--radius-md) border border-(--plum-border) bg-(--plum-panel) px-3 py-1 text-sm text-(--plum-text) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-(--plum-ring) focus-visible:ring-offset-2 focus-visible:ring-offset-(--plum-bg)"
            >
              <option value={0}>Select a quality profile</option>
              {(validation?.qualityProfiles ?? []).map((profile) => (
                <option key={profile.id} value={profile.id}>
                  {profile.name}
                </option>
              ))}
            </select>
          </div>
        </div>
      </div>

      <p
        className={`mt-4 text-sm ${
          validation?.errorMessage ? "text-amber-200" : "text-(--plum-muted)"
        }`}
      >
        {validation?.errorMessage ??
          "Use Validate & load defaults to pull the live root folders and quality profiles from this service."}
      </p>
    </article>
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
    <label className="inline-flex cursor-pointer items-start gap-3 text-sm text-(--plum-text)">
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="sr-only"
      />
      <span
        aria-hidden
        className={`relative mt-0.5 inline-flex h-5 w-9 shrink-0 rounded-full border-2 border-transparent transition-colors duration-200 ${
          checked ? "bg-(--plum-accent)" : "bg-(--plum-panel-alt)"
        }`}
      >
        <span
          className={`inline-block size-4 rounded-full bg-white shadow-sm transition-transform duration-200 ${
            checked ? "translate-x-4" : "translate-x-0"
          }`}
        />
      </span>
      <span className="flex flex-col">
        <span>{label}</span>
        {description ? (
          <span className="text-xs text-(--plum-muted)">{description}</span>
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
      className={cn(
        "flex gap-3 rounded-(--radius-md) border p-3 transition-colors",
        disabled
          ? "cursor-not-allowed border-(--plum-border) bg-(--plum-panel-alt)/60 opacity-70"
          : checked
            ? "cursor-pointer border-(--plum-accent-soft) bg-(--plum-accent-subtle)"
            : "cursor-pointer border-(--plum-border) bg-(--plum-panel-alt)/60 hover:border-(--plum-accent-soft)",
      )}
    >
      <input
        type="checkbox"
        checked={checked}
        aria-label={label}
        disabled={disabled}
        onChange={(event) => onChange(event.target.checked)}
        className="mt-1 size-4 shrink-0 rounded border-(--plum-border) bg-(--plum-panel-alt) accent-(--plum-accent)"
      />
      <span className="flex min-w-0 flex-col">
        <span className="text-sm font-medium text-(--plum-text)">{label}</span>
        <span className="text-xs text-(--plum-muted)">{description}</span>
      </span>
    </label>
  );
}
