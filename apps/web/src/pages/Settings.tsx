import { useEffect, useRef, useState, type ReactNode } from "react";
import type {
  HardwareEncodeFormat,
  Library,
  MetadataArtworkSettings as MetadataArtworkSettingsShape,
  MediaStackServiceValidationResult,
  MediaStackSettings as MediaStackSettingsShape,
  TranscodingSettings as TranscodingSettingsShape,
  MetadataArtworkProviderStatus,
  TranscodingSettingsWarning,
  VaapiDecodeCodec,
} from "@plum/contracts";
import { useAuthState } from "@/contexts/AuthContext";
import { normalizeLanguagePreference, resolveLibraryPlaybackPreferences } from "@/lib/playbackPreferences";
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
import {
  Captions,
  CircleUser,
  Cpu,
  Image,
  Link2,
  ListTree,
  Server,
  Shield,
  SkipForward,
  Volume2,
} from "lucide-react";
import { AdminSettingsTab } from "@/pages/AdminSettingsTab";
import { ServerEnvSettingsTab } from "@/pages/ServerEnvSettingsTab";
import { IntroSkipperPluginTab } from "@/pages/IntroSkipperPluginTab";
import { SettingsGeneralTab } from "@/pages/settings/SettingsGeneralTab";
import { SettingsArrProfilesTab, SettingsMediaStackTab } from "@/pages/settings/SettingsMediaStackTab";
import { SettingsMetadataTab } from "@/pages/settings/SettingsMetadataTab";
import { SettingsPlaybackTab } from "@/pages/settings/SettingsPlaybackTab";
import { SettingsSubtitlesTab } from "@/pages/settings/SettingsSubtitlesTab";
import { SettingsTranscodingTab } from "@/pages/settings/SettingsTranscodingTab";
import { encodeFormatOptions } from "@/pages/settings/settingsOptions";
import {
  libraryPreferencesEqual,
  type LibraryPlaybackPreferencesForm,
  type SettingsTab,
} from "@/pages/settings/settingsTypes";

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
    if (
      !isAdmin &&
      activeTab !== "general" &&
      activeTab !== "playback" &&
      activeTab !== "subtitles"
    ) {
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

  const generalTabContent = user ? (
    <SettingsGeneralTab
      userEmail={user.email}
      quickConnect={quickConnect}
      quickConnectBusy={quickConnectBusy}
      quickConnectErr={quickConnectErr}
      onGenerateQuickConnect={() => void generateQuickConnect()}
    />
  ) : null;

  const playbackTabContent = (
    <SettingsPlaybackTab
      librariesQuery={librariesQuery}
      libraries={libraries}
      libraryForms={libraryForms}
      librarySaveMessages={librarySaveMessages}
      savingLibraryId={savingLibraryId}
      cloneLibraryPlaybackPreferences={cloneLibraryPlaybackPreferences}
      setLibraryField={setLibraryField}
      saveLibraryPreferences={saveLibraryPreferences}
    />
  );

  const subtitlesTabContent = <SettingsSubtitlesTab />;

  // ── Tab: Media Stack ───────────────────────────────────────────────────────

  const mediaStackTabContent = (
    <SettingsMediaStackTab
      mediaStackQuery={mediaStackQuery}
      mediaStackForm={mediaStackForm}
      mediaStackValidation={mediaStackValidation}
      mediaStackSaveMessage={mediaStackSaveMessage}
      mediaStackSaveTone={mediaStackSaveTone}
      mediaStackDirty={mediaStackDirty}
      mediaStackSaving={updateMediaStack.isPending}
      handleValidateMediaStack={handleValidateMediaStack}
      handleSaveMediaStack={handleSaveMediaStack}
      setMediaStackServiceField={setMediaStackServiceField}
    />
  );

  const arrProfilesTabContent = (
    <SettingsArrProfilesTab
      mediaStackQuery={mediaStackQuery}
      mediaStackForm={mediaStackForm}
      mediaStackValidation={mediaStackValidation}
      mediaStackSaveMessage={mediaStackSaveMessage}
      mediaStackSaveTone={mediaStackSaveTone}
      mediaStackDirty={mediaStackDirty}
      mediaStackSaving={updateMediaStack.isPending}
      handleValidateMediaStack={handleValidateMediaStack}
      handleSaveMediaStack={handleSaveMediaStack}
      setMediaStackServiceField={setMediaStackServiceField}
    />
  );

  const metadataTabContent = (
    <SettingsMetadataTab
      metadataArtworkQuery={metadataArtworkQuery}
      metadataArtworkForm={metadataArtworkForm}
      metadataArtworkAvailabilityByProvider={metadataArtworkAvailabilityByProvider}
      metadataArtworkSaveMessage={metadataArtworkSaveMessage}
      metadataArtworkDirty={metadataArtworkDirty}
      metadataArtworkSaving={updateMetadataArtwork.isPending}
      handleSaveMetadataArtwork={handleSaveMetadataArtwork}
      setArtworkField={setArtworkField}
      setEpisodeArtworkField={setEpisodeArtworkField}
    />
  );

  const transcodingTabContent = (
    <SettingsTranscodingTab
      settingsQuery={settingsQuery}
      form={form}
      warnings={warnings}
      saveMessage={saveMessage}
      dirty={dirty}
      saving={updateSettings.isPending}
      handleSave={handleSave}
      setField={setField}
      setDecodeCodec={setDecodeCodec}
      setEncodeFormat={setEncodeFormat}
    />
  );

  // ── Tab definitions ────────────────────────────────────────────────────────

  const navItemBase =
    "relative flex items-center gap-2 px-2.5 py-1.5 text-sm font-medium rounded-md transition-all cursor-pointer select-none w-full";
  const navItemActive =
    "text-(--plum-text) bg-[rgba(181,123,255,0.1)] before:absolute before:left-0 before:top-1/2 before:-translate-y-1/2 before:h-5 before:w-[3px] before:rounded-r-full before:bg-(--plum-accent) before:content-[''] shadow-[0_0_20px_rgba(139,92,246,0.08)]";
  const navItemInactive =
    "text-(--plum-muted) hover:text-(--plum-text) hover:bg-[rgba(181,123,255,0.06)]";

  type TabItem = { id: SettingsTab; label: string; icon: ReactNode };
  const adminTabSections: { heading: string; tabs: TabItem[] }[] = [
    {
      heading: "Host",
      tabs: [
        { id: "server-env", label: "Environment", icon: <Server className="size-4 shrink-0" /> },
        { id: "admin", label: "Admin", icon: <Shield className="size-4 shrink-0" /> },
      ],
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
    {
      heading: "Plugins",
      tabs: [
        { id: "plugins-intro-skipper", label: "Intro Skipper", icon: <SkipForward className="size-4 shrink-0" /> },
      ],
    },
  ];

  const tabContent: Record<SettingsTab, ReactNode> = {
    general: generalTabContent,
    playback: playbackTabContent,
    subtitles: subtitlesTabContent,
    "server-env": <ServerEnvSettingsTab />,
    admin: <AdminSettingsTab />,
    "media-stack": mediaStackTabContent,
    "arr-profiles": arrProfilesTabContent,
    metadata: metadataTabContent,
    transcoding: transcodingTabContent,
    "plugins-intro-skipper": <IntroSkipperPluginTab />,
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
              onClick={() => setActiveTab("general")}
              className={cn(
                navItemBase,
                activeTab === "general" ? navItemActive : navItemInactive,
              )}
              aria-current={activeTab === "general" ? "page" : undefined}
            >
              <CircleUser className="size-4 shrink-0" />
              <span className="truncate">General</span>
            </button>
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
            <button
              type="button"
              onClick={() => setActiveTab("subtitles")}
              className={cn(
                navItemBase,
                activeTab === "subtitles" ? navItemActive : navItemInactive,
              )}
              aria-current={activeTab === "subtitles" ? "page" : undefined}
            >
              <Captions className="size-4 shrink-0" />
              <span className="truncate">Subtitles</span>
            </button>
          </div>
          {isAdmin ? <div className="my-3 border-t border-(--plum-border)" /> : null}
          {isAdmin ? (
            <p className="mb-2 px-2 text-[10px] font-semibold uppercase tracking-[0.2em] text-(--plum-muted)">
              Advanced
            </p>
          ) : null}
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
        </nav>

        <div className="min-w-0 flex-1 pb-2">{tabContent[activeTab]}</div>
      </div>
    </div>
  );
}
