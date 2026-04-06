import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type ChangeEvent,
  type CSSProperties,
  type PointerEvent,
  type RefObject,
  type SyntheticEvent,
} from "react";
import { useQueryClient } from "@tanstack/react-query";
import Hls from "hls.js";
import { PLAYBACK_PROGRESS_HEARTBEAT_MS } from "@plum/contracts";
import {
  embeddedSubtitleAssUrl,
  embeddedSubtitleNeedsWebBurnIn,
  embeddedSubtitleUrl,
  externalSubtitleAssUrl,
  externalSubtitleUrl,
  mediaStreamUrl,
  resolveBackdropUrl,
  resolvePosterUrl,
} from "@plum/shared";
import {
  FastForward,
  Maximize2,
  Settings,
  Minimize2,
  Pause,
  Play,
  Ratio,
  Repeat,
  Rewind,
  SkipBack,
  SkipForward,
  Subtitles,
  Volume2,
  VolumeX,
  X,
} from "lucide-react";
import {
  BASE_URL,
  refreshPlaybackTracks,
  updateMediaProgress,
  type EmbeddedAudioTrack,
  type EmbeddedSubtitle,
  type MediaItem,
  type PlaybackTrackMetadata,
  type Subtitle,
} from "../api";

const EMPTY_PLAYBACK_QUEUE: MediaItem[] = [];
import {
  usePlayerQueue,
  usePlayerSession,
  usePlayerTransport,
} from "../contexts/PlayerContext";
import {
  getPlayerLocalSettingsSnapshot,
  isEnglishSubtitleTrackForMenu,
  languageMatchesPreference,
  normalizeLanguagePreference,
  PLAYER_WEB_TRACK_LANGUAGE_NONE,
  mergeShowTrackDefaultsForEpisode,
  readStoredPlayerWebDefaults,
  resolveEffectiveWebTrackDefaults,
  resolveLibraryPlaybackPreferences,
  subscribePlayerLocalSettings,
  subtitleFontSizeValue,
  subtitlePositionOptions,
  subtitleSizeOptions,
  formatDetectedVideoAspectLabel,
  videoAspectModeOptions,
  writeStoredPlayerWebDefaults,
  writeStoredSubtitleAppearance,
  writeStoredVideoAutoplayEnabled,
  writeStoredVideoAspectMode,
  type SubtitleAppearance,
  type VideoAspectMode,
} from "../lib/playbackPreferences";
import {
  applyCueLineSetting,
  bufferedRangeStartsNearZero,
  buildSubtitleCues,
  clearTextTrackCues,
  consumeSubtitleResponseWithPartialUpdates,
  findHlsSubtitleTrackIndexForPlumKey,
  formatClock,
  formatHlsErrorMessage,
  formatTrackLabel,
  getBrowserAudioTracks,
  getPreferredAudioKey,
  getPreferredSubtitleKey,
  getSeasonEpisodeLabel,
  hasTextTrack,
  nudgeVideoIntoBufferedRange,
  clampVideoSeekSeconds,
  resolvedVideoDuration,
  seekUpperBoundSeconds,
} from "../lib/playback/playerMedia";
import type {
  AudioTrackOption,
  HlsErrorData,
  SubtitleTrackOption,
  TrackMenuOption,
} from "../lib/playback/playerMedia";
import { queryKeys, useLibraries } from "../queries";
import { JassubRenderer } from "./JassubRenderer";

type PlaybackState = {
  currentTime: number;
  duration: number;
  isPlaying: boolean;
};

type LoadedSubtitleTrack = SubtitleTrackOption & {
  body: string;
};

type PlaybackTrackMetadataInput = {
  subtitles?: readonly Subtitle[];
  embeddedSubtitles?: readonly EmbeddedSubtitle[];
  embeddedAudioTracks?: readonly EmbeddedAudioTrack[];
};

type PlaybackTrackSource = PlaybackTrackMetadataInput & {
  mediaId: number;
};

type VideoProgressSnapshot = {
  mediaId: number;
  positionSeconds: number;
  durationSeconds: number;
  shouldResumePlayback: boolean;
  ended: boolean;
};

type QueuedSubtitlePreference =
  | { kind: "off" }
  | {
      kind: "track";
      label: string;
      language: string;
    };

const CONTROLS_HIDE_DELAY = 3000;
/** Sidecar SRT/VTT etc. */
const SUBTITLE_LOAD_TIMEOUT_MS = 45_000;
/** Embedded tracks are transcoded server-side (often full-file ffmpeg); client abort was killing ffmpeg at ~45s. */
const EMBEDDED_SUBTITLE_LOAD_TIMEOUT_MS = 600_000;
const VIDEO_PREVIOUS_RESTART_THRESHOLD_SECONDS = 5;
const VIDEO_SKIP_BUTTON_SECONDS = 30;
const UPNEXT_COUNTDOWN_SECONDS = 10;

/* ── Track popover menu (shared between docked & fullscreen) ── */
function TrackMenu({
  options,
  selectedKey,
  onSelect,
  menuRef,
  position = "above",
  ariaLabel,
  offLabel,
}: {
  options: TrackMenuOption[];
  selectedKey: string;
  onSelect: (key: string) => void;
  menuRef: RefObject<HTMLDivElement | null>;
  position?: "above" | "below";
  ariaLabel: string;
  offLabel?: string;
}) {
  return (
    <div
      ref={menuRef}
      className={`subtitle-menu subtitle-menu--${position}`}
      role="listbox"
      aria-label={ariaLabel}
    >
      {offLabel && (
        <button
          type="button"
          role="option"
          aria-selected={selectedKey === "off"}
          className={`subtitle-menu__item${selectedKey === "off" ? " is-selected" : ""}`}
          onClick={() => onSelect("off")}
        >
          <span className="subtitle-menu__check">{selectedKey === "off" ? "✓" : ""}</span>
          <span>{offLabel}</span>
        </button>
      )}
      {options.map((option) => (
        <button
          key={option.key}
          type="button"
          role="option"
          aria-selected={selectedKey === option.key}
          disabled={option.disabled}
          className={`subtitle-menu__item${selectedKey === option.key ? " is-selected" : ""}`}
          onClick={() => onSelect(option.key)}
        >
          <span className="subtitle-menu__check">{selectedKey === option.key ? "✓" : ""}</span>
          <span>{option.label}</span>
        </button>
      ))}
    </div>
  );
}

function PlayerSettingsMenu({
  menuRef,
  preferences,
  videoAutoplayEnabled,
  onChange,
  onVideoAutoplayChange,
}: {
  menuRef: RefObject<HTMLDivElement | null>;
  preferences: SubtitleAppearance;
  videoAutoplayEnabled: boolean;
  onChange: (value: SubtitleAppearance) => void;
  onVideoAutoplayChange: (enabled: boolean) => void;
}) {
  return (
    <div
      ref={menuRef}
      className="player-settings-menu"
      role="dialog"
      aria-label="Player settings"
    >
      <div className="player-settings-menu__field">
        <span id="player-settings-subtitle-size">Subtitle size</span>
        <div
          className="player-settings-menu__choice-row player-settings-menu__choice-row--thirds"
          role="group"
          aria-labelledby="player-settings-subtitle-size"
        >
          {subtitleSizeOptions.map((option) => (
            <button
              key={option.value}
              type="button"
              className={`player-settings-menu__choice${preferences.size === option.value ? " is-active" : ""}`}
              onClick={() => onChange({ ...preferences, size: option.value })}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>

      <div className="player-settings-menu__field">
        <span id="player-settings-subtitle-location">Subtitle location</span>
        <div
          className="player-settings-menu__choice-row"
          role="group"
          aria-labelledby="player-settings-subtitle-location"
        >
          {subtitlePositionOptions.map((option) => (
            <button
              key={option.value}
              type="button"
              className={`player-settings-menu__choice${preferences.position === option.value ? " is-active" : ""}`}
              onClick={() => onChange({ ...preferences, position: option.value })}
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>

      <label className="player-settings-menu__field">
        <span>Subtitle color</span>
        <input
          type="color"
          value={preferences.color}
          onChange={(event) =>
            onChange({
              ...preferences,
              color: event.target.value,
            })
          }
        />
      </label>

      <label className="player-settings-menu__field player-settings-menu__checkbox-row">
        <input
          type="checkbox"
          checked={videoAutoplayEnabled}
          onChange={(event) => onVideoAutoplayChange(event.target.checked)}
        />
        <span>Autoplay next</span>
      </label>
    </div>
  );
}

function PlayerLoadingOverlay({
  label,
  fullscreen = false,
}: {
  label: string;
  fullscreen?: boolean;
}) {
  const ariaLabel = label.trim() !== "" ? label : "Loading";
  return (
    <div
      className={`player-loading-overlay${fullscreen ? " player-loading-overlay--fullscreen" : ""}`}
      role="status"
      aria-live="polite"
      aria-label={ariaLabel}
    >
      <div className="player-loading-overlay__spinner" aria-hidden="true" />
      {label.trim() !== "" ? (
        <span className="player-loading-overlay__label">{label}</span>
      ) : null}
    </div>
  );
}

function embeddedStreamIndexFromKey(key: string): number | null {
  if (!key.startsWith("emb-")) return null;
  const n = Number(key.slice(4));
  return Number.isFinite(n) ? n : null;
}

function isAssFormat(format: string): boolean {
  const f = format.trim().toLowerCase();
  return f === "ass" || f === "ssa";
}

function buildSubtitleTrackRequests(source: PlaybackTrackSource | null): SubtitleTrackOption[] {
  if (source == null) return [];
  const external =
    source.subtitles?.map((subtitle, index) => {
      const assEligible = isAssFormat(subtitle.format ?? "");
      return {
        key: `ext-${subtitle.id}`,
        label: subtitle.title || subtitle.language || `Subtitle ${index + 1}`,
        src: externalSubtitleUrl(BASE_URL, subtitle.id),
        srcLang: subtitle.language || "und",
        supported: true,
        assEligible,
        assSrc: assEligible ? externalSubtitleAssUrl(BASE_URL, subtitle.id) : undefined,
      };
    }) ?? [];
  const embedded =
    source.embeddedSubtitles?.map((subtitle, index) => {
      const catalogOk = subtitle.supported !== false;
      const requiresBurn = catalogOk && embeddedSubtitleNeedsWebBurnIn(subtitle);
      const assEligible = catalogOk && !requiresBurn && subtitle.assEligible === true;
      const labelBase =
        subtitle.title || subtitle.language || `Embedded subtitle ${index + 1}`;
      const label = !catalogOk
        ? `${labelBase} (Unavailable)`
        : requiresBurn
          ? `${labelBase} (burn-in)`
          : labelBase;
      return {
        key: `emb-${subtitle.streamIndex}`,
        label,
        src: embeddedSubtitleUrl(BASE_URL, source.mediaId, subtitle.streamIndex),
        srcLang: subtitle.language || "und",
        supported: catalogOk,
        disabled: !catalogOk,
        requiresBurn,
        assEligible,
        assSrc: assEligible
          ? embeddedSubtitleAssUrl(BASE_URL, source.mediaId, subtitle.streamIndex)
          : undefined,
      };
    }) ?? [];
  return [...external, ...embedded];
}

function clonePlaybackTrackMetadata(metadata: PlaybackTrackMetadataInput): PlaybackTrackMetadata {
  return {
    subtitles: metadata.subtitles?.map((subtitle) => ({ ...subtitle })),
    embeddedSubtitles: metadata.embeddedSubtitles?.map((subtitle) => ({ ...subtitle })),
    embeddedAudioTracks: metadata.embeddedAudioTracks?.map((track) => ({ ...track })),
  };
}

export function PlaybackDock() {
  const queryClient = useQueryClient();
  const { data: libraries = [], isFetched: librariesFetched } = useLibraries();
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const playerRootRef = useRef<HTMLElement | null>(null);
  const lastPersistedRef = useRef<{
    mediaId: number;
    positionSeconds: number;
    completed: boolean;
  } | null>(null);
  const initialProgressPersistedRef = useRef<number | null>(null);
  const resumeAppliedRef = useRef<number | null>(null);
  const defaultTrackSelectionAppliedRef = useRef<number | null>(null);
  const [playbackState, setPlaybackState] = useState<PlaybackState>({
    currentTime: 0,
    duration: 0,
    isPlaying: false,
  });
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
  const [selectedSubtitleKey, setSelectedSubtitleKey] = useState("off");
  const [activeAssSource, setActiveAssSource] = useState<string | null>(null);
  /** Mirrored from the video ref so JassubRenderer re-renders when the element mounts (ref alone does not). */
  const [jassubVideoElement, setJassubVideoElement] = useState<HTMLVideoElement | null>(null);
  const [loadedSubtitleTracks, setLoadedSubtitleTracks] = useState<LoadedSubtitleTrack[]>([]);
  const [refreshedPlaybackTracks, setRefreshedPlaybackTracks] = useState<PlaybackTrackMetadata | null>(null);
  const [subtitleStatusMessage, setSubtitleStatusMessage] = useState("");
  const [selectedAudioKey, setSelectedAudioKey] = useState("");
  const [audioTrackVersion, setAudioTrackVersion] = useState(0);
  const [videoAttachmentVersion, setVideoAttachmentVersion] = useState(0);
  const [subtitleAttachmentVersion, setSubtitleAttachmentVersion] = useState(0);
  const [subtitleReadyVersion, setSubtitleReadyVersion] = useState(0);
  const [subtitleMenuOpen, setSubtitleMenuOpen] = useState(false);
  const [audioMenuOpen, setAudioMenuOpen] = useState(false);
  const [aspectMenuOpen, setAspectMenuOpen] = useState(false);
  const [playerSettingsOpen, setPlayerSettingsOpen] = useState(false);
  const [detectedVideoAspectLabel, setDetectedVideoAspectLabel] = useState<string | null>(null);
  const [browserFullscreenActive, setBrowserFullscreenActive] = useState(false);
  const [upNextTarget, setUpNextTarget] = useState<MediaItem | null>(null);
  const [upNextSecondsLeft, setUpNextSecondsLeft] = useState(UPNEXT_COUNTDOWN_SECONDS);
  const [resumePrompt, setResumePrompt] = useState<{ seconds: number; mediaId: number } | null>(null);
  const [isVideoLoading, setIsVideoLoading] = useState(false);
  const [pendingSubtitleKey, setPendingSubtitleKey] = useState<string | null>(null);
  const subtitleMenuRef = useRef<HTMLDivElement | null>(null);
  const subtitleBtnRef = useRef<HTMLButtonElement | null>(null);
  const audioMenuRef = useRef<HTMLDivElement | null>(null);
  const audioBtnRef = useRef<HTMLButtonElement | null>(null);
  const aspectMenuRef = useRef<HTMLDivElement | null>(null);
  const aspectBtnRef = useRef<HTMLButtonElement | null>(null);
  const playerSettingsMenuRef = useRef<HTMLDivElement | null>(null);
  const playerSettingsBtnRef = useRef<HTMLButtonElement | null>(null);
  const hlsRef = useRef<Hls | null>(null);
  const requestedAudioTrackRef = useRef<{ mediaId: number; key: string } | null>(null);
  const dispatchedAudioTrackRef = useRef<{ mediaId: number; key: string } | null>(null);
  const [controlsVisible, setControlsVisible] = useState(true);
  const hideTimerRef = useRef<ReturnType<typeof setTimeout>>(0);
  const overlayRef = useRef<HTMLDivElement | null>(null);
  const seekToAfterReloadRef = useRef<number | null>(null);
  const resumePlaybackAfterReloadRef = useRef(false);
  /** After a same-item source reload with playback left paused, skip programmatic play on `canplay` (browser `autoPlay` is unreliable after async session setup). */
  const suppressVideoAutoplayOnCanPlayRef = useRef(false);
  /** True until the first `playing` event for the current item — avoids resuming on every `canplay` after the user pauses. */
  const kickstartVideoPlaybackRef = useRef(false);
  const previousVideoSourceUrlRef = useRef("");
  /** Tracks stream URL / item for a deferred play() after the async session URL lands (user activation is gone by then). */
  const prevAutoplayStreamUrlRef = useRef("");
  const prevAutoplayItemIdRef = useRef<number | null>(null);
  const [hlsStatusMessage, setHlsStatusMessage] = useState("");
  const mediaRecoveryAttemptsRef = useRef(0);
  const networkRecoveryAttemptsRef = useRef(0);
  const initialBufferGapHandledRef = useRef(false);
  /** After the first `playing` event, brief `waiting` / `loadstart` must not show the full-screen loading overlay. */
  const videoPlaybackStartedRef = useRef(false);
  const manualSubtitleTrackRef = useRef<TextTrack | null>(null);
  const manualSubtitleVideoRef = useRef<HTMLVideoElement | null>(null);
  const subtitleLoadControllersRef = useRef<Map<string, AbortController>>(new Map());
  const blockedSubtitleRetryKeysRef = useRef<Set<string>>(new Set());
  const currentSubtitleMediaIdRef = useRef<number | null>(null);
  const lastVideoProgressRef = useRef<VideoProgressSnapshot | null>(null);
  const queuedSubtitlePreferenceRef = useRef<QueuedSubtitlePreference | null>(null);
  const manualSubtitleSelectionRef = useRef<number | null>(null);
  const manualAudioSelectionRef = useRef<number | null>(null);
  const {
    activeItem,
    activeMode,
    isDockOpen,
    videoSourceUrl,
    playbackDurationSeconds,
    videoDelivery,
    videoAudioIndex,
    wsConnected,
    lastEvent,
    dismissDock,
    burnEmbeddedSubtitleStreamIndex,
  } = usePlayerSession();
  const { queue, queueIndex, repeatMode, playNextInQueue, playPreviousInQueue } =
    usePlayerQueue();
  const {
    volume,
    muted,
    registerMediaElement,
    togglePlayPause,
    seekTo,
    setMuted,
    setVolume,
    changeAudioTrack,
    changeEmbeddedSubtitleBurn,
  } = usePlayerTransport();
  const captureSubtitleResumePosition = useCallback(() => {
    const v = videoRef.current;
    if (v && Number.isFinite(v.currentTime) && v.currentTime > 0) {
      seekToAfterReloadRef.current = v.currentTime;
      resumePlaybackAfterReloadRef.current = !v.paused && !v.ended;
    }
  }, []);

  const seekToRef = useRef(seekTo);
  const seekSliderRef = useRef<HTMLInputElement | null>(null);
  const scrubWindowListenersRef = useRef<(() => void) | null>(null);
  const playbackQueue = queue ?? EMPTY_PLAYBACK_QUEUE;
  const playNextInQueueRef = useRef(playNextInQueue);
  const registerMediaElementRef = useRef(registerMediaElement);

  useEffect(() => {
    playNextInQueueRef.current = playNextInQueue;
  }, [playNextInQueue]);

  useEffect(() => {
    seekToRef.current = seekTo;
  }, [seekTo]);

  const removeScrubWindowListeners = useCallback(() => {
    scrubWindowListenersRef.current?.();
    scrubWindowListenersRef.current = null;
  }, []);

  useEffect(
    () => () => {
      scrubWindowListenersRef.current?.();
      scrubWindowListenersRef.current = null;
    },
    [],
  );

  const upNextIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const clearUpNextTimer = useCallback(() => {
    if (upNextIntervalRef.current != null) {
      clearInterval(upNextIntervalRef.current);
      upNextIntervalRef.current = null;
    }
  }, []);

  const dismissUpNext = useCallback(() => {
    clearUpNextTimer();
    setUpNextTarget(null);
  }, [clearUpNextTimer]);

  const confirmUpNextNow = useCallback(() => {
    clearUpNextTimer();
    setUpNextTarget(null);
    playNextInQueue();
  }, [clearUpNextTimer, playNextInQueue]);

  const handleResumeFromProgress = useCallback(() => {
    const prompt = resumePrompt;
    if (!prompt) return;
    const element = videoRef.current;
    if (element && activeItem) {
      const delivery = videoDelivery ?? "direct";
      element.currentTime = clampVideoSeekSeconds(
        element,
        prompt.seconds,
        playbackDurationSeconds,
        activeItem.duration,
        delivery,
      );
      void element.play().catch(() => {});
    }
    resumeAppliedRef.current = prompt.mediaId;
    setResumePrompt(null);
  }, [activeItem, playbackDurationSeconds, resumePrompt, videoDelivery]);

  const handleStartFromBeginning = useCallback(() => {
    const prompt = resumePrompt;
    if (!prompt) return;
    const element = videoRef.current;
    if (element) {
      element.currentTime = 0;
      void element.play().catch(() => {});
    }
    resumeAppliedRef.current = prompt.mediaId;
    setResumePrompt(null);
  }, [resumePrompt]);

  useEffect(() => {
    if (!upNextTarget) {
      clearUpNextTimer();
      return;
    }
    setUpNextSecondsLeft(UPNEXT_COUNTDOWN_SECONDS);
    let remaining = UPNEXT_COUNTDOWN_SECONDS;
    upNextIntervalRef.current = setInterval(() => {
      remaining -= 1;
      setUpNextSecondsLeft(remaining);
      if (remaining <= 0) {
        clearUpNextTimer();
        setUpNextTarget(null);
        playNextInQueueRef.current();
      }
    }, 1000);
    return () => clearUpNextTimer();
  }, [upNextTarget, clearUpNextTimer]);

  const prevQueueIndexRef = useRef(queueIndex);
  useEffect(() => {
    if (upNextTarget != null && queueIndex !== prevQueueIndexRef.current) {
      dismissUpNext();
    }
    prevQueueIndexRef.current = queueIndex;
  }, [queueIndex, upNextTarget, dismissUpNext]);

  useEffect(() => {
    if (activeMode !== "video" || !isDockOpen) {
      dismissUpNext();
      setResumePrompt(null);
    }
  }, [activeMode, isDockOpen, dismissUpNext]);

  const upNextBackdropUrl = useMemo(() => {
    if (!upNextTarget) return "";
    const fromBackdrop = resolveBackdropUrl(
      upNextTarget.backdrop_url,
      upNextTarget.backdrop_path,
      "original",
      BASE_URL,
    );
    if (fromBackdrop) return fromBackdrop;
    return (
      resolvePosterUrl(
        upNextTarget.show_poster_url,
        upNextTarget.show_poster_path,
        "original",
        BASE_URL,
      ) ||
      resolvePosterUrl(upNextTarget.poster_url, upNextTarget.poster_path, "original", BASE_URL)
    );
  }, [upNextTarget]);

  const isVideo = activeMode === "video" && activeItem != null;
  const isWindowPlayer = isVideo;
  const activeItemId = activeItem?.id ?? null;
  const activeItemDuration = activeItem?.duration ?? 0;
  const hasNextQueueItem = queueIndex < playbackQueue.length - 1;
  const hasVideoQueueNavigation = isVideo && playbackQueue.length > 1;
  currentSubtitleMediaIdRef.current = activeItemId;
  const videoStatusMessage =
    hlsStatusMessage ||
    subtitleStatusMessage ||
    lastEvent ||
    (wsConnected ? "Waiting for transcode updates" : "WebSocket disconnected");
  const playerLoadingLabelRaw =
    hlsStatusMessage && !hlsStatusMessage.startsWith("Stream error:")
      ? hlsStatusMessage
      : lastEvent && !lastEvent.startsWith("Error:")
        ? lastEvent
        : "Preparing playback...";
  const playerLoadingLabel =
    playerLoadingLabelRaw === "Stream ready" ? "" : playerLoadingLabelRaw;
  const showPlayerLoadingOverlay =
    isVideo &&
    (isVideoLoading ||
      (hlsStatusMessage !== "" && !hlsStatusMessage.startsWith("Stream error:")));
  const videoSourceIsHls = useMemo(
    () => /\.m3u8(?:$|\?)/i.test(videoSourceUrl),
    [videoSourceUrl],
  );

  useEffect(() => {
    setDetectedVideoAspectLabel(null);
  }, [activeItemId]);

  useEffect(() => {
    const el = videoRef.current;
    if (!isVideo || !el) return;
    const onResize = () => {
      setDetectedVideoAspectLabel(
        formatDetectedVideoAspectLabel(el.videoWidth, el.videoHeight),
      );
    };
    el.addEventListener("resize", onResize);
    return () => el.removeEventListener("resize", onResize);
  }, [isVideo, activeItemId, videoAttachmentVersion]);

  useEffect(() => {
    setIsVideoLoading(isVideo && activeItemId != null);
  }, [activeItemId, isVideo]);
  const activeLibrary = useMemo(
    () => libraries.find((library) => library.id === activeItem?.library_id) ?? null,
    [activeItem?.library_id, libraries],
  );
  const libraryPlaybackPreferences = useMemo(
    () =>
      resolveLibraryPlaybackPreferences(
        activeLibrary ?? (activeItem ? { type: activeItem.type } : null),
      ),
    [activeItem, activeLibrary],
  );

  const effectiveWebTrackDefaults = useMemo(
    () =>
      resolveEffectiveWebTrackDefaults(activeItem ?? null, playerLocalSettings.webDefaults),
    [activeItem, playerLocalSettings.webDefaults],
  );

  const clientSubtitleAutoPickDisabled = useMemo(() => {
    return effectiveWebTrackDefaults.defaultSubtitleLanguage.trim() === PLAYER_WEB_TRACK_LANGUAGE_NONE;
  }, [effectiveWebTrackDefaults.defaultSubtitleLanguage]);

  const clientAudioAutoPickDisabled = useMemo(() => {
    return effectiveWebTrackDefaults.defaultAudioLanguage.trim() === PLAYER_WEB_TRACK_LANGUAGE_NONE;
  }, [effectiveWebTrackDefaults.defaultAudioLanguage]);

  const effectivePreferredSubtitleLanguage = useMemo(() => {
    if (clientSubtitleAutoPickDisabled) {
      return "";
    }
    const fromClient = effectiveWebTrackDefaults.defaultSubtitleLanguage.trim();
    if (fromClient !== "") {
      return normalizeLanguagePreference(effectiveWebTrackDefaults.defaultSubtitleLanguage);
    }
    return libraryPlaybackPreferences.preferredSubtitleLanguage;
  }, [
    clientSubtitleAutoPickDisabled,
    effectiveWebTrackDefaults.defaultSubtitleLanguage,
    libraryPlaybackPreferences.preferredSubtitleLanguage,
  ]);

  const effectivePreferredAudioLanguage = useMemo(() => {
    if (clientAudioAutoPickDisabled) {
      return "";
    }
    const fromClient = effectiveWebTrackDefaults.defaultAudioLanguage.trim();
    if (fromClient !== "") {
      return normalizeLanguagePreference(effectiveWebTrackDefaults.defaultAudioLanguage);
    }
    return libraryPlaybackPreferences.preferredAudioLanguage;
  }, [
    clientAudioAutoPickDisabled,
    effectiveWebTrackDefaults.defaultAudioLanguage,
    libraryPlaybackPreferences.preferredAudioLanguage,
  ]);

  /** Only used when the client overrides subtitle language (not “follow library”). */
  const effectiveSubtitleLabelHint = useMemo(() => {
    if (clientSubtitleAutoPickDisabled) return "";
    const sub = effectiveWebTrackDefaults.defaultSubtitleLanguage.trim();
    if (sub === "" || sub === PLAYER_WEB_TRACK_LANGUAGE_NONE) return "";
    return effectiveWebTrackDefaults.defaultSubtitleLabelHint.trim();
  }, [
    clientSubtitleAutoPickDisabled,
    effectiveWebTrackDefaults.defaultSubtitleLabelHint,
    effectiveWebTrackDefaults.defaultSubtitleLanguage,
  ]);

  const introEndSec = useMemo(() => {
    const end = activeItem?.intro_end_seconds;
    if (end == null || !Number.isFinite(end) || end <= 0) {
      return null;
    }
    return end;
  }, [activeItem?.intro_end_seconds]);

  const introStartSec = useMemo(() => {
    const s = activeItem?.intro_start_seconds;
    if (s != null && Number.isFinite(s) && s >= 0) {
      return s;
    }
    return 0;
  }, [activeItem?.intro_start_seconds]);

  const introSkipStateRef = useRef({ consumedAuto: false, suppressed: false, lastTime: 0 });
  const [introButtonDismissed, setIntroButtonDismissed] = useState(false);

  useEffect(() => {
    introSkipStateRef.current = { consumedAuto: false, suppressed: false, lastTime: 0 };
    setIntroButtonDismissed(false);
  }, [activeItemId]);

  const handleSkipIntroClick = useCallback(() => {
    if (introEndSec == null) return;
    seekTo(introEndSec);
    setIntroButtonDismissed(true);
  }, [introEndSec, seekTo]);

  const INTRO_END_MARGIN_SEC = 0.5;

  const processIntroSkip = useCallback(
    (video: HTMLVideoElement) => {
      const mode = libraryPlaybackPreferences.introSkipMode;
      const end = introEndSec;
      if (mode === "off" || end == null || !isVideo) {
        return;
      }
      const st = introStartSec;
      const t = Number.isFinite(video.currentTime) ? video.currentTime : 0;
      const state = introSkipStateRef.current;
      // Reset auto-skip when user seeks backward into/before the intro window
      if (t < state.lastTime - 1.0 && t < end) {
        state.consumedAuto = false;
        state.suppressed = false;
      }
      if (state.lastTime >= end && t < end - 0.25) {
        state.suppressed = true;
      }
      state.lastTime = t;
      if (t < st || t >= end - INTRO_END_MARGIN_SEC) {
        return;
      }
      if (mode === "auto" && !state.consumedAuto && !state.suppressed && video.readyState >= 2) {
        state.consumedAuto = true;
        seekTo(end);
      }
    },
    [introEndSec, introStartSec, isVideo, libraryPlaybackPreferences.introSkipMode, seekTo],
  );

  const showSkipIntroControl =
    isVideo &&
    introEndSec != null &&
    libraryPlaybackPreferences.introSkipMode !== "off" &&
    !introButtonDismissed &&
    playbackState.currentTime >= introStartSec &&
    playbackState.currentTime < introEndSec - INTRO_END_MARGIN_SEC;

  useEffect(() => {
    const previousUrl = previousVideoSourceUrlRef.current;
    previousVideoSourceUrlRef.current = videoSourceUrl;
    const sourceChanged = previousUrl !== videoSourceUrl;
    if (sourceChanged) {
      setIsVideoLoading(true);
      setHlsStatusMessage("");
      mediaRecoveryAttemptsRef.current = 0;
      networkRecoveryAttemptsRef.current = 0;
      initialBufferGapHandledRef.current = false;
      videoPlaybackStartedRef.current = false;
      setSubtitleReadyVersion(0);
    }
    if (!videoSourceUrl || !previousUrl || !sourceChanged) return;
    const video = videoRef.current;
    if (!video) return;
    seekToAfterReloadRef.current =
      Number.isFinite(video.currentTime) && video.currentTime > 0
        ? video.currentTime
        : playbackState.currentTime;
    resumePlaybackAfterReloadRef.current = !video.paused && !video.ended;
    video.pause();
    video.load();
  }, [playbackState.currentTime, videoSourceUrl]);

  useEffect(() => {
    const prevUrl = prevAutoplayStreamUrlRef.current;
    const prevItemId = prevAutoplayItemIdRef.current;

    if (!isVideo || !activeItemId || !videoSourceUrl) {
      prevAutoplayStreamUrlRef.current = videoSourceUrl;
      prevAutoplayItemIdRef.current = activeItemId;
      return;
    }

    prevAutoplayStreamUrlRef.current = videoSourceUrl;
    prevAutoplayItemIdRef.current = activeItemId;

    const streamJustBecameReady = prevUrl === "" && videoSourceUrl !== "";
    const switchedToAnotherItem =
      prevItemId != null && prevItemId !== activeItemId;

    if (!streamJustBecameReady && !switchedToAnotherItem) {
      return;
    }

    const video = videoRef.current;
    if (!video) return;

    const handle = window.setTimeout(() => {
      if (videoRef.current !== video) return;
      void video.play().catch(() => {});
    }, 0);
    return () => window.clearTimeout(handle);
  }, [activeItemId, isVideo, videoSourceUrl]);

  const playbackTrackSource = useMemo<PlaybackTrackSource | null>(() => {
    if (!isVideo || !activeItem) return null;
    return {
      mediaId: activeItem.id,
      subtitles: refreshedPlaybackTracks?.subtitles ?? activeItem.subtitles,
      embeddedSubtitles:
        refreshedPlaybackTracks?.embeddedSubtitles ?? activeItem.embeddedSubtitles,
      embeddedAudioTracks:
        refreshedPlaybackTracks?.embeddedAudioTracks ?? activeItem.embeddedAudioTracks,
    };
  }, [activeItem, isVideo, refreshedPlaybackTracks]);

  const subtitleTrackRequests = useMemo<SubtitleTrackOption[]>(
    () => buildSubtitleTrackRequests(playbackTrackSource),
    [playbackTrackSource],
  );

  const subtitleTrackOptions = subtitleTrackRequests;

  const subtitleMenuTrackOptions = useMemo(() => {
    if (!playerLocalSettings.webDefaults.subtitleMenuEnglishOnly) {
      return subtitleTrackRequests;
    }
    const filtered = subtitleTrackRequests.filter(
      (t) => t.key === "off" || isEnglishSubtitleTrackForMenu(t),
    );
    const selected = subtitleTrackRequests.find((t) => t.key === selectedSubtitleKey);
    if (
      selected != null &&
      selected.key !== "off" &&
      !filtered.some((t) => t.key === selected.key)
    ) {
      return [...filtered, selected];
    }
    return filtered;
  }, [
    playerLocalSettings.webDefaults.subtitleMenuEnglishOnly,
    selectedSubtitleKey,
    subtitleTrackRequests,
  ]);
  const hasSupportedSubtitleTracks = subtitleTrackRequests.some(
    (track) => track.supported !== false,
  );
  const rememberQueuedSubtitlePreference = useCallback(
    (key: string) => {
      if (key === "off") {
        queuedSubtitlePreferenceRef.current = { kind: "off" };
        return;
      }
      const track = subtitleTrackRequests.find((candidate) => candidate.key === key);
      if (!track) {
        queuedSubtitlePreferenceRef.current = null;
        return;
      }
      queuedSubtitlePreferenceRef.current = {
        kind: "track",
        label: track.label,
        language: track.srcLang,
      };
    },
    [subtitleTrackRequests],
  );
  const resolveQueuedSubtitleKey = useCallback(() => {
    const preference = queuedSubtitlePreferenceRef.current;
    if (!preference) return null;
    if (preference.kind === "off") return "off";
    const match =
      subtitleTrackOptions.find(
        (track) =>
          languageMatchesPreference(track.srcLang, preference.language) ||
          languageMatchesPreference(track.label, preference.language),
      ) ??
      subtitleTrackOptions.find((track) => track.label === preference.label) ??
      null;
    return match?.key ?? null;
  }, [subtitleTrackOptions]);
  const refreshActivePlaybackTracks = useCallback(
    async (statusMessage = "Refreshing subtitle tracks...") => {
      if (!isVideo || !activeItem) return null;
      const mediaId = activeItem.id;
      setSubtitleStatusMessage(statusMessage);
      try {
        const metadata = await refreshPlaybackTracks(mediaId);
        if (currentSubtitleMediaIdRef.current !== mediaId) {
          return null;
        }
        setRefreshedPlaybackTracks(clonePlaybackTrackMetadata(metadata));
        setSubtitleStatusMessage("");
        return metadata;
      } catch (error) {
        console.error("[PlaybackDock] Playback track refresh failed", {
          mediaId,
          error,
        });
        if (currentSubtitleMediaIdRef.current === mediaId) {
          setSubtitleStatusMessage("Subtitle refresh failed. Try again.");
        }
        return null;
      }
    },
    [activeItem, isVideo],
  );
  const ensureSubtitleTrackLoaded = useCallback(
    async (trackKey: string) => {
      if (trackKey === "off") return;
      if (loadedSubtitleTracks.some((track) => track.key === trackKey)) return;
      if (subtitleLoadControllersRef.current.has(trackKey)) return;
      if (blockedSubtitleRetryKeysRef.current.has(trackKey)) return;
      const track = subtitleTrackRequests.find((candidate) => candidate.key === trackKey);
      if (!track) return;
      if (track.requiresBurn) {
        setPendingSubtitleKey(null);
        return;
      }
      if (track.assEligible && track.assSrc) {
        // ASS tracks are rendered by JASSUB; no VTT load needed.
        setActiveAssSource(track.assSrc);
        setPendingSubtitleKey(null);
        setSubtitleStatusMessage("");
        return;
      }
      if (videoSourceIsHls && hlsRef.current) {
        const hlsIdx = findHlsSubtitleTrackIndexForPlumKey(hlsRef.current, trackKey);
        if (hlsIdx >= 0) {
          setPendingSubtitleKey(null);
          setSubtitleStatusMessage("");
          return;
        }
      }
      if (track.supported === false) {
        setSubtitleStatusMessage("This subtitle track is unavailable.");
        setPendingSubtitleKey(null);
        return;
      }

      const controller = new AbortController();
      subtitleLoadControllersRef.current.set(trackKey, controller);
      let timedOut = false;
      const subtitleTimeoutMs = track.src.includes("/subtitles/embedded/")
        ? EMBEDDED_SUBTITLE_LOAD_TIMEOUT_MS
        : SUBTITLE_LOAD_TIMEOUT_MS;
      const timeoutId =
        typeof window === "undefined"
          ? null
          : window.setTimeout(() => {
              timedOut = true;
              controller.abort();
            }, subtitleTimeoutMs);

      try {
        setSubtitleStatusMessage("Loading subtitles...");
        const response = await fetch(track.src, {
          credentials: "include",
          signal: controller.signal,
        });
        if (!response.ok) {
          throw new Error(`Subtitle request failed: ${response.status}`);
        }
        let lastFlushedCueCount = 0;
        let lastFlushedBodyLen = 0;
        await consumeSubtitleResponseWithPartialUpdates(
          response,
          controller.signal,
          (bodyForState, streamDone) => {
            const cues = buildSubtitleCues(bodyForState);
            if (!streamDone) {
              if (cues.length === 0) return;
              if (
                cues.length === lastFlushedCueCount &&
                bodyForState.length === lastFlushedBodyLen
              ) {
                return;
              }
              lastFlushedCueCount = cues.length;
              lastFlushedBodyLen = bodyForState.length;
            } else {
              lastFlushedCueCount = cues.length;
              lastFlushedBodyLen = bodyForState.length;
            }
            setLoadedSubtitleTracks((current) => {
              const rest = current.filter((candidate) => candidate.key !== track.key);
              return [...rest, { ...track, body: bodyForState }];
            });
            if (cues.length > 0) {
              setSubtitleStatusMessage("");
            }
          },
        );
        blockedSubtitleRetryKeysRef.current.delete(track.key);
        setPendingSubtitleKey((current) => (current === track.key ? null : current));
        setSubtitleStatusMessage("");
      } catch (error) {
        let loadError: unknown = error;
        if (
          (error instanceof DOMException && error.name === "AbortError") ||
          controller.signal.aborted
        ) {
          if (!timedOut) {
            return;
          }
          loadError = new Error("Subtitle request timed out");
        }
        console.error("[PlaybackDock] Subtitle load failed", {
          mediaId: activeItem?.id ?? null,
          source: track.src,
          error: loadError,
        });
        setLoadedSubtitleTracks((current) =>
          current.filter((candidate) => candidate.key !== track.key),
        );
        blockedSubtitleRetryKeysRef.current.add(track.key);
        setPendingSubtitleKey((current) => (current === track.key ? null : current));
        setSubtitleStatusMessage(
          loadError instanceof Error && loadError.message === "Subtitle request timed out"
            ? "Subtitle load timed out. Try again."
            : "Subtitle load failed. Try again.",
        );
      } finally {
        if (timeoutId != null) {
          window.clearTimeout(timeoutId);
        }
        subtitleLoadControllersRef.current.delete(trackKey);
      }
    },
    [
      activeItem?.id,
      loadedSubtitleTracks,
      subtitleTrackRequests,
      videoSourceIsHls,
    ],
  );

  useEffect(() => {
    queuedSubtitlePreferenceRef.current = null;
  }, [playbackQueue]);

  // When the media first becomes ready (subtitleReadyVersion goes from 0 → 1), clear any blocked
  // subtitle key so the loading effect below can retry. This handles the case where the initial
  // auto-selected fetch failed transiently (e.g. the server was still warming the subtitle cache)
  // and the user would otherwise need to manually toggle to trigger a retry.
  useEffect(() => {
    if (selectedSubtitleKey !== "off" && subtitleReadyVersion === 1) {
      blockedSubtitleRetryKeysRef.current.delete(selectedSubtitleKey);
    }
  }, [selectedSubtitleKey, subtitleReadyVersion]);

  useEffect(() => {
    if (selectedSubtitleKey === "off") return;
    const req = subtitleTrackRequests.find((t) => t.key === selectedSubtitleKey);
    if (req?.requiresBurn) {
      setPendingSubtitleKey(null);
      return;
    }
    if (!loadedSubtitleTracks.some((track) => track.key === selectedSubtitleKey)) {
      setPendingSubtitleKey((current) =>
        current === selectedSubtitleKey ? current : selectedSubtitleKey,
      );
    }
    void ensureSubtitleTrackLoaded(selectedSubtitleKey);
  }, [
    ensureSubtitleTrackLoaded,
    loadedSubtitleTracks,
    selectedSubtitleKey,
    subtitleReadyVersion,
    subtitleTrackRequests,
  ]);

  useEffect(() => {
    if (selectedSubtitleKey === "off") {
      setPendingSubtitleKey(null);
      setSubtitleStatusMessage("");
      return;
    }
    if (loadedSubtitleTracks.some((track) => track.key === selectedSubtitleKey)) {
      setPendingSubtitleKey((current) => (current === selectedSubtitleKey ? null : current));
    }
  }, [loadedSubtitleTracks, selectedSubtitleKey]);

  useEffect(() => {
    if (pendingSubtitleKey == null) return;
    if (
      selectedSubtitleKey === "off" ||
      selectedSubtitleKey !== pendingSubtitleKey ||
      loadedSubtitleTracks.some((track) => track.key === pendingSubtitleKey) ||
      !subtitleLoadControllersRef.current.has(pendingSubtitleKey) ||
      !subtitleTrackRequests.some((track) => track.key === pendingSubtitleKey)
    ) {
      setPendingSubtitleKey((current) => (current == null ? current : null));
    }
  }, [
    loadedSubtitleTracks,
    pendingSubtitleKey,
    selectedSubtitleKey,
    subtitleTrackRequests,
  ]);

  const audioTracks = useMemo<AudioTrackOption[]>(() => {
    if (!isVideo || playbackTrackSource == null) return [];
    return (
      playbackTrackSource.embeddedAudioTracks?.map((track, index) => ({
        key: `aud-${track.streamIndex}`,
        label: formatTrackLabel(track.title, track.language, `Audio ${index + 1}`),
        streamIndex: track.streamIndex,
        language: track.language,
      })) ?? []
    );
  }, [isVideo, playbackTrackSource]);

  const selectedAudioIndex = useMemo(
    () => audioTracks.findIndex((track) => track.key === selectedAudioKey),
    [audioTracks, selectedAudioKey],
  );

  const selectedAudioLabel =
    (selectedAudioIndex >= 0 ? audioTracks[selectedAudioIndex]?.label : audioTracks[0]?.label) ||
    "Audio";
  const videoSubtitleStyle = useMemo(
    () =>
      ({
        "--plum-subtitle-color": subtitleAppearance.color,
        "--plum-subtitle-size": subtitleFontSizeValue(subtitleAppearance.size),
      }) as CSSProperties,
    [subtitleAppearance.color, subtitleAppearance.size],
  );

  const syncPlaybackState = useCallback(
    (element: HTMLMediaElement | null) => {
      if (!element) {
        setPlaybackState({ currentTime: 0, duration: 0, isPlaying: false });
        return;
      }
      const elementDuration =
        Number.isFinite(element.duration) && element.duration > 0 ? element.duration : 0;
      setPlaybackState({
        currentTime: Number.isFinite(element.currentTime) ? element.currentTime : 0,
        duration: isVideo
          ? resolvedVideoDuration(
              playbackDurationSeconds,
              activeItem?.duration ?? 0,
              elementDuration,
            )
          : elementDuration,
        isPlaying: !element.paused && !element.ended,
      });
    },
    [activeItem?.duration, isVideo, playbackDurationSeconds],
  );
  const syncPlaybackStateRef = useRef(syncPlaybackState);

  useEffect(() => {
    registerMediaElementRef.current = registerMediaElement;
  }, [registerMediaElement]);

  useEffect(() => {
    syncPlaybackStateRef.current = syncPlaybackState;
  }, [syncPlaybackState]);

  const markSubtitleReady = useCallback(() => {
    setSubtitleReadyVersion((value) => value + 1);
  }, []);

  const maybeRecoverInitialBufferGap = useCallback((video: HTMLVideoElement | null): boolean => {
    if (!video || initialBufferGapHandledRef.current) {
      return false;
    }

    if ((Number.isFinite(video.currentTime) ? video.currentTime : 0) > 1) {
      initialBufferGapHandledRef.current = true;
      return false;
    }

    if (bufferedRangeStartsNearZero(video)) {
      initialBufferGapHandledRef.current = true;
      return false;
    }

    const nudged = nudgeVideoIntoBufferedRange(video);
    if (nudged || video.buffered.length > 0) {
      initialBufferGapHandledRef.current = true;
    }
    return nudged;
  }, []);

  const captureVideoProgressSnapshot = useCallback(
    (element?: HTMLVideoElement | null): VideoProgressSnapshot | null => {
      if (!isVideo || !activeItem) return null;
      const candidate = element ?? videoRef.current;
      const fallback = lastVideoProgressRef.current;
      const fallbackDuration =
        fallback?.mediaId === activeItem.id ? fallback.durationSeconds : 0;
      const fallbackPosition =
        fallback?.mediaId === activeItem.id ? fallback.positionSeconds : playbackState.currentTime;
      const duration = resolvedVideoDuration(
        playbackDurationSeconds,
        activeItem.duration,
        candidate?.duration ?? fallbackDuration,
      );
      if (!Number.isFinite(duration) || duration <= 0) return null;
      const rawPosition =
        candidate && Number.isFinite(candidate.currentTime) ? candidate.currentTime : fallbackPosition;
      const delivery = videoDelivery ?? "direct";
      let positionCap = duration;
      if (candidate != null) {
        const ub = seekUpperBoundSeconds(
          candidate,
          playbackDurationSeconds,
          activeItem.duration,
          delivery,
        );
        if (ub > 0) {
          positionCap = Math.min(positionCap, ub);
        }
      }
      const positionSeconds = Math.max(0, Math.min(rawPosition, positionCap));
      const ended = candidate?.ended ?? (fallback?.mediaId === activeItem.id ? fallback.ended : false);
      return {
        mediaId: activeItem.id,
        positionSeconds,
        durationSeconds: duration,
        shouldResumePlayback:
          candidate != null
            ? !candidate.paused && !candidate.ended
            : (fallback?.mediaId === activeItem.id ? fallback.shouldResumePlayback : false),
        ended,
      };
    },
    [
      activeItem,
      isVideo,
      playbackDurationSeconds,
      playbackState.currentTime,
      videoDelivery,
    ],
  );

  const syncVideoProgressSnapshot = useCallback(
    (element: HTMLVideoElement | null) => {
      const snapshot = captureVideoProgressSnapshot(element);
      if (!snapshot) return;
      lastVideoProgressRef.current = snapshot;
    },
    [captureVideoProgressSnapshot],
  );

  const persistPlaybackProgress = useCallback(
    async (options?: { force?: boolean; completed?: boolean; snapshot?: VideoProgressSnapshot | null }) => {
      if (!isVideo || !activeItem) return;
      const snapshot =
        options?.snapshot && options.snapshot.mediaId === activeItem.id
          ? options.snapshot
          : captureVideoProgressSnapshot();
      if (!snapshot) return;
      lastVideoProgressRef.current = snapshot;
      const completed = options?.completed === true || snapshot.ended;
      const previous = lastPersistedRef.current;
      if (
        !options?.force &&
        previous?.mediaId === activeItem.id &&
        previous.completed === completed &&
        Math.abs(previous.positionSeconds - snapshot.positionSeconds) < 10
      ) {
        return;
      }
      await updateMediaProgress(activeItem.id, {
        position_seconds: snapshot.positionSeconds,
        duration_seconds: snapshot.durationSeconds,
        completed,
      }).catch(() => {});
      lastPersistedRef.current = {
        mediaId: activeItem.id,
        positionSeconds: snapshot.positionSeconds,
        completed,
      };
      if (activeItem.library_id != null) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.library(activeItem.library_id) });
      }
      void queryClient.invalidateQueries({ queryKey: queryKeys.home });
    },
    [activeItem, captureVideoProgressSnapshot, isVideo, queryClient],
  );
  const captureVideoProgressSnapshotRef = useRef(captureVideoProgressSnapshot);
  const syncVideoProgressSnapshotRef = useRef(syncVideoProgressSnapshot);
  const persistPlaybackProgressRef = useRef(persistPlaybackProgress);

  useEffect(() => {
    captureVideoProgressSnapshotRef.current = captureVideoProgressSnapshot;
  }, [captureVideoProgressSnapshot]);

  useEffect(() => {
    syncVideoProgressSnapshotRef.current = syncVideoProgressSnapshot;
  }, [syncVideoProgressSnapshot]);

  useEffect(() => {
    persistPlaybackProgressRef.current = persistPlaybackProgress;
  }, [persistPlaybackProgress]);

  const applyResumePosition = useCallback(
    (element: HTMLMediaElement) => {
      if (
        !isVideo ||
        !activeItem ||
        activeItem.completed ||
        resumeAppliedRef.current === activeItem.id
      ) {
        return;
      }
      const resumeAt = activeItem.progress_seconds ?? 0;
      if (!Number.isFinite(resumeAt) || resumeAt <= 0) {
        resumeAppliedRef.current = activeItem.id;
        return;
      }
      const delivery = videoDelivery ?? "direct";
      element.currentTime = clampVideoSeekSeconds(
        element,
        resumeAt,
        playbackDurationSeconds,
        activeItem.duration,
        delivery,
      );
      resumeAppliedRef.current = activeItem.id;
    },
    [activeItem, isVideo, playbackDurationSeconds, videoDelivery],
  );

  const persistInitialPlaybackProgress = useCallback(
    (element: HTMLVideoElement) => {
      if (!isVideo || !activeItem) return;
      if (initialProgressPersistedRef.current === activeItem.id) return;
      if (!Number.isFinite(element.currentTime) || element.currentTime <= 0) return;
      initialProgressPersistedRef.current = activeItem.id;
      const snapshot = captureVideoProgressSnapshot(element);
      void persistPlaybackProgress({ force: true, snapshot });
    },
    [activeItem, captureVideoProgressSnapshot, isVideo, persistPlaybackProgress],
  );

  const setVideoRef = useCallback((element: HTMLVideoElement | null) => {
    if (videoRef.current !== element) {
      if (videoRef.current && !element) {
        const snapshot = captureVideoProgressSnapshotRef.current(videoRef.current);
        if (snapshot) {
          lastVideoProgressRef.current = snapshot;
          seekToAfterReloadRef.current = snapshot.positionSeconds;
          resumePlaybackAfterReloadRef.current = snapshot.shouldResumePlayback;
          if (snapshot.positionSeconds > 0 || snapshot.ended) {
            void persistPlaybackProgressRef.current({ force: true, snapshot });
          }
        }
      }
      manualSubtitleTrackRef.current = null;
      manualSubtitleVideoRef.current = null;
      setVideoAttachmentVersion((value) => value + 1);
      setSubtitleAttachmentVersion((value) => value + 1);
    }
    videoRef.current = element;
    setJassubVideoElement(element);
    registerMediaElementRef.current("video", element);
    syncPlaybackStateRef.current(element);
    if (element) {
      syncVideoProgressSnapshotRef.current(element);
    }
  }, []);

  const setAudioRef = useCallback((element: HTMLAudioElement | null) => {
    audioRef.current = element;
    registerMediaElementRef.current("audio", element);
    syncPlaybackStateRef.current(element);
  }, []);

  const handleVideoLoadedMetadata = useCallback(
    (element: HTMLVideoElement) => {
      const seekToAfterReload = seekToAfterReloadRef.current;
      if (seekToAfterReload != null) {
        const delivery = videoDelivery ?? "direct";
        element.currentTime =
          activeItem != null
            ? clampVideoSeekSeconds(
                element,
                seekToAfterReload,
                playbackDurationSeconds,
                activeItem.duration,
                delivery,
              )
            : Math.max(0, seekToAfterReload);
        seekToAfterReloadRef.current = null;
        const shouldResumePlayback = resumePlaybackAfterReloadRef.current;
        resumePlaybackAfterReloadRef.current = false;
        if (shouldResumePlayback) {
          suppressVideoAutoplayOnCanPlayRef.current = false;
          void element.play().catch(() => {});
        } else {
          element.pause();
          suppressVideoAutoplayOnCanPlayRef.current = true;
          kickstartVideoPlaybackRef.current = false;
        }
      } else {
        const resumeAt = activeItem?.progress_seconds ?? 0;
        const hasResumableProgress =
          activeItem != null &&
          !activeItem.completed &&
          Number.isFinite(resumeAt) &&
          resumeAt > 0 &&
          resumeAppliedRef.current !== activeItem.id;
        if (hasResumableProgress) {
          element.pause();
          suppressVideoAutoplayOnCanPlayRef.current = true;
          kickstartVideoPlaybackRef.current = false;
          setResumePrompt({ seconds: resumeAt, mediaId: activeItem.id });
        } else {
          suppressVideoAutoplayOnCanPlayRef.current = false;
          applyResumePosition(element);
          void element.play().catch(() => {});
        }
      }
      syncPlaybackState(element);
      setAudioTrackVersion((value) => value + 1);
      markSubtitleReady();
      setIsVideoLoading(false);
      setDetectedVideoAspectLabel(
        formatDetectedVideoAspectLabel(element.videoWidth, element.videoHeight),
      );
    },
    [
      activeItem,
      applyResumePosition,
      markSubtitleReady,
      playbackDurationSeconds,
      syncPlaybackState,
      videoDelivery,
    ],
  );

  const handleVideoCanPlay = useCallback(
    (element: HTMLVideoElement) => {
      maybeRecoverInitialBufferGap(element);
      syncPlaybackState(element);
      syncVideoProgressSnapshot(element);
      markSubtitleReady();
      setIsVideoLoading(false);
      if (
        !suppressVideoAutoplayOnCanPlayRef.current &&
        kickstartVideoPlaybackRef.current
      ) {
        void element.play().catch(() => {});
      }
    },
    [maybeRecoverInitialBufferGap, markSubtitleReady, syncPlaybackState, syncVideoProgressSnapshot],
  );

  useEffect(() => {
    kickstartVideoPlaybackRef.current = isVideo && activeItemId != null;
  }, [activeItemId, isVideo]);

  useEffect(() => {
    setPlaybackState({
      currentTime: 0,
      duration: activeItemDuration,
      isPlaying: false,
    });
    setSelectedSubtitleKey("off");
    for (const controller of subtitleLoadControllersRef.current.values()) {
      controller.abort();
    }
    subtitleLoadControllersRef.current.clear();
    blockedSubtitleRetryKeysRef.current.clear();
    setLoadedSubtitleTracks([]);
    setRefreshedPlaybackTracks(null);
    setSubtitleStatusMessage("");
    setSelectedAudioKey("");
    setAudioTrackVersion(0);
    setVideoAttachmentVersion(0);
    setSubtitleAttachmentVersion(0);
    setSubtitleReadyVersion(0);
    initialProgressPersistedRef.current = null;
    resumeAppliedRef.current = null;
    setResumePrompt(null);
    defaultTrackSelectionAppliedRef.current = null;
    manualSubtitleSelectionRef.current = null;
    manualAudioSelectionRef.current = null;
    lastVideoProgressRef.current = null;
    requestedAudioTrackRef.current =
      activeItemId != null ? { mediaId: activeItemId, key: "" } : null;
    dispatchedAudioTrackRef.current = null;
    seekToAfterReloadRef.current = null;
    resumePlaybackAfterReloadRef.current = false;
    suppressVideoAutoplayOnCanPlayRef.current = false;
    previousVideoSourceUrlRef.current = "";
    setHlsStatusMessage("");
    mediaRecoveryAttemptsRef.current = 0;
    networkRecoveryAttemptsRef.current = 0;
    initialBufferGapHandledRef.current = false;
    videoPlaybackStartedRef.current = false;
    setSubtitleMenuOpen(false);
    setAudioMenuOpen(false);
    setPlayerSettingsOpen(false);
    setIsVideoLoading(isVideo);
    setPendingSubtitleKey(null);
  }, [activeItemDuration, activeItemId, isVideo]);

  useEffect(() => {
    if (!isVideo) return;
    const nextDuration = resolvedVideoDuration(playbackDurationSeconds, activeItemDuration, 0);
    if (nextDuration <= 0) return;
    setPlaybackState((current) =>
      current.duration === nextDuration ? current : { ...current, duration: nextDuration },
    );
  }, [activeItemDuration, isVideo, playbackDurationSeconds]);

  useEffect(() => {
    if (!activeItem) {
      requestedAudioTrackRef.current = null;
      dispatchedAudioTrackRef.current = null;
      manualSubtitleSelectionRef.current = null;
      manualAudioSelectionRef.current = null;
      return;
    }
    if (requestedAudioTrackRef.current?.mediaId !== activeItem.id) {
      requestedAudioTrackRef.current = null;
    }
    if (dispatchedAudioTrackRef.current?.mediaId !== activeItem.id) {
      dispatchedAudioTrackRef.current = null;
    }
  }, [activeItem]);

  useEffect(() => {
    if (!isVideo || !activeItem) return;
    const intervalId = window.setInterval(() => {
      void persistPlaybackProgress();
    }, PLAYBACK_PROGRESS_HEARTBEAT_MS);
    return () => window.clearInterval(intervalId);
  }, [activeItem, isVideo, persistPlaybackProgress]);

  useEffect(() => {
    if (!isVideo || !activeItem) return;
    if (
      defaultTrackSelectionAppliedRef.current === activeItem.id &&
      (manualSubtitleSelectionRef.current === activeItem.id ||
        selectedSubtitleKey !== "off" ||
        !hasSupportedSubtitleTracks)
    ) {
      return;
    }
    if (activeItem.library_id != null && !librariesFetched) return;
    if (
      videoSourceUrl === "" &&
      audioTracks.length === 0 &&
      subtitleTrackOptions.length === 0 &&
      videoAudioIndex < 0
    ) {
      return;
    }
    const queuedSubtitleKey = resolveQueuedSubtitleKey();
    const preferredSubtitleKey =
      queuedSubtitleKey ??
      getPreferredSubtitleKey(
        subtitleTrackOptions,
        effectivePreferredSubtitleLanguage,
        libraryPlaybackPreferences.subtitlesEnabledByDefault && !clientSubtitleAutoPickDisabled,
        effectiveSubtitleLabelHint,
      );
    const preferredAudioKey = getPreferredAudioKey(
      audioTracks,
      effectivePreferredAudioLanguage,
    );
    if (manualSubtitleSelectionRef.current !== activeItem.id) {
      setSelectedSubtitleKey((current) =>
        current === preferredSubtitleKey ? current : preferredSubtitleKey,
      );
    }
    if (manualAudioSelectionRef.current !== activeItem.id) {
      setSelectedAudioKey((current) =>
        current === preferredAudioKey ? current : preferredAudioKey,
      );
    }
    defaultTrackSelectionAppliedRef.current = activeItem.id;
  }, [
    activeItem,
    audioTracks,
    isVideo,
    librariesFetched,
    clientSubtitleAutoPickDisabled,
    effectivePreferredAudioLanguage,
    effectivePreferredSubtitleLanguage,
    effectiveSubtitleLabelHint,
    libraryPlaybackPreferences.subtitlesEnabledByDefault,
    resolveQueuedSubtitleKey,
    hasSupportedSubtitleTracks,
    selectedSubtitleKey,
    subtitleTrackOptions,
    videoAudioIndex,
    videoSourceUrl,
  ]);

  useEffect(
    () => () => {
      void persistPlaybackProgress({ force: true });
    },
    [persistPlaybackProgress],
  );

  useEffect(() => {
    if (!isVideo || !activeItem) return;
    const persist = () => {
      void persistPlaybackProgress({ force: true });
    };
    const onVisibilityChange = () => {
      if (document.visibilityState === "hidden") persist();
    };
    window.addEventListener("pagehide", persist);
    document.addEventListener("visibilitychange", onVisibilityChange);
    return () => {
      window.removeEventListener("pagehide", persist);
      document.removeEventListener("visibilitychange", onVisibilityChange);
    };
  }, [activeItem, isVideo, persistPlaybackProgress]);

  const applyManagedSubtitleTrack = useCallback(() => {
    const video = videoRef.current;
    if (!video) return;
    const hasLoadedSubtitles = loadedSubtitleTracks.length > 0;
    const hasSelectedSubtitle = selectedSubtitleKey !== "off";

    if (!hasSelectedSubtitle && !hasLoadedSubtitles) {
      return;
    }

    const burnIdx = burnEmbeddedSubtitleStreamIndex;
    if (
      burnIdx != null &&
      selectedSubtitleKey === `emb-${burnIdx}`
    ) {
      clearTextTrackCues(manualSubtitleTrackRef.current);
      if (manualSubtitleTrackRef.current) {
        manualSubtitleTrackRef.current.mode = "disabled";
      }
      return;
    }

    // ASS tracks are rendered by JassubRenderer; skip TextTrack for them.
    if (selectedSubtitleKey !== "off") {
      const selectedTrackReq = subtitleTrackRequests.find((t) => t.key === selectedSubtitleKey);
      if (selectedTrackReq?.assEligible) {
        clearTextTrackCues(manualSubtitleTrackRef.current);
        if (manualSubtitleTrackRef.current) {
          manualSubtitleTrackRef.current.mode = "disabled";
        }
        return;
      }
    }

    if (videoSourceIsHls && hlsRef.current && selectedSubtitleKey !== "off") {
      const hlsIdx = findHlsSubtitleTrackIndexForPlumKey(hlsRef.current, selectedSubtitleKey);
      if (hlsIdx >= 0) {
        clearTextTrackCues(manualSubtitleTrackRef.current);
        if (manualSubtitleTrackRef.current) {
          manualSubtitleTrackRef.current.mode = "disabled";
        }
        return;
      }
    }

    let track = manualSubtitleTrackRef.current;
    if (manualSubtitleVideoRef.current !== video || track == null || !hasTextTrack(video, track)) {
      try {
        track = video.addTextTrack("subtitles", "Plum subtitles", "und");
      } catch {
        return;
      }
      if (!track) {
        return;
      }
      manualSubtitleTrackRef.current = track;
      manualSubtitleVideoRef.current = video;
    }

    clearTextTrackCues(track);

    if (selectedSubtitleKey === "off") {
      track.mode = "disabled";
      return;
    }

    const selectedTrack =
      loadedSubtitleTracks.find((candidate) => candidate.key === selectedSubtitleKey) ?? null;
    if (!selectedTrack) {
      track.mode = "disabled";
      return;
    }

    for (const cue of buildSubtitleCues(selectedTrack.body)) {
      applyCueLineSetting(cue, subtitleAppearance.position);
      track.addCue(cue);
    }
    track.mode = "showing";
  }, [
    burnEmbeddedSubtitleStreamIndex,
    loadedSubtitleTracks,
    selectedSubtitleKey,
    subtitleAppearance.position,
    subtitleTrackRequests,
    videoSourceIsHls,
  ]);

  useEffect(() => {
    applyManagedSubtitleTrack();
    return () => {
      clearTextTrackCues(manualSubtitleTrackRef.current);
      if (manualSubtitleTrackRef.current) {
        manualSubtitleTrackRef.current.mode = "disabled";
      }
    };
  }, [applyManagedSubtitleTrack, subtitleAttachmentVersion, subtitleReadyVersion]);

  // Clear JASSUB renderer when the selected subtitle is no longer an ASS track.
  useEffect(() => {
    if (selectedSubtitleKey === "off") {
      setActiveAssSource(null);
      return;
    }
    // When there is no playback track source, `subtitleTrackRequests` is always [] — do not treat a
    // missing row as a stale selection (avoids racing `ensureSubtitleTrackLoaded` on the way up).
    if (playbackTrackSource == null) {
      return;
    }
    const track = subtitleTrackRequests.find((t) => t.key === selectedSubtitleKey);
    if (track == null || !track.assEligible) {
      setActiveAssSource(null);
    }
  }, [playbackTrackSource, selectedSubtitleKey, subtitleTrackRequests]);

  const syncBrowserAudioTrackSelection = useCallback(() => {
    const browserAudioTracks = getBrowserAudioTracks(videoRef.current);
    if (
      browserAudioTracks == null ||
      browserAudioTracks.length <= 1 ||
      browserAudioTracks.length !== audioTracks.length
    ) {
      return;
    }

    const detectedIndex = Array.from(
      { length: browserAudioTracks.length },
      (_, index) => index,
    ).find((index) => browserAudioTracks[index]?.enabled);
    const activeIndex =
      selectedAudioIndex >= 0 ? selectedAudioIndex : Math.max(0, detectedIndex ?? 0);

    for (let i = 0; i < browserAudioTracks.length; i += 1) {
      const audioTrack = browserAudioTracks[i];
      if (!audioTrack) continue;
      audioTrack.enabled = i === activeIndex;
    }
  }, [audioTracks, selectedAudioIndex]);

  const requestAudioTrackChange = useCallback(
    (key: string) => {
      if (!isVideo || !activeItem || !key) return;
      const track = audioTracks.find((candidate) => candidate.key === key);
      if (!track) return;
      const previousRequest = dispatchedAudioTrackRef.current;
      if (previousRequest?.mediaId === activeItem.id && previousRequest.key === key) return;
      dispatchedAudioTrackRef.current = { mediaId: activeItem.id, key };
      void changeAudioTrack(track.streamIndex);
    },
    [activeItem, audioTracks, changeAudioTrack, isVideo],
  );

  useEffect(() => {
    syncBrowserAudioTrackSelection();
  }, [audioTrackVersion, syncBrowserAudioTrackSelection]);

  useEffect(() => {
    if (!selectedAudioKey) return;
    if (!videoSourceUrl) return;
    syncBrowserAudioTrackSelection();
    if (
      requestedAudioTrackRef.current?.mediaId !== activeItem?.id ||
      requestedAudioTrackRef.current?.key !== selectedAudioKey
    ) {
      return;
    }
    requestAudioTrackChange(selectedAudioKey);
  }, [
    activeItem?.id,
    requestAudioTrackChange,
    selectedAudioKey,
    syncBrowserAudioTrackSelection,
    videoSourceUrl,
  ]);

  useEffect(() => {
    if (!isVideo || !activeItem || videoAudioIndex < 0) return;
    const sessionAudioKey =
      audioTracks.find((track) => track.streamIndex === videoAudioIndex)?.key ?? "";
    if (!sessionAudioKey) return;
    setSelectedAudioKey((current) => (current === sessionAudioKey ? current : sessionAudioKey));
    dispatchedAudioTrackRef.current = { mediaId: activeItem.id, key: sessionAudioKey };
    // Session/WebSocket can report a new audio index without reloading the element metadata handler;
    // bump so native audioTracks selection re-syncs (fixes UI showing server track while mux played another).
    setAudioTrackVersion((v) => v + 1);
    if (
      requestedAudioTrackRef.current?.mediaId === activeItem.id &&
      requestedAudioTrackRef.current.key === sessionAudioKey
    ) {
      requestedAudioTrackRef.current = null;
    }
  }, [activeItem, audioTracks, isVideo, videoAudioIndex]);

  const retrySubtitleTrackLoad = useCallback(
    async (key: string) => {
      if (!activeItem) return;
      const metadata = await refreshActivePlaybackTracks();
      if (currentSubtitleMediaIdRef.current !== activeItem.id) {
        return;
      }
      const refreshedTrack = buildSubtitleTrackRequests({
        mediaId: activeItem.id,
        subtitles: metadata?.subtitles ?? playbackTrackSource?.subtitles,
        embeddedSubtitles: metadata?.embeddedSubtitles ?? playbackTrackSource?.embeddedSubtitles,
      }).find((candidate) => candidate.key === key);
      if (!refreshedTrack || refreshedTrack.supported === false) {
        setPendingSubtitleKey(null);
        if (!refreshedTrack) {
          setSubtitleStatusMessage("Subtitle track is no longer available.");
        }
        return;
      }
      blockedSubtitleRetryKeysRef.current.delete(key);
      await ensureSubtitleTrackLoaded(key);
    },
    [activeItem, ensureSubtitleTrackLoaded, playbackTrackSource, refreshActivePlaybackTracks],
  );

  const toggleSubtitleMenu = useCallback(() => {
    setSubtitleMenuOpen((value) => {
      const nextOpen = !value;
      if (
        nextOpen &&
        (!hasSupportedSubtitleTracks || blockedSubtitleRetryKeysRef.current.size > 0)
      ) {
        void refreshActivePlaybackTracks();
      }
      return nextOpen;
    });
    setAudioMenuOpen(false);
    setAspectMenuOpen(false);
    setPlayerSettingsOpen(false);
  }, [hasSupportedSubtitleTracks, refreshActivePlaybackTracks]);

  const selectSubtitleTrack = useCallback(
    async (key: string) => {
      const track = subtitleTrackOptions.find((candidate) => candidate.key === key) ?? null;
      if (key !== "off" && (track == null || track.supported === false)) {
        setSubtitleStatusMessage("This subtitle track is unavailable.");
        return;
      }
      {
        const stored = readStoredPlayerWebDefaults();
        if (key === "off") {
          writeStoredPlayerWebDefaults({
            ...stored,
            defaultSubtitleLanguage: PLAYER_WEB_TRACK_LANGUAGE_NONE,
            defaultSubtitleLabelHint: "",
          });
          mergeShowTrackDefaultsForEpisode(activeItem ?? null, {
            defaultSubtitleLanguage: PLAYER_WEB_TRACK_LANGUAGE_NONE,
            defaultSubtitleLabelHint: "",
          });
        } else if (track) {
          const langRaw = track.srcLang.trim();
          const langNorm =
            (langRaw !== "" ? normalizeLanguagePreference(langRaw) : "") ||
            normalizeLanguagePreference(track.label) ||
            "und";
          writeStoredPlayerWebDefaults({
            ...stored,
            defaultSubtitleLanguage: langNorm,
            defaultSubtitleLabelHint: track.label.trim(),
          });
          mergeShowTrackDefaultsForEpisode(activeItem ?? null, {
            defaultSubtitleLanguage: langNorm,
            defaultSubtitleLabelHint: track.label.trim(),
          });
        }
      }
      rememberQueuedSubtitlePreference(key);
      setSubtitleStatusMessage("");
      manualSubtitleSelectionRef.current = activeItem?.id ?? null;
      if (key === "off") {
        if (burnEmbeddedSubtitleStreamIndex != null) {
          captureSubtitleResumePosition();
          await changeEmbeddedSubtitleBurn(null);
        }
        blockedSubtitleRetryKeysRef.current.clear();
        setSelectedSubtitleKey("off");
        setPendingSubtitleKey(null);
        return;
      }

      if (track?.requiresBurn) {
        const idx = embeddedStreamIndexFromKey(key);
        if (idx == null) return;
        if (
          selectedSubtitleKey === key &&
          burnEmbeddedSubtitleStreamIndex === idx
        ) {
          return;
        }
        captureSubtitleResumePosition();
        blockedSubtitleRetryKeysRef.current.delete(key);
        setLoadedSubtitleTracks((current) =>
          current.filter((candidate) => candidate.key !== key),
        );
        setPendingSubtitleKey(null);
        await changeEmbeddedSubtitleBurn(idx);
        setSelectedSubtitleKey(key);
        return;
      }

      if (burnEmbeddedSubtitleStreamIndex != null) {
        captureSubtitleResumePosition();
        await changeEmbeddedSubtitleBurn(null);
      }

      const shouldRefreshBeforeRetry = blockedSubtitleRetryKeysRef.current.has(key);
      setLoadedSubtitleTracks((current) =>
        current.filter((candidate) => candidate.key !== key),
      );
      setPendingSubtitleKey(key);
      if (shouldRefreshBeforeRetry) {
        if (selectedSubtitleKey !== key) {
          setSelectedSubtitleKey(key);
        }
        void retrySubtitleTrackLoad(key);
        return;
      }
      blockedSubtitleRetryKeysRef.current.delete(key);
      if (selectedSubtitleKey === key) {
        void ensureSubtitleTrackLoaded(key);
        return;
      }
      setSelectedSubtitleKey(key);
    },
    [
      activeItem,
      burnEmbeddedSubtitleStreamIndex,
      captureSubtitleResumePosition,
      changeEmbeddedSubtitleBurn,
      ensureSubtitleTrackLoaded,
      rememberQueuedSubtitlePreference,
      retrySubtitleTrackLoad,
      selectedSubtitleKey,
      subtitleTrackOptions,
    ],
  );

  useEffect(() => {
    if (!isVideo || activeItemId == null || videoSourceUrl === "") return;
    if (selectedSubtitleKey === "off") return;
    const req = subtitleTrackRequests.find((t) => t.key === selectedSubtitleKey);
    if (!req?.requiresBurn) return;
    const idx = embeddedStreamIndexFromKey(selectedSubtitleKey);
    if (idx == null) return;
    if (burnEmbeddedSubtitleStreamIndex === idx) return;
    captureSubtitleResumePosition();
    void changeEmbeddedSubtitleBurn(idx);
  }, [
    activeItemId,
    burnEmbeddedSubtitleStreamIndex,
    captureSubtitleResumePosition,
    changeEmbeddedSubtitleBurn,
    isVideo,
    selectedSubtitleKey,
    subtitleTrackRequests,
    videoSourceUrl,
  ]);

  const selectAudioTrack = useCallback(
    (key: string) => {
      manualAudioSelectionRef.current = activeItem?.id ?? null;
      requestedAudioTrackRef.current =
        activeItem != null ? { mediaId: activeItem.id, key } : null;
      const picked = audioTracks.find((candidate) => candidate.key === key);
      if (picked) {
        const langNorm =
          normalizeLanguagePreference(picked.language) ||
          normalizeLanguagePreference(picked.label) ||
          "";
        const stored = readStoredPlayerWebDefaults();
        writeStoredPlayerWebDefaults({ ...stored, defaultAudioLanguage: langNorm });
        mergeShowTrackDefaultsForEpisode(activeItem ?? null, { defaultAudioLanguage: langNorm });
      }
      setSelectedAudioKey((current) => (current === key ? current : key));
    },
    [activeItem, audioTracks],
  );

  useEffect(() => {
    if (hlsRef.current != null) {
      hlsRef.current.destroy();
      hlsRef.current = null;
    }

    const video = videoRef.current;
    if (!isVideo || !video) return;

    if (!videoSourceUrl) {
      video.removeAttribute("src");
      video.load();
      return;
    }

    if (!videoSourceIsHls || !Hls.isSupported()) {
      video.src = videoSourceUrl;
      return;
    }

    const hls = new Hls({
      enableWorker: true,
      backBufferLength: 90,
      maxBufferLength: 60,
      maxMaxBufferLength: 120,
      startFragPrefetch: true,
      startPosition: seekToAfterReloadRef.current !== null ? seekToAfterReloadRef.current : -1,
      xhrSetup: (xhr) => {
        xhr.withCredentials = true;
      },
    });
    hlsRef.current = hls;
    hls.on(Hls.Events.MANIFEST_PARSED, () => {
      setHlsStatusMessage("");
      mediaRecoveryAttemptsRef.current = 0;
      networkRecoveryAttemptsRef.current = 0;
      markSubtitleReady();
    });
    hls.on(Hls.Events.ERROR, (_event, data: HlsErrorData) => {
      const formattedError = formatHlsErrorMessage(data);
      const isRecoverableGapError =
        !data.fatal &&
        (data.details === "bufferStalledError" || data.details === "bufferSeekOverHole");
      if (!isRecoverableGapError) {
        console.error("[PlaybackDock] HLS error", {
          mediaId: activeItemId,
          source: videoSourceUrl,
          fatal: data.fatal,
          type: data.type,
          details: data.details,
          error: data.error,
        });
      }

      if (!data.fatal) {
        if (data.details === "bufferStalledError") {
          const video = videoRef.current;
          if (maybeRecoverInitialBufferGap(video)) {
            setHlsStatusMessage("Resyncing playback...");
            void video?.play().catch(() => {});
          }
        }
        return;
      }

      if (data.type === Hls.ErrorTypes.NETWORK_ERROR && networkRecoveryAttemptsRef.current < 2) {
        networkRecoveryAttemptsRef.current += 1;
        setHlsStatusMessage("Reconnecting stream...");
        hls.startLoad();
        return;
      }

      if (data.type === Hls.ErrorTypes.MEDIA_ERROR && mediaRecoveryAttemptsRef.current < 2) {
        mediaRecoveryAttemptsRef.current += 1;
        setHlsStatusMessage("Recovering playback...");
        hls.recoverMediaError();
        return;
      }

      setHlsStatusMessage(`Stream error: ${formattedError}`);
    });
    hls.loadSource(videoSourceUrl);
    hls.attachMedia(video);

    return () => {
      hls.destroy();
      if (hlsRef.current === hls) {
        hlsRef.current = null;
      }
    };
  }, [
    activeItemId,
    isVideo,
    markSubtitleReady,
    maybeRecoverInitialBufferGap,
    videoAttachmentVersion,
    videoSourceIsHls,
    videoSourceUrl,
  ]);

  useEffect(() => {
    if (!videoSourceIsHls) return;
    const hls = hlsRef.current;
    if (!hls) return;
    const burnIdx = burnEmbeddedSubtitleStreamIndex;
    if (
      burnIdx != null &&
      selectedSubtitleKey === `emb-${burnIdx}`
    ) {
      hls.subtitleTrack = -1;
      hls.subtitleDisplay = false;
      return;
    }
    if (selectedSubtitleKey === "off") {
      hls.subtitleTrack = -1;
      return;
    }
    const selectedReq = subtitleTrackRequests.find((t) => t.key === selectedSubtitleKey);
    if (selectedReq?.assEligible && selectedReq.assSrc) {
      // ASS is rendered by JASSUB; the same logical track may still appear as HLS WebVTT.
      hls.subtitleTrack = -1;
      hls.subtitleDisplay = false;
      return;
    }
    const idx = findHlsSubtitleTrackIndexForPlumKey(hls, selectedSubtitleKey);
    if (idx >= 0) {
      hls.subtitleTrack = idx;
      hls.subtitleDisplay = true;
    } else {
      hls.subtitleTrack = -1;
    }
  }, [
    burnEmbeddedSubtitleStreamIndex,
    selectedSubtitleKey,
    subtitleReadyVersion,
    subtitleTrackRequests,
    videoAttachmentVersion,
    videoSourceIsHls,
  ]);

  useEffect(() => {
    if (!videoSourceIsHls || selectedSubtitleKey === "off") return;
    const hls = hlsRef.current;
    if (!hls) return;
    const idx = findHlsSubtitleTrackIndexForPlumKey(hls, selectedSubtitleKey);
    if (idx < 0) return;
    const controller = subtitleLoadControllersRef.current.get(selectedSubtitleKey);
    if (controller) {
      controller.abort();
      subtitleLoadControllersRef.current.delete(selectedSubtitleKey);
    }
    setLoadedSubtitleTracks((current) =>
      current.filter((candidate) => candidate.key !== selectedSubtitleKey),
    );
  }, [selectedSubtitleKey, subtitleReadyVersion, videoSourceIsHls]);

  /* ── Close track menus on outside click ── */
  useEffect(() => {
    if (!subtitleMenuOpen && !audioMenuOpen && !aspectMenuOpen && !playerSettingsOpen) return;
    const onClick = (e: MouseEvent) => {
      if (
        subtitleMenuRef.current?.contains(e.target as Node) ||
        subtitleBtnRef.current?.contains(e.target as Node) ||
        audioMenuRef.current?.contains(e.target as Node) ||
        audioBtnRef.current?.contains(e.target as Node) ||
        aspectMenuRef.current?.contains(e.target as Node) ||
        aspectBtnRef.current?.contains(e.target as Node) ||
        playerSettingsMenuRef.current?.contains(e.target as Node) ||
        playerSettingsBtnRef.current?.contains(e.target as Node)
      )
        return;
      setSubtitleMenuOpen(false);
      setAudioMenuOpen(false);
      setAspectMenuOpen(false);
      setPlayerSettingsOpen(false);
    };
    document.addEventListener("pointerdown", onClick);
    return () => document.removeEventListener("pointerdown", onClick);
  }, [aspectMenuOpen, audioMenuOpen, playerSettingsOpen, subtitleMenuOpen]);

  const syncBrowserFullscreenState = useCallback(() => {
    setBrowserFullscreenActive(document.fullscreenElement === playerRootRef.current);
  }, []);

  useEffect(() => {
    syncBrowserFullscreenState();
    const handleFullscreenChange = () => syncBrowserFullscreenState();
    document.addEventListener("fullscreenchange", handleFullscreenChange);
    return () => document.removeEventListener("fullscreenchange", handleFullscreenChange);
  }, [syncBrowserFullscreenState]);

  const toggleBrowserFullscreen = useCallback(async () => {
    if (document.fullscreenElement === playerRootRef.current) {
      await document.exitFullscreen().catch(() => {});
      return;
    }
    if (!playerRootRef.current) return;
    await playerRootRef.current.requestFullscreen?.().catch(() => {});
  }, []);

  /* ── Auto-hide controls in fullscreen ── */
  const resetHideTimer = useCallback(() => {
    setControlsVisible(true);
    clearTimeout(hideTimerRef.current);
    hideTimerRef.current = setTimeout(() => {
      setControlsVisible(false);
    }, CONTROLS_HIDE_DELAY);
  }, []);

  useEffect(() => {
    if (!isWindowPlayer) {
      setControlsVisible(true);
      clearTimeout(hideTimerRef.current);
      return;
    }
    resetHideTimer();
    return () => clearTimeout(hideTimerRef.current);
  }, [isWindowPlayer, resetHideTimer]);

  const handleFullscreenMouseMove = useCallback(() => {
    if (isWindowPlayer) resetHideTimer();
  }, [isWindowPlayer, resetHideTimer]);

  const handleOverlayMouseEnter = useCallback(() => {
    clearTimeout(hideTimerRef.current);
    setControlsVisible(true);
  }, []);

  /* ── Up next: keyboard (theater player) ── */
  useEffect(() => {
    if (!upNextTarget || activeMode !== "video" || activeItem == null) return;
    const onKeyDown = (event: KeyboardEvent) => {
      const tag = (event.target as HTMLElement)?.tagName;
      if (tag === "INPUT" || tag === "SELECT" || tag === "TEXTAREA") return;
      if (event.key === "Escape") {
        event.preventDefault();
        dismissUpNext();
        return;
      }
      if (event.key === "Enter") {
        event.preventDefault();
        confirmUpNextNow();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [upNextTarget, activeMode, activeItem, dismissUpNext, confirmUpNextNow]);

  /* ── Keyboard shortcuts (fullscreen) ── */
  useEffect(() => {
    if (!isWindowPlayer || !isVideo) return;
    const onKeyDown = (event: KeyboardEvent) => {
      /* Ignore when a form element is focused */
      const tag = (event.target as HTMLElement)?.tagName;
      if (tag === "INPUT" || tag === "SELECT" || tag === "TEXTAREA") return;

      switch (event.key) {
        case "Escape":
          if (document.fullscreenElement === playerRootRef.current) {
            void document.exitFullscreen().catch(() => {});
          } else {
            const snapshot = captureVideoProgressSnapshot(videoRef.current);
            void persistPlaybackProgress({ force: true, snapshot });
            dismissDock();
          }
          break;
        case "f":
        case "F":
          event.preventDefault();
          void toggleBrowserFullscreen();
          break;
        case " ":
          event.preventDefault();
          togglePlayPause();
          resetHideTimer();
          break;
        case "ArrowLeft":
          event.preventDefault();
          seekTo(Math.max(0, (videoRef.current?.currentTime ?? 0) - 10));
          resetHideTimer();
          break;
        case "ArrowRight":
          event.preventDefault();
          seekTo((videoRef.current?.currentTime ?? 0) + 10);
          resetHideTimer();
          break;
        case "ArrowUp":
          event.preventDefault();
          setVolume(Math.min(1, volume + 0.1));
          resetHideTimer();
          break;
        case "ArrowDown":
          event.preventDefault();
          setVolume(Math.max(0, volume - 0.1));
          resetHideTimer();
          break;
        case "m":
        case "M":
          setMuted(!muted);
          resetHideTimer();
          break;
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [
    captureVideoProgressSnapshot,
    dismissDock,
    isWindowPlayer,
    isVideo,
    muted,
    persistPlaybackProgress,
    resetHideTimer,
    seekTo,
    setMuted,
    setVolume,
    toggleBrowserFullscreen,
    togglePlayPause,
    volume,
  ]);

  const handleVideoPrevious = useCallback(() => {
    const currentTime = videoRef.current?.currentTime ?? playbackState.currentTime;
    if (currentTime > VIDEO_PREVIOUS_RESTART_THRESHOLD_SECONDS) {
      seekTo(0);
      return;
    }
    playPreviousInQueue();
  }, [playPreviousInQueue, playbackState.currentTime, seekTo]);

  const handleVideoEnded = useCallback(
    (event: SyntheticEvent<HTMLVideoElement>) => {
      const snapshot = captureVideoProgressSnapshot(event.currentTarget);
      void persistPlaybackProgress({ force: true, completed: true, snapshot });
      if (videoAutoplayEnabled && hasNextQueueItem) {
        const next = playbackQueue[queueIndex + 1];
        if (next) {
          setUpNextTarget(next);
        }
        return;
      }
    },
    [
      captureVideoProgressSnapshot,
      hasNextQueueItem,
      persistPlaybackProgress,
      playbackQueue,
      queueIndex,
      videoAutoplayEnabled,
    ],
  );

  const progressMax = useMemo(
    () =>
      playbackState.duration > 0
        ? playbackState.duration
        : Math.max(playbackDurationSeconds, 0),
    [playbackState.duration, playbackDurationSeconds],
  );

  const [seekPreviewSec, setSeekPreviewSec] = useState<number | null>(null);
  /** True while pointer-dragging the seek slider (synchronous; avoids seek spam before pointerup). */
  const seekScrubActiveRef = useRef(false);
  /** Latest slider seconds during scrub; fallback if DOM value is not yet readable. */
  const seekPreviewValueRef = useRef<number | null>(null);

  const finishSeekScrub = useCallback(
    (input: HTMLInputElement | null) => {
      if (!seekScrubActiveRef.current) return;
      seekScrubActiveRef.current = false;
      removeScrubWindowListeners();
      const el = input ?? seekSliderRef.current;
      const parsed = el ? Number(el.value) : NaN;
      const preview = seekPreviewValueRef.current;
      seekPreviewValueRef.current = null;
      setSeekPreviewSec(null);
      const v = Number.isFinite(parsed)
        ? parsed
        : preview != null && Number.isFinite(preview)
          ? preview
          : null;
      if (v != null && Number.isFinite(v)) {
        seekToRef.current(v);
      }
    },
    [removeScrubWindowListeners],
  );

  const handleSeekSliderPointerDown = useCallback(
    (e: PointerEvent<HTMLInputElement>) => {
      try {
        e.currentTarget.setPointerCapture(e.pointerId);
      } catch {
        /* ignore */
      }
      removeScrubWindowListeners();

      seekScrubActiveRef.current = true;
      const pointerId = e.pointerId;

      const onWindowPointerEnd = (ev: Event) => {
        if (!(ev instanceof PointerEvent) || ev.pointerId !== pointerId) return;
        removeScrubWindowListeners();
        queueMicrotask(() => {
          finishSeekScrub(seekSliderRef.current);
        });
      };

      window.addEventListener("pointerup", onWindowPointerEnd);
      window.addEventListener("pointercancel", onWindowPointerEnd);
      scrubWindowListenersRef.current = () => {
        window.removeEventListener("pointerup", onWindowPointerEnd);
        window.removeEventListener("pointercancel", onWindowPointerEnd);
      };
    },
    [finishSeekScrub, removeScrubWindowListeners],
  );

  const handleSeekSliderChange = useCallback(
    (event: ChangeEvent<HTMLInputElement> | { currentTarget: HTMLInputElement }) => {
      const next = Number(event.currentTarget.value);
      if (!Number.isFinite(next)) return;
      seekPreviewValueRef.current = next;
      setSeekPreviewSec(next);
      /* During pointer scrub, commit once on pointer release — rapid seeks confuse MSE/HLS. */
      if (!seekScrubActiveRef.current) {
        seekToRef.current(next);
      }
    },
    [],
  );

  const seekRelativeSeconds = useCallback(
    (delta: number) => {
      removeScrubWindowListeners();
      seekScrubActiveRef.current = false;
      seekPreviewValueRef.current = null;
      setSeekPreviewSec(null);
      const cap =
        progressMax > 0 && Number.isFinite(progressMax) ? progressMax : Number.POSITIVE_INFINITY;
      const el = videoRef.current;
      const t =
        el != null && Number.isFinite(el.currentTime) ? el.currentTime : playbackState.currentTime;
      seekTo(Math.max(0, Math.min(cap, t + delta)));
      resetHideTimer();
    },
    [
      playbackState.currentTime,
      progressMax,
      removeScrubWindowListeners,
      resetHideTimer,
      seekTo,
    ],
  );

  useEffect(() => {
    removeScrubWindowListeners();
    seekScrubActiveRef.current = false;
    seekPreviewValueRef.current = null;
    setSeekPreviewSec(null);
  }, [activeItemId, removeScrubWindowListeners]);

  const seekSliderDisplayValue = Math.min(
    seekPreviewSec !== null ? seekPreviewSec : playbackState.currentTime,
    progressMax || 0,
  );
  const seekTimeLabelSec = seekPreviewSec !== null ? seekPreviewSec : playbackState.currentTime;

  const aspectTrackMenuOptions = useMemo(
    () =>
      videoAspectModeOptions.map((opt) => ({
        key: opt.value,
        label:
          opt.value === "auto" && detectedVideoAspectLabel != null
            ? `Auto (${detectedVideoAspectLabel})`
            : opt.label,
      })),
    [detectedVideoAspectLabel],
  );

  if (!activeItem || !isDockOpen || !activeMode) {
    return null;
  }

  if (activeMode === "music") {
    return (
      <audio
        key={activeItem.id}
        ref={setAudioRef}
        className="playback-dock__audio plum-music-audio"
        src={mediaStreamUrl(BASE_URL, activeItem.id)}
        autoPlay
        onLoadedMetadata={(event) => syncPlaybackState(event.currentTarget)}
        onTimeUpdate={(event) => syncPlaybackState(event.currentTarget)}
        onPlay={(event) => syncPlaybackState(event.currentTarget)}
        onPause={(event) => syncPlaybackState(event.currentTarget)}
        onVolumeChange={(event) => syncPlaybackState(event.currentTarget)}
        onEnded={() => {
          if (repeatMode === "one" && audioRef.current) {
            audioRef.current.currentTime = 0;
            void audioRef.current.play().catch(() => {});
            return;
          }
          playNextInQueue();
        }}
      />
    );
  }

  const muteButtonLabel = muted || volume === 0 ? "Unmute" : "Mute";
  const autoplayButtonLabel = videoAutoplayEnabled ? "Disable autoplay next" : "Enable autoplay next";
  const handleClosePlayer = () => {
    const snapshot = captureVideoProgressSnapshot(videoRef.current);
    void persistPlaybackProgress({ force: true, snapshot });
    dismissDock();
  };

  const upNextSeasonLabel = upNextTarget ? getSeasonEpisodeLabel(upNextTarget) : "";
  const upNextOverlay =
    upNextTarget != null ? (
      <div
        className="playback-up-next"
        role="dialog"
        aria-modal="true"
        aria-label="Up next"
      >
        {upNextBackdropUrl ? (
          <img src={upNextBackdropUrl} alt="" className="playback-up-next__bg" />
        ) : (
          <div className="playback-up-next__bg playback-up-next__bg--empty" aria-hidden />
        )}
        <div className="playback-up-next__scrim" />
        <div className="playback-up-next__content">
          <p className="playback-up-next__eyebrow">Up next</p>
          <h2 className="playback-up-next__title">
            {upNextTarget.title}
          </h2>
          {upNextSeasonLabel ? (
            <p className="playback-up-next__meta">{upNextSeasonLabel}</p>
          ) : null}
          <p className="playback-up-next__timer">
            Starting in{" "}
            <span className="playback-up-next__timer-value">{upNextSecondsLeft}</span>s
          </p>
          <div className="playback-up-next__actions">
            <button type="button" className="playback-up-next__play-now" onClick={confirmUpNextNow}>
              Play now
            </button>
            <button type="button" className="playback-up-next__cancel" onClick={dismissUpNext}>
              Cancel
            </button>
          </div>
        </div>
      </div>
    ) : null;

  const resumePromptOverlay =
    resumePrompt != null ? (
      <div
        className="playback-resume-prompt"
        role="dialog"
        aria-modal="true"
        aria-label="Resume playback"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="playback-resume-prompt__scrim" />
        <div className="playback-resume-prompt__content">
          <p className="playback-resume-prompt__eyebrow">Resume playback</p>
          <div className="playback-resume-prompt__actions">
            <button
              type="button"
              className="playback-resume-prompt__resume"
              onClick={handleResumeFromProgress}
            >
              Resume from {formatClock(resumePrompt.seconds)}
            </button>
            <button
              type="button"
              className="playback-resume-prompt__restart"
              onClick={handleStartFromBeginning}
            >
              Start from beginning
            </button>
          </div>
        </div>
      </div>
    ) : null;

  /* ── Full-viewport in-app video (then use Fullscreen API for OS-level full screen) ── */
  if (isWindowPlayer) {
    const seasonEpisode = getSeasonEpisodeLabel(activeItem);
    const titleDisplay = seasonEpisode
      ? `${seasonEpisode} · ${activeItem.title}`
      : activeItem.title;

    return (
      <section
        ref={(node) => {
          playerRootRef.current = node;
        }}
        className={`fullscreen-player fullscreen-player--aspect-${videoAspectMode}${
          controlsVisible ? "" : " fullscreen-player--hidden"
        }`}
        aria-label="Video player"
        aria-busy={showPlayerLoadingOverlay}
        role="button"
        tabIndex={0}
        onMouseMove={handleFullscreenMouseMove}
        onClick={(event) => {
          /* Toggle play/pause on click (but not on controls) */
          if (
            event.target === event.currentTarget ||
            (event.target as HTMLElement).tagName === "VIDEO"
          ) {
            togglePlayPause();
            resetHideTimer();
          }
        }}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            togglePlayPause();
            resetHideTimer();
          }
        }}
      >
        <div className="fullscreen-player__video-stage">
          <div className="fullscreen-player__video-frame">
            <video
              key={activeItem.id}
              ref={setVideoRef}
              className="fullscreen-player__video"
              style={videoSubtitleStyle}
              crossOrigin="use-credentials"
              autoPlay
              playsInline
              onLoadStart={() => {
                if (!videoPlaybackStartedRef.current) {
                  setIsVideoLoading(true);
                }
              }}
              onLoadedMetadata={(event) => handleVideoLoadedMetadata(event.currentTarget)}
              onCanPlay={(event) => handleVideoCanPlay(event.currentTarget)}
              onTimeUpdate={(event) => {
                if (event.currentTarget.currentTime > 1) {
                  initialBufferGapHandledRef.current = true;
                }
                syncPlaybackState(event.currentTarget);
                syncVideoProgressSnapshot(event.currentTarget);
                persistInitialPlaybackProgress(event.currentTarget);
                processIntroSkip(event.currentTarget);
              }}
              onPlay={(event) => {
                if (event.currentTarget.currentTime > 1) {
                  initialBufferGapHandledRef.current = true;
                }
                setIsVideoLoading(false);
                setHlsStatusMessage("");
                syncPlaybackState(event.currentTarget);
                syncVideoProgressSnapshot(event.currentTarget);
                persistInitialPlaybackProgress(event.currentTarget);
                processIntroSkip(event.currentTarget);
              }}
              onPlaying={() => {
                kickstartVideoPlaybackRef.current = false;
                videoPlaybackStartedRef.current = true;
                setIsVideoLoading(false);
              }}
              onPause={(event) => {
                syncPlaybackState(event.currentTarget);
                const snapshot = captureVideoProgressSnapshot(event.currentTarget);
                void persistPlaybackProgress({ force: true, snapshot });
              }}
              onWaiting={(event) => {
                if (
                  !event.currentTarget.ended &&
                  !videoPlaybackStartedRef.current
                ) {
                  setIsVideoLoading(true);
                }
              }}
              onSeeked={(event) => {
                syncPlaybackState(event.currentTarget);
                syncVideoProgressSnapshot(event.currentTarget);
              }}
              onVolumeChange={(event) => syncPlaybackState(event.currentTarget)}
              onError={() => {
                setIsVideoLoading(false);
                setHlsStatusMessage("Stream error: browser media element failed to load playback");
              }}
              onEnded={(event) => {
                setIsVideoLoading(false);
                handleVideoEnded(event);
              }}
            ></video>
          </div>
          <JassubRenderer videoElement={jassubVideoElement} assSrc={activeAssSource} />
        </div>
        {showSkipIntroControl && (
          <button
            type="button"
            className="fullscreen-player__skip-intro"
            onClick={(event) => {
              event.stopPropagation();
              handleSkipIntroClick();
            }}
          >
            Skip intro
          </button>
        )}
        {showPlayerLoadingOverlay && (
          <PlayerLoadingOverlay label={playerLoadingLabel} fullscreen />
        )}

        <div className="fullscreen-player__top-bar">
          <div className="fullscreen-player__top-bar-lead" aria-hidden="true" />
          <div className="fullscreen-player__title-area">
            <h2 className="fullscreen-player__title">{titleDisplay}</h2>
            <div className="fullscreen-player__status">
              {videoStatusMessage && (
                <>
                  <span className="status-dot" data-connected={wsConnected} />
                  <span>{videoStatusMessage}</span>
                </>
              )}
            </div>
          </div>
          <div className="fullscreen-player__top-bar-tail">
            <div className="fullscreen-player__top-bar-actions">
              <button
                type="button"
                className={`fullscreen-player__close-btn${browserFullscreenActive ? " is-active" : ""}`}
                onClick={() => {
                  void toggleBrowserFullscreen();
                }}
                aria-label={
                  browserFullscreenActive ? "Exit full screen" : "Full screen on this display"
                }
                title={
                  browserFullscreenActive
                    ? "Exit full screen"
                    : "Full screen on this display (hides browser UI)"
                }
              >
                {browserFullscreenActive ? (
                  <Minimize2 className="size-5" strokeWidth={2.25} />
                ) : (
                  <Maximize2 className="size-5" strokeWidth={2.25} />
                )}
              </button>
              <button
                type="button"
                className="fullscreen-player__close-btn"
                onClick={(event) => {
                  event.stopPropagation();
                  handleClosePlayer();
                }}
                aria-label="Close player"
                title="Close player"
              >
                <X className="size-5" strokeWidth={2.25} />
              </button>
            </div>
          </div>
        </div>

        {/* Bottom controls overlay */}
        <div
          ref={overlayRef}
          className="fullscreen-player__controls"
          onMouseEnter={handleOverlayMouseEnter}
        >
          {/* Seek bar full-width */}
          <div className="fullscreen-player__seek">
            <input
              ref={seekSliderRef}
              type="range"
              className="fullscreen-player__seek-slider"
              aria-label="Seek playback"
              min={0}
              max={progressMax || 0}
              step={0.1}
              value={seekSliderDisplayValue}
              onPointerDown={handleSeekSliderPointerDown}
              onChange={handleSeekSliderChange}
              onInput={handleSeekSliderChange}
            />
          </div>

          <div className="fullscreen-player__controls-row">
            {/* Left: play + time */}
            <div className="fullscreen-player__controls-left">
              <button
                type="button"
                className="fullscreen-player__ctrl-btn"
                onClick={togglePlayPause}
                aria-label={playbackState.isPlaying ? "Pause playback" : "Play playback"}
                title={playbackState.isPlaying ? "Pause" : "Play"}
              >
                {playbackState.isPlaying ? (
                  <Pause className="size-[1.125rem]" strokeWidth={2.25} />
                ) : (
                  <Play className="size-[1.125rem]" strokeWidth={2.25} />
                )}
              </button>
              <button
                type="button"
                className="fullscreen-player__ctrl-btn"
                onClick={() => seekRelativeSeconds(-VIDEO_SKIP_BUTTON_SECONDS)}
                aria-label={`Seek back ${VIDEO_SKIP_BUTTON_SECONDS} seconds`}
                title={`Back ${VIDEO_SKIP_BUTTON_SECONDS}s`}
              >
                <Rewind className="size-[1.125rem]" strokeWidth={2.25} />
              </button>
              <button
                type="button"
                className="fullscreen-player__ctrl-btn"
                onClick={() => seekRelativeSeconds(VIDEO_SKIP_BUTTON_SECONDS)}
                aria-label={`Seek forward ${VIDEO_SKIP_BUTTON_SECONDS} seconds`}
                title={`Forward ${VIDEO_SKIP_BUTTON_SECONDS}s`}
              >
                <FastForward className="size-[1.125rem]" strokeWidth={2.25} />
              </button>
              <span className="fullscreen-player__time">
                {formatClock(seekTimeLabelSec)} / {formatClock(progressMax)}
              </span>
            </div>

            {/* Right: subtitles + settings + volume + fullscreen + exit */}
            <div className="fullscreen-player__controls-right">
              {isVideo && (
                <div className="fullscreen-player__aspect-wrap">
                  <button
                    ref={aspectBtnRef}
                    type="button"
                    className={`fullscreen-player__ctrl-btn${videoAspectMode !== "auto" ? " is-active" : ""}`}
                    aria-label="Aspect ratio"
                    title="Aspect ratio"
                    onClick={() => {
                      setAspectMenuOpen((value) => !value);
                      setSubtitleMenuOpen(false);
                      setAudioMenuOpen(false);
                      setPlayerSettingsOpen(false);
                      resetHideTimer();
                    }}
                  >
                    <Ratio className="size-[1.125rem]" strokeWidth={2.25} />
                  </button>
                  {aspectMenuOpen && (
                    <TrackMenu
                      menuRef={aspectMenuRef}
                      options={aspectTrackMenuOptions}
                      selectedKey={videoAspectMode}
                      ariaLabel="Select aspect ratio"
                      onSelect={(key) => {
                        setVideoAspectMode(key as VideoAspectMode);
                        setAspectMenuOpen(false);
                        resetHideTimer();
                      }}
                    />
                  )}
                </div>
              )}

              {isVideo && (
                <div className="fullscreen-player__subtitle-wrap">
                  <button
                    ref={subtitleBtnRef}
                    type="button"
                    className={`fullscreen-player__ctrl-btn${selectedSubtitleKey !== "off" ? " is-active" : ""}`}
                    aria-label="Subtitles"
                    title="Subtitles"
                    onClick={toggleSubtitleMenu}
                  >
                    <Subtitles className="size-[1.125rem]" strokeWidth={2.25} />
                  </button>
                  {subtitleMenuOpen && (
                    <TrackMenu
                      menuRef={subtitleMenuRef}
                      options={subtitleMenuTrackOptions}
                      selectedKey={selectedSubtitleKey}
                      ariaLabel="Select subtitle track"
                      offLabel="Off"
                      onSelect={(key) => {
                        void selectSubtitleTrack(key);
                        setSubtitleMenuOpen(false);
                      }}
                    />
                  )}
                </div>
              )}

              {audioTracks.length > 1 && (
                <div className="fullscreen-player__audio-wrap">
                  <button
                    ref={audioBtnRef}
                    type="button"
                    className="fullscreen-player__ctrl-btn"
                    aria-label={`Audio track: ${selectedAudioLabel}`}
                    title={`Audio: ${selectedAudioLabel}`}
                    onClick={() => {
                      setAudioMenuOpen((value) => !value);
                      setSubtitleMenuOpen(false);
                      setAspectMenuOpen(false);
                      setPlayerSettingsOpen(false);
                    }}
                  >
                    <Volume2 className="size-[1.125rem]" strokeWidth={2.25} />
                  </button>
                  {audioMenuOpen && (
                    <TrackMenu
                      menuRef={audioMenuRef}
                      options={audioTracks}
                      selectedKey={selectedAudioKey}
                      ariaLabel="Select audio track"
                      onSelect={(key) => {
                        selectAudioTrack(key);
                        setAudioMenuOpen(false);
                      }}
                    />
                  )}
                </div>
              )}

              {isVideo && (
                <div className="fullscreen-player__settings-wrap">
                  <button
                    ref={playerSettingsBtnRef}
                    type="button"
                    className="fullscreen-player__ctrl-btn"
                    aria-label="Player settings"
                    title="Player settings"
                    onClick={() => {
                      setPlayerSettingsOpen((value) => !value);
                      setSubtitleMenuOpen(false);
                      setAudioMenuOpen(false);
                      setAspectMenuOpen(false);
                    }}
                  >
                    <Settings className="size-[1.125rem]" strokeWidth={2.25} />
                  </button>
                  {playerSettingsOpen && (
                    <PlayerSettingsMenu
                      menuRef={playerSettingsMenuRef}
                      preferences={subtitleAppearance}
                      videoAutoplayEnabled={videoAutoplayEnabled}
                      onChange={setSubtitleAppearance}
                      onVideoAutoplayChange={setVideoAutoplayEnabled}
                    />
                  )}
                </div>
              )}

              {hasVideoQueueNavigation && (
                <>
                  <button
                    type="button"
                    className="fullscreen-player__ctrl-btn"
                    onClick={handleVideoPrevious}
                    aria-label="Previous episode"
                    title="Previous episode"
                  >
                    <SkipBack className="size-[1.125rem]" strokeWidth={2.25} />
                  </button>

                  <button
                    type="button"
                    className="fullscreen-player__ctrl-btn"
                    onClick={playNextInQueue}
                    aria-label="Next episode"
                    title="Next episode"
                    disabled={!hasNextQueueItem}
                  >
                    <SkipForward className="size-[1.125rem]" strokeWidth={2.25} />
                  </button>

                  <button
                    type="button"
                    className={`fullscreen-player__ctrl-btn${videoAutoplayEnabled ? " is-active" : ""}`}
                    onClick={() => setVideoAutoplayEnabled((value) => !value)}
                    aria-label="Autoplay next episode"
                    title={autoplayButtonLabel}
                    aria-pressed={videoAutoplayEnabled}
                  >
                    <Repeat className="size-[1.125rem]" strokeWidth={2.25} />
                  </button>
                </>
              )}

              <div className="fullscreen-player__volume-group">
                <button
                  type="button"
                  className="fullscreen-player__ctrl-btn"
                  onClick={() => setMuted(!muted)}
                  aria-label={muteButtonLabel}
                  title={muteButtonLabel}
                >
                  {muted || volume === 0 ? (
                    <VolumeX className="size-[1.125rem]" strokeWidth={2.25} />
                  ) : (
                    <Volume2 className="size-[1.125rem]" strokeWidth={2.25} />
                  )}
                </button>
                <input
                  type="range"
                  className="fullscreen-player__volume-slider"
                  aria-label="Set volume"
                  min={0}
                  max={1}
                  step={0.01}
                  value={muted ? 0 : volume}
                  onChange={(event) => setVolume(Number(event.target.value))}
                />
              </div>
            </div>
          </div>
        </div>
        {resumePromptOverlay}
        {upNextOverlay}
      </section>
    );
  }

  return null;
}
