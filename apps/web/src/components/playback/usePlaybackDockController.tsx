import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type ReactNode,
  type SyntheticEvent,
} from "react";
import { Ratio, Volume2, VolumeX } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import type Hls from "hls.js";
import { PLAYBACK_PROGRESS_HEARTBEAT_MS } from "@plum/contracts";
import { mediaStreamUrl } from "@plum/shared";
import {
  BASE_URL,
  refreshPlaybackTracks,
  updateMediaProgress,
  type MediaItem,
  type PlaybackTrackMetadata,
} from "../../api";
import {
  usePlayerPlaybackPreferences,
  usePlayerQueue,
  usePlayerSession,
  usePlayerTransport,
} from "../../contexts/PlayerContext";
import {
  isEnglishSubtitleTrackForMenu,
  languageMatchesPreference,
  normalizeLanguagePreference,
  PLAYER_WEB_TRACK_LANGUAGE_NONE,
  mergeShowTrackDefaultsForEpisode,
  readStoredPlayerWebDefaults,
  resolveEffectiveWebTrackDefaults,
  resolveLibraryPlaybackPreferences,
  subtitleFontSizeValue,
  formatDetectedVideoAspectLabel,
  videoAspectModeOptions,
  writeStoredPlayerWebDefaults,
  type VideoAspectMode,
} from "../../lib/playbackPreferences";
import {
  buildSubtitleTrackRequests,
  clonePlaybackTrackMetadata,
  embeddedStreamIndexFromKey,
  type PlaybackTrackSource,
} from "../../lib/playback/playbackDockSubtitleTracks";
import {
  applyCueLineSetting,
  bufferedRangeStartsNearZero,
  buildSubtitleCues,
  clearTextTrackCues,
  formatClock,
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
} from "../../lib/playback/playerMedia";
import type {
  AudioTrackOption,
  SubtitleTrackOption,
} from "../../lib/playback/playerMedia";
import {
  useFullscreenPlaybackKeyboard,
  usePlaybackUpNextKeyboard,
  type FullscreenPlaybackKeyboardDeps,
} from "@/hooks/usePlaybackDockKeyboard";
import { ignorePromise } from "@/lib/ignorePromise";
import {
  hasNextQueueItem as computeHasNextQueueItem,
  musicPlaybackShouldLoopSameTrack,
  resolveUpNextItemOnVideoEnd,
  shouldRestartCurrentVideoOnPrevious,
} from "../../lib/playback/playbackDockQueuePlayback";
import { queryKeys } from "../../queries";
import { VIDEO_PREVIOUS_RESTART_THRESHOLD_SECONDS } from "./constants";
import { PlaybackControls } from "./PlaybackControls";
import { PlaybackDockShell } from "./PlaybackDockShell";
import { PlaybackInfoPanel } from "./PlaybackInfoPanel";
import { PlaybackTimeline } from "./PlaybackTimeline";
import { PlaybackTrackMenus } from "./PlaybackTrackMenus";
import { TrackMenu } from "./TrackMenu";
import { PlaybackVideoStage } from "./PlaybackVideoStage";
import { PlayerLoadingOverlay } from "./PlayerLoadingOverlay";
import { useHlsAttachment } from "./useHlsAttachment";
import { usePlaybackDockPlayerLocalSettings } from "./usePlaybackDockPlayerLocalSettings";
import { usePlaybackDockSeekControls } from "./usePlaybackDockSeekControls";
import { usePlaybackDockUpNext } from "./usePlaybackDockUpNext";
import { usePlaybackDockWindowChrome } from "./usePlaybackDockWindowChrome";
import { useSubtitleController } from "./useSubtitleController";
import { useSubtitleTransport, type LoadedSubtitleTrack } from "./useSubtitleTransport";

const EMPTY_PLAYBACK_QUEUE: MediaItem[] = [];

type PlaybackState = {
  currentTime: number;
  duration: number;
  isPlaying: boolean;
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

export function usePlaybackDockController(): ReactNode {
  const queryClient = useQueryClient();
  const { api: playbackPreferences, librariesFetched } =
    usePlayerPlaybackPreferences();
  const {
    playerLocalSettings,
    subtitleAppearance,
    setSubtitleAppearance,
    videoAutoplayEnabled,
    setVideoAutoplayEnabled,
    videoAspectMode,
    setVideoAspectMode,
  } = usePlaybackDockPlayerLocalSettings();
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const audioRef = useRef<HTMLAudioElement | null>(null);
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
  const [selectedSubtitleKey, setSelectedSubtitleKey] = useState("off");
  /** Mirrored from the video ref so JassubRenderer re-renders when the element mounts (ref alone does not). */
  const [jassubVideoElement, setJassubVideoElement] =
    useState<HTMLVideoElement | null>(null);
  const [loadedSubtitleTracks, setLoadedSubtitleTracks] = useState<
    LoadedSubtitleTrack[]
  >([]);
  const [refreshedPlaybackTracks, setRefreshedPlaybackTracks] =
    useState<PlaybackTrackMetadata | null>(null);
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
  const [detectedVideoAspectLabel, setDetectedVideoAspectLabel] = useState<
    string | null
  >(null);
  const [resumePrompt, setResumePrompt] = useState<{
    seconds: number;
    mediaId: number;
  } | null>(null);
  const [isVideoLoading, setIsVideoLoading] = useState(false);
  const [pendingSubtitleKey, setPendingSubtitleKey] = useState<string | null>(
    null,
  );
  const subtitleMenuRef = useRef<HTMLDivElement | null>(null);
  const subtitleBtnRef = useRef<HTMLButtonElement | null>(null);
  const audioMenuRef = useRef<HTMLDivElement | null>(null);
  const audioBtnRef = useRef<HTMLButtonElement | null>(null);
  const aspectMenuRef = useRef<HTMLDivElement | null>(null);
  const aspectBtnRef = useRef<HTMLButtonElement | null>(null);
  const playerSettingsMenuRef = useRef<HTMLDivElement | null>(null);
  const playerSettingsBtnRef = useRef<HTMLButtonElement | null>(null);
  const hlsRef = useRef<Hls | null>(null);
  const requestedAudioTrackRef = useRef<{
    mediaId: number;
    key: string;
  } | null>(null);
  const dispatchedAudioTrackRef = useRef<{
    mediaId: number;
    key: string;
  } | null>(null);
  const handleAssStatusChange = useCallback(
    (status: "loading" | "ready" | "error" | "timeout") => {
      switch (status) {
        case "loading":
          setSubtitleStatusMessage("Loading subtitles...");
          break;
        case "ready":
          setSubtitleStatusMessage("");
          break;
        case "timeout":
          setSubtitleStatusMessage("Subtitle load timed out. Try again.");
          break;
        default:
          setSubtitleStatusMessage("Subtitle load failed. Try again.");
          break;
      }
    },
    [],
  );
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
  const subtitleLoadControllersRef = useRef<Map<string, AbortController>>(
    new Map(),
  );
  const blockedSubtitleRetryKeysRef = useRef<Set<string>>(new Set());
  const currentSubtitleMediaIdRef = useRef<number | null>(null);
  const lastVideoProgressRef = useRef<VideoProgressSnapshot | null>(null);
  const queuedSubtitlePreferenceRef = useRef<QueuedSubtitlePreference | null>(
    null,
  );
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
  const {
    queue,
    queueIndex,
    repeatMode,
    playNextInQueue,
    playPreviousInQueue,
  } = usePlayerQueue();
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
  const {
    setPlayerRootNode,
    playerRootRef,
    controlsVisible,
    resetHideTimer,
    browserFullscreenActive,
    toggleBrowserFullscreen,
    handleVideoDoubleClick,
    handleFullscreenMouseMove,
    handleOverlayMouseEnter,
  } = usePlaybackDockWindowChrome(activeMode === "video" && activeItem != null);
  const captureSubtitleResumePosition = useCallback(() => {
    const v = videoRef.current;
    if (v && Number.isFinite(v.currentTime) && v.currentTime > 0) {
      seekToAfterReloadRef.current = v.currentTime;
      resumePlaybackAfterReloadRef.current = !v.paused && !v.ended;
    }
  }, []);

  // Attempt autoplay with a muted fallback when the browser's autoplay policy blocks
  // unmuted play (NotAllowedError). Calling `setMuted(true)` keeps the UI in sync.
  const attemptAutoplay = useCallback(
    (element: HTMLVideoElement, label: string) => {
      void element.play().catch((err: unknown) => {
        const name =
          err instanceof Error ||
          (typeof DOMException !== "undefined" && err instanceof DOMException)
            ? err.name
            : "";
        if (name === "NotAllowedError") {
          // Browser blocked unmuted autoplay — retry muted so the video at least starts.
          element.muted = true;
          setMuted(true);
          void element.play().catch((err2: unknown) => {
            if (import.meta.env.DEV) {
              console.warn(`[${label}] Muted autoplay also blocked`, err2);
            }
          });
        } else if (name !== "AbortError" && name !== "InvalidStateError") {
          if (import.meta.env.DEV) {
            console.warn(`[${label}]`, err);
          }
        }
      });
    },
    [setMuted],
  );

  const playbackQueue = queue ?? EMPTY_PLAYBACK_QUEUE;
  const {
    upNextTarget,
    setUpNextTarget,
    upNextSecondsLeft,
    dismissUpNext,
    confirmUpNextNow,
    upNextBackdropUrl,
  } = usePlaybackDockUpNext({
    playNextInQueue,
    queueIndex,
    activeMode,
    isDockOpen,
  });
  const playNextInQueueRef = useRef(playNextInQueue);
  const registerMediaElementRef = useRef(registerMediaElement);

  useEffect(() => {
    playNextInQueueRef.current = playNextInQueue;
  }, [playNextInQueue]);

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
      ignorePromise(element.play(), "PlaybackDock:resumeFromProgressPlay");
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
      ignorePromise(element.play(), "PlaybackDock:startFromBeginningPlay");
    }
    resumeAppliedRef.current = prompt.mediaId;
    setResumePrompt(null);
  }, [resumePrompt]);

  useEffect(() => {
    if (activeMode !== "video" || !isDockOpen) {
      setResumePrompt(null);
    }
  }, [activeMode, isDockOpen]);

  const isVideo = activeMode === "video" && activeItem != null;
  const isWindowPlayer = isVideo;
  const activeItemId = activeItem?.id ?? null;
  const activeItemDuration = activeItem?.duration ?? 0;
  const hasNextQueueItem = computeHasNextQueueItem(
    playbackQueue.length,
    queueIndex,
  );
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
      (hlsStatusMessage !== "" &&
        !hlsStatusMessage.startsWith("Stream error:")));
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
  const libraryPlaybackPreferences = useMemo(
    () =>
      activeItem
        ? playbackPreferences.libraryPrefsForItem(activeItem)
        : resolveLibraryPlaybackPreferences(null),
    [activeItem, playbackPreferences],
  );

  const effectiveWebTrackDefaults = useMemo(
    () =>
      resolveEffectiveWebTrackDefaults(
        activeItem ?? null,
        playerLocalSettings.webDefaults,
      ),
    [activeItem, playerLocalSettings.webDefaults],
  );

  const clientSubtitleAutoPickDisabled = useMemo(() => {
    return (
      effectiveWebTrackDefaults.defaultSubtitleLanguage.trim() ===
      PLAYER_WEB_TRACK_LANGUAGE_NONE
    );
  }, [effectiveWebTrackDefaults.defaultSubtitleLanguage]);

  const clientAudioAutoPickDisabled = useMemo(() => {
    return (
      effectiveWebTrackDefaults.defaultAudioLanguage.trim() ===
      PLAYER_WEB_TRACK_LANGUAGE_NONE
    );
  }, [effectiveWebTrackDefaults.defaultAudioLanguage]);

  const effectivePreferredSubtitleLanguage = useMemo(() => {
    if (clientSubtitleAutoPickDisabled) {
      return "";
    }
    const fromClient = effectiveWebTrackDefaults.defaultSubtitleLanguage.trim();
    if (fromClient !== "") {
      return normalizeLanguagePreference(
        effectiveWebTrackDefaults.defaultSubtitleLanguage,
      );
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
      return normalizeLanguagePreference(
        effectiveWebTrackDefaults.defaultAudioLanguage,
      );
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
      ignorePromise(video.play(), "PlaybackDock:autoplayAfterStreamReady");
    }, 0);
    return () => window.clearTimeout(handle);
  }, [activeItemId, isVideo, videoSourceUrl]);

  const playbackTrackSource = useMemo<PlaybackTrackSource | null>(() => {
    if (!isVideo || !activeItem) return null;
    return {
      mediaId: activeItem.id,
      subtitles: refreshedPlaybackTracks?.subtitles ?? activeItem.subtitles,
      embeddedSubtitles:
        refreshedPlaybackTracks?.embeddedSubtitles ??
        activeItem.embeddedSubtitles,
      embeddedAudioTracks:
        refreshedPlaybackTracks?.embeddedAudioTracks ??
        activeItem.embeddedAudioTracks,
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
    const selected = subtitleTrackRequests.find(
      (t) => t.key === selectedSubtitleKey,
    );
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
      const track = subtitleTrackRequests.find(
        (candidate) => candidate.key === key,
      );
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
  const { ensureSubtitleTrackLoaded, subtitleLoadStateByKey } =
    useSubtitleTransport({
    activeMediaId: activeItem?.id ?? null,
    loadedSubtitleTracks,
    subtitleTrackRequests,
    subtitleLoadControllersRef,
    blockedSubtitleRetryKeysRef,
    setPendingSubtitleKey,
    setLoadedSubtitleTracks,
    setSubtitleStatusMessage,
  });
  const subtitleSelection = useSubtitleController({
    selectedSubtitleKey,
    subtitleTrackRequests,
    subtitleLoadStateByKey,
    burnEmbeddedSubtitleStreamIndex,
    videoSourceIsHls,
    hls: hlsRef.current,
    resolutionVersion: subtitleReadyVersion,
  });
  const subtitleRenderer = subtitleSelection.renderer;
  const activeAssSource = subtitleSelection.activeAssSource;
  const manualSubtitleTrackKey = subtitleSelection.manualTrackKey;

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
    if (manualSubtitleTrackKey == null) {
      setPendingSubtitleKey(null);
      if (subtitleRenderer !== "manual_vtt") {
        setSubtitleStatusMessage("");
      }
      return;
    }
    if (
      !loadedSubtitleTracks.some((track) => track.key === manualSubtitleTrackKey)
    ) {
      setPendingSubtitleKey((current) =>
        current === manualSubtitleTrackKey ? current : manualSubtitleTrackKey,
      );
    }
    void ensureSubtitleTrackLoaded(manualSubtitleTrackKey);
  }, [
    ensureSubtitleTrackLoaded,
    loadedSubtitleTracks,
    manualSubtitleTrackKey,
    subtitleRenderer,
    subtitleReadyVersion,
  ]);

  useEffect(() => {
    if (manualSubtitleTrackKey == null) {
      setPendingSubtitleKey(null);
      return;
    }
    if (
      loadedSubtitleTracks.some((track) => track.key === manualSubtitleTrackKey)
    ) {
      setPendingSubtitleKey((current) =>
        current === manualSubtitleTrackKey ? null : current,
      );
    }
  }, [loadedSubtitleTracks, manualSubtitleTrackKey, subtitleRenderer]);

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
        label: formatTrackLabel(
          track.title,
          track.language,
          `Audio ${index + 1}`,
        ),
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
    (selectedAudioIndex >= 0
      ? audioTracks[selectedAudioIndex]?.label
      : audioTracks[0]?.label) || "Audio";
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
        Number.isFinite(element.duration) && element.duration > 0
          ? element.duration
          : 0;
      setPlaybackState({
        currentTime: Number.isFinite(element.currentTime)
          ? element.currentTime
          : 0,
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

  const maybeRecoverInitialBufferGap = useCallback(
    (video: HTMLVideoElement | null): boolean => {
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
    },
    [],
  );

  const captureVideoProgressSnapshot = useCallback(
    (element?: HTMLVideoElement | null): VideoProgressSnapshot | null => {
      if (!isVideo || !activeItem) return null;
      const candidate = element ?? videoRef.current;
      const fallback = lastVideoProgressRef.current;
      const fallbackDuration =
        fallback?.mediaId === activeItem.id ? fallback.durationSeconds : 0;
      const fallbackPosition =
        fallback?.mediaId === activeItem.id
          ? fallback.positionSeconds
          : playbackState.currentTime;
      const duration = resolvedVideoDuration(
        playbackDurationSeconds,
        activeItem.duration,
        candidate?.duration ?? fallbackDuration,
      );
      if (!Number.isFinite(duration) || duration <= 0) return null;
      const rawPosition =
        candidate && Number.isFinite(candidate.currentTime)
          ? candidate.currentTime
          : fallbackPosition;
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
      const ended =
        candidate?.ended ??
        (fallback?.mediaId === activeItem.id ? fallback.ended : false);
      return {
        mediaId: activeItem.id,
        positionSeconds,
        durationSeconds: duration,
        shouldResumePlayback:
          candidate != null
            ? !candidate.paused && !candidate.ended
            : fallback?.mediaId === activeItem.id
              ? fallback.shouldResumePlayback
              : false,
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
    async (options?: {
      force?: boolean;
      completed?: boolean;
      snapshot?: VideoProgressSnapshot | null;
    }) => {
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
      try {
        await updateMediaProgress(activeItem.id, {
          position_seconds: snapshot.positionSeconds,
          duration_seconds: snapshot.durationSeconds,
          completed,
        });
        if (activeItem.library_id != null) {
          void queryClient.invalidateQueries({
            queryKey: queryKeys.library(activeItem.library_id),
          });
        }
        void queryClient.invalidateQueries({ queryKey: queryKeys.home });
      } catch (err) {
        if (import.meta.env.DEV) {
          console.warn("[PlaybackDock:updateMediaProgress]", err);
        }
      }
      lastPersistedRef.current = {
        mediaId: activeItem.id,
        positionSeconds: snapshot.positionSeconds,
        completed,
      };
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
      if (!Number.isFinite(element.currentTime) || element.currentTime <= 0)
        return;
      initialProgressPersistedRef.current = activeItem.id;
      const snapshot = captureVideoProgressSnapshot(element);
      void persistPlaybackProgress({ force: true, snapshot });
    },
    [
      activeItem,
      captureVideoProgressSnapshot,
      isVideo,
      persistPlaybackProgress,
    ],
  );

  const setVideoRef = useCallback((element: HTMLVideoElement | null) => {
    if (videoRef.current !== element) {
      if (videoRef.current && !element) {
        const snapshot = captureVideoProgressSnapshotRef.current(
          videoRef.current,
        );
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
          ignorePromise(
            element.play(),
            "PlaybackDock:resumePlaybackAfterReload",
          );
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
          attemptAutoplay(element, "PlaybackDock:loadedMetadataAutoplay");
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
      attemptAutoplay,
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
      // Do not call markSubtitleReady() here: `canplay` can fire repeatedly during HLS buffering,
      // which re-ran subtitle effects, briefly saw no HLS subtitle tracks, and set subtitleTrack -1.
      setIsVideoLoading(false);
      if (
        !suppressVideoAutoplayOnCanPlayRef.current &&
        kickstartVideoPlaybackRef.current
      ) {
        attemptAutoplay(element, "PlaybackDock:canPlayKickstart");
      }
    },
    [
      attemptAutoplay,
      maybeRecoverInitialBufferGap,
      syncPlaybackState,
      syncVideoProgressSnapshot,
    ],
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
    const nextDuration = resolvedVideoDuration(
      playbackDurationSeconds,
      activeItemDuration,
      0,
    );
    if (nextDuration <= 0) return;
    setPlaybackState((current) =>
      current.duration === nextDuration
        ? current
        : { ...current, duration: nextDuration },
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
        libraryPlaybackPreferences.subtitlesEnabledByDefault &&
          !clientSubtitleAutoPickDisabled,
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

    if (subtitleRenderer !== "manual_vtt") {
      clearTextTrackCues(manualSubtitleTrackRef.current);
      if (manualSubtitleTrackRef.current) {
        manualSubtitleTrackRef.current.mode = "disabled";
      }
      return;
    }

    let track = manualSubtitleTrackRef.current;
    if (
      manualSubtitleVideoRef.current !== video ||
      track == null ||
      !hasTextTrack(video, track)
    ) {
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

    const selectedTrack =
      loadedSubtitleTracks.find(
        (candidate) => candidate.key === manualSubtitleTrackKey,
      ) ?? null;
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
    loadedSubtitleTracks,
    manualSubtitleTrackKey,
    selectedSubtitleKey,
    subtitleAppearance.position,
    subtitleRenderer,
  ]);

  useEffect(() => {
    applyManagedSubtitleTrack();
    return () => {
      clearTextTrackCues(manualSubtitleTrackRef.current);
      if (manualSubtitleTrackRef.current) {
        manualSubtitleTrackRef.current.mode = "disabled";
      }
    };
  }, [
    applyManagedSubtitleTrack,
    subtitleAttachmentVersion,
    subtitleReadyVersion,
  ]);
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
      selectedAudioIndex >= 0
        ? selectedAudioIndex
        : Math.max(0, detectedIndex ?? 0);

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
      if (
        previousRequest?.mediaId === activeItem.id &&
        previousRequest.key === key
      )
        return;
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
      audioTracks.find((track) => track.streamIndex === videoAudioIndex)?.key ??
      "";
    if (!sessionAudioKey) return;
    setSelectedAudioKey((current) =>
      current === sessionAudioKey ? current : sessionAudioKey,
    );
    dispatchedAudioTrackRef.current = {
      mediaId: activeItem.id,
      key: sessionAudioKey,
    };
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
        embeddedSubtitles:
          metadata?.embeddedSubtitles ?? playbackTrackSource?.embeddedSubtitles,
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
    [
      activeItem,
      ensureSubtitleTrackLoaded,
      playbackTrackSource,
      refreshActivePlaybackTracks,
    ],
  );

  const toggleSubtitleMenu = useCallback(() => {
    setSubtitleMenuOpen((value) => {
      const nextOpen = !value;
      if (
        nextOpen &&
        (!hasSupportedSubtitleTracks ||
          blockedSubtitleRetryKeysRef.current.size > 0)
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
      const track =
        subtitleTrackOptions.find((candidate) => candidate.key === key) ?? null;
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

      const shouldRefreshBeforeRetry =
        blockedSubtitleRetryKeysRef.current.has(key);
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
    const req = subtitleTrackRequests.find(
      (t) => t.key === selectedSubtitleKey,
    );
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
        writeStoredPlayerWebDefaults({
          ...stored,
          defaultAudioLanguage: langNorm,
        });
        mergeShowTrackDefaultsForEpisode(activeItem ?? null, {
          defaultAudioLanguage: langNorm,
        });
      }
      setSelectedAudioKey((current) => (current === key ? current : key));
    },
    [activeItem, audioTracks],
  );

  // Called when HLS.js has parsed the manifest — the earliest HLS-ready moment to
  // trigger autoplay (standard HLS.js pattern). Uses the same guards as handleVideoCanPlay
  // plus a resume-progress check to avoid calling play() before the resume prompt appears.
  const handleHlsManifestParsed = useCallback(() => {
    if (suppressVideoAutoplayOnCanPlayRef.current) return;
    if (!kickstartVideoPlaybackRef.current) return;
    if (seekToAfterReloadRef.current != null) return; // source reload — loadedmetadata handles it
    const resumeAt = activeItem?.progress_seconds ?? 0;
    const hasResumableProgress =
      activeItem != null &&
      !activeItem.completed &&
      Number.isFinite(resumeAt) &&
      resumeAt > 0 &&
      resumeAppliedRef.current !== activeItem.id;
    if (hasResumableProgress) return;
    const video = videoRef.current;
    if (!video) return;
    attemptAutoplay(video, "PlaybackDock:manifestParsedAutoplay");
  }, [activeItem, attemptAutoplay]);

  useHlsAttachment({
    isVideo,
    activeItemId,
    videoSourceUrl,
    videoSourceIsHls,
    videoAttachmentVersion,
    videoRef,
    hlsRef,
    seekToAfterReloadRef,
    setHlsStatusMessage,
    markSubtitleReady,
    maybeRecoverInitialBufferGap,
    mediaRecoveryAttemptsRef,
    networkRecoveryAttemptsRef,
    selectedSubtitleKey,
    subtitleRenderer,
    subtitleTrackRequests,
    subtitleReadyVersion,
    subtitleLoadControllersRef,
    setLoadedSubtitleTracks,
    onManifestParsed: handleHlsManifestParsed,
  });

  /* ── Close track menus on outside click ── */
  useEffect(() => {
    if (
      !subtitleMenuOpen &&
      !audioMenuOpen &&
      !aspectMenuOpen &&
      !playerSettingsOpen
    )
      return;
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

  usePlaybackUpNextKeyboard(
    Boolean(upNextTarget && activeMode === "video" && activeItem != null),
    {
      dismissUpNext,
      confirmUpNextNow,
    },
  );

  useFullscreenPlaybackKeyboard(isWindowPlayer && isVideo, {
    playerRootRef,
    videoRef,
    volume,
    muted,
    dismissDock,
    toggleBrowserFullscreen,
    togglePlayPause,
    seekTo,
    setVolume,
    setMuted,
    resetHideTimer,
    captureVideoProgressSnapshot,
    persistPlaybackProgress:
      persistPlaybackProgress as FullscreenPlaybackKeyboardDeps["persistPlaybackProgress"],
  });

  const handleVideoPrevious = useCallback(() => {
    const currentTime =
      videoRef.current?.currentTime ?? playbackState.currentTime;
    if (
      shouldRestartCurrentVideoOnPrevious(
        currentTime,
        VIDEO_PREVIOUS_RESTART_THRESHOLD_SECONDS,
      )
    ) {
      seekTo(0);
      return;
    }
    playPreviousInQueue();
  }, [playPreviousInQueue, playbackState.currentTime, seekTo]);

  const handleVideoEnded = useCallback(
    (event: SyntheticEvent<HTMLVideoElement>) => {
      const snapshot = captureVideoProgressSnapshot(event.currentTarget);
      void persistPlaybackProgress({ force: true, completed: true, snapshot });
      const next = resolveUpNextItemOnVideoEnd({
        videoAutoplayEnabled,
        queue: playbackQueue,
        queueIndex,
      });
      if (next) setUpNextTarget(next);
    },
    [
      captureVideoProgressSnapshot,
      persistPlaybackProgress,
      playbackQueue,
      queueIndex,
      setUpNextTarget,
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

  const {
    seekSliderRef,
    seekSliderDisplayValue,
    seekTimeLabelSec,
    handleSeekSliderPointerDown,
    handleSeekSliderChange,
    seekRelativeSeconds,
  } = usePlaybackDockSeekControls({
    activeItemId,
    playbackCurrentTime: playbackState.currentTime,
    progressMax,
    seekTo,
    videoRef,
    resetHideTimer,
  });

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
          if (
            musicPlaybackShouldLoopSameTrack(repeatMode) &&
            audioRef.current
          ) {
            audioRef.current.currentTime = 0;
            ignorePromise(
              audioRef.current.play(),
              "PlaybackDock:musicRepeatOne",
            );
            return;
          }
          playNextInQueue();
        }}
      />
    );
  }

  const muteButtonLabel = muted || volume === 0 ? "Unmute" : "Mute";
  const autoplayButtonLabel = videoAutoplayEnabled
    ? "Disable autoplay next"
    : "Enable autoplay next";
  const handleClosePlayer = () => {
    const snapshot = captureVideoProgressSnapshot(videoRef.current);
    void persistPlaybackProgress({ force: true, snapshot });
    dismissDock();
  };

  const upNextSeasonLabel = upNextTarget
    ? getSeasonEpisodeLabel(upNextTarget)
    : "";
  const upNextOverlay =
    upNextTarget != null ? (
      <div
        className="playback-up-next"
        role="dialog"
        aria-modal="true"
        aria-label="Up next"
      >
        {upNextBackdropUrl ? (
          <img
            src={upNextBackdropUrl}
            alt=""
            className="playback-up-next__bg"
          />
        ) : (
          <div
            className="playback-up-next__bg playback-up-next__bg--empty"
            aria-hidden
          />
        )}
        <div className="playback-up-next__scrim" />
        <div className="playback-up-next__content">
          <p className="playback-up-next__eyebrow">Up next</p>
          <h2 className="playback-up-next__title">{upNextTarget.title}</h2>
          {upNextSeasonLabel ? (
            <p className="playback-up-next__meta">{upNextSeasonLabel}</p>
          ) : null}
          <p className="playback-up-next__timer">
            Starting in{" "}
            <span className="playback-up-next__timer-value">
              {upNextSecondsLeft}
            </span>
            s
          </p>
          <div className="playback-up-next__actions">
            <button
              type="button"
              className="playback-up-next__play-now"
              onClick={confirmUpNextNow}
            >
              Play now
            </button>
            <button
              type="button"
              className="playback-up-next__cancel"
              onClick={dismissUpNext}
            >
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
      <PlaybackDockShell
        playerRootRef={setPlayerRootNode}
        controlsVisible={controlsVisible}
        videoAspectMode={videoAspectMode}
        showPlayerLoadingOverlay={showPlayerLoadingOverlay}
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
        <PlaybackVideoStage
          mediaItemId={activeItem.id}
          setVideoRef={setVideoRef}
          videoSubtitleStyle={videoSubtitleStyle}
          jassubVideoElement={jassubVideoElement}
          activeAssSource={activeAssSource}
          onAssStatusChange={handleAssStatusChange}
          onVideoDoubleClick={handleVideoDoubleClick}
          onLoadStart={() => {
            if (!videoPlaybackStartedRef.current) {
              setIsVideoLoading(true);
            }
          }}
          onLoadedMetadata={handleVideoLoadedMetadata}
          onCanPlay={handleVideoCanPlay}
          onTimeUpdate={(event) => {
            if (event.currentTarget.currentTime > 1) {
              initialBufferGapHandledRef.current = true;
            }
            syncPlaybackState(event.currentTarget);
            syncVideoProgressSnapshot(event.currentTarget);
            persistInitialPlaybackProgress(event.currentTarget);
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
            setHlsStatusMessage(
              "Stream error: browser media element failed to load playback",
            );
          }}
          onEnded={(event) => {
            setIsVideoLoading(false);
            handleVideoEnded(event);
          }}
        />
        {showPlayerLoadingOverlay && (
          <PlayerLoadingOverlay label={playerLoadingLabel} fullscreen />
        )}

        <PlaybackInfoPanel
          titleDisplay={titleDisplay}
          videoStatusMessage={videoStatusMessage}
          wsConnected={wsConnected}
          browserFullscreenActive={browserFullscreenActive}
          onToggleBrowserFullscreen={toggleBrowserFullscreen}
          onClosePlayer={handleClosePlayer}
        />

        <div
          ref={overlayRef}
          className="fullscreen-player__controls"
          onMouseEnter={handleOverlayMouseEnter}
        >
          <PlaybackTimeline
            progressMax={progressMax}
            seekTimeLabelSec={seekTimeLabelSec}
            seekSliderRef={seekSliderRef}
            seekSliderDisplayValue={seekSliderDisplayValue}
            onSeekPointerDown={handleSeekSliderPointerDown}
            onSeekChange={handleSeekSliderChange}
          />
          <div className="fullscreen-player__controls-row">
            {/* LEFT: Aspect ratio + volume */}
            <div className="fullscreen-player__controls-left">
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

            {/* CENTER: Transport controls */}
            <PlaybackControls
              isPlaying={playbackState.isPlaying}
              onTogglePlayPause={togglePlayPause}
              onSeekRelative={seekRelativeSeconds}
              hasVideoQueueNavigation={hasVideoQueueNavigation}
              onVideoPrevious={handleVideoPrevious}
              onPlayNextInQueue={playNextInQueue}
              hasNextQueueItem={hasNextQueueItem}
              onToggleQueueAutoplay={() =>
                setVideoAutoplayEnabled((value) => !value)
              }
              autoplayNextLabel={autoplayButtonLabel}
              queueAutoplayActive={videoAutoplayEnabled}
            />

            {/* RIGHT: Audio, subtitles, settings */}
            <PlaybackTrackMenus
              showSubtitleControls={isVideo}
              subtitleBtnRef={subtitleBtnRef}
              subtitleMenuRef={subtitleMenuRef}
              subtitleMenuOpen={subtitleMenuOpen}
              onSubtitleButtonClick={toggleSubtitleMenu}
              subtitleMenuTrackOptions={subtitleMenuTrackOptions}
              selectedSubtitleKey={selectedSubtitleKey}
              onSelectSubtitleTrack={(key) => {
                void selectSubtitleTrack(key);
                setSubtitleMenuOpen(false);
              }}
              audioBtnRef={audioBtnRef}
              audioMenuRef={audioMenuRef}
              audioMenuOpen={audioMenuOpen}
              onAudioButtonClick={() => {
                setAudioMenuOpen((value) => !value);
                setSubtitleMenuOpen(false);
                setAspectMenuOpen(false);
                setPlayerSettingsOpen(false);
              }}
              audioTracks={audioTracks}
              selectedAudioKey={selectedAudioKey}
              selectedAudioLabel={selectedAudioLabel}
              onSelectAudioTrack={(key) => {
                selectAudioTrack(key);
                setAudioMenuOpen(false);
              }}
              showSettingsControls={isVideo}
              playerSettingsBtnRef={playerSettingsBtnRef}
              playerSettingsMenuRef={playerSettingsMenuRef}
              playerSettingsOpen={playerSettingsOpen}
              onPlayerSettingsButtonClick={() => {
                setPlayerSettingsOpen((value) => !value);
                setSubtitleMenuOpen(false);
                setAudioMenuOpen(false);
                setAspectMenuOpen(false);
              }}
              subtitleAppearance={subtitleAppearance}
              onSubtitleAppearanceChange={setSubtitleAppearance}
              videoAutoplayEnabled={videoAutoplayEnabled}
              onVideoAutoplayEnabledChange={setVideoAutoplayEnabled}
            />
          </div>
        </div>
        {resumePromptOverlay}
        {upNextOverlay}
      </PlaybackDockShell>
    );
  }

  return null;
}
