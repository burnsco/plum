import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { buildBackendUrl } from "@plum/shared";
import { embeddedSubtitleNeedsWebBurnIn } from "@plum/shared";
import {
  BASE_URL,
  PLAYBACK_STREAM_BASE_URL,
  closePlaybackSession,
  createPlaybackSession,
  warmEmbeddedSubtitleCaches,
  getShowEpisodes,
  type CreatePlaybackSessionPayload,
  type MediaItem,
  type PlaybackSession as ApiPlaybackSession,
  type PlumWebSocketCommand,
  updatePlaybackSessionAudio,
} from "../api";
import { resolveEffectivePreferredAudioLanguage, resolveLibraryPlaybackPreferences } from "../lib/playbackPreferences";
import {
  clampVolume,
  indexOfQueueItem,
  preferredInitialAudioIndex,
  shuffleQueue,
} from "../lib/playback/playerQueue";
import {
  clampVideoSeekSeconds,
  detectClientPlaybackCapabilities,
  formatTrackLabel,
  getPreferredSubtitleKey,
  type SubtitleTrackOption,
} from "../lib/playback/playerMedia";
import { ignorePromise, ignorePromiseAlwaysLogUnexpected } from "../lib/ignorePromise";
import { sortMusicTracks } from "../lib/musicGrouping";
import { getShowKey, sortEpisodes } from "../lib/showGrouping";
import { useLibraries } from "../queries";
import { useWs } from "./WsContext";
import {
  normalizeLanguagePreference,
  PLAYER_WEB_TRACK_LANGUAGE_NONE,
  readStoredPlayerWebDefaults,
  resolveEffectiveWebTrackDefaults,
} from "../lib/playbackPreferences";

export type PlaybackKind = "video" | "music";
/** Video is always `window` (theater: fixed overlay filling the viewport). Display fullscreen uses the Fullscreen API separately. Music ignores layout and uses the in-page bar on the music library view. */
export type PlayerViewMode = "window";
export type MusicRepeatMode = "off" | "all" | "one";
export type MediaElementSlot = "audio" | "video";

export type PlaybackSession = {
  activeMode: PlaybackKind;
  isDockOpen: boolean;
  viewMode: PlayerViewMode;
  queue: MediaItem[];
  queueIndex: number;
  shuffle: boolean;
  repeatMode: MusicRepeatMode;
};

type VideoSessionState = {
  delivery: ApiPlaybackSession["delivery"];
  sessionId: string | null;
  mediaId: number;
  desiredRevision: number;
  currentRevision: number;
  audioIndex: number;
  status: "starting" | "ready" | "error" | "closed";
  streamUrl: string;
  durationSeconds: number;
  error: string;
  /** Server-side PGS burn-in; null when subtitles are not burned into the video. */
  burnEmbeddedSubtitleStreamIndex: number | null;
};

type PlaybackSessionSource =
  | {
      delivery: "direct";
      mediaId: number;
      audioIndex?: number;
      status: ApiPlaybackSession["status"];
      streamUrl: string;
      durationSeconds: number;
      error?: string;
      burnEmbeddedSubtitleStreamIndex?: number;
    }
  | {
      delivery: "remux" | "transcode";
      sessionId: string;
      mediaId: number;
      revision: number;
      audioIndex: number;
      status: ApiPlaybackSession["status"];
      streamUrl: string;
      durationSeconds: number;
      error?: string;
      burnEmbeddedSubtitleStreamIndex?: number;
    };

function mergePlaybackTracks(
  item: MediaItem,
  session: ApiPlaybackSession,
): MediaItem {
  return {
    ...item,
    subtitles: session.subtitles?.map((subtitle) => ({ ...subtitle })) ?? item.subtitles,
    embeddedSubtitles:
      session.embeddedSubtitles?.map((subtitle) => ({ ...subtitle })) ?? item.embeddedSubtitles,
    embeddedAudioTracks:
      session.embeddedAudioTracks?.map((track) => ({ ...track })) ?? item.embeddedAudioTracks,
    intro_start_seconds: item.intro_start_seconds ?? session.intro_start_seconds,
    intro_end_seconds: item.intro_end_seconds ?? session.intro_end_seconds,
    credits_start_seconds: item.credits_start_seconds ?? session.credits_start_seconds,
    credits_end_seconds: item.credits_end_seconds ?? session.credits_end_seconds,
  };
}

function resolvePlaybackStreamUrl(streamUrl: string): string {
  const base = PLAYBACK_STREAM_BASE_URL || BASE_URL;
  return buildBackendUrl(base, streamUrl);
}

function toVideoSessionState(session: PlaybackSessionSource): VideoSessionState {
  const burn =
    session.burnEmbeddedSubtitleStreamIndex !== undefined
      ? session.burnEmbeddedSubtitleStreamIndex
      : null;
  if (session.delivery === "direct") {
    return {
      delivery: session.delivery,
      sessionId: null,
      mediaId: session.mediaId,
      desiredRevision: 0,
      currentRevision: 0,
      audioIndex: session.audioIndex ?? -1,
      status: session.status,
      streamUrl: resolvePlaybackStreamUrl(session.streamUrl),
      durationSeconds: session.durationSeconds,
      error: session.error ?? "",
      burnEmbeddedSubtitleStreamIndex: burn,
    };
  }

  return {
    delivery: session.delivery,
    sessionId: session.sessionId,
    mediaId: session.mediaId,
    desiredRevision: session.revision,
    currentRevision: session.status === "ready" ? session.revision : 0,
    audioIndex: session.audioIndex,
    status: session.status,
    streamUrl: resolvePlaybackStreamUrl(session.streamUrl),
    durationSeconds: session.durationSeconds,
    error: session.error ?? "",
    burnEmbeddedSubtitleStreamIndex: burn,
  };
}

function playbackStatusMessage(status: VideoSessionState["status"]): string {
  return status === "ready" ? "Stream ready" : "Preparing stream...";
}

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

function resolveInitialBurnSubtitleStreamIndex(
  item: MediaItem,
  libraryPrefs: ReturnType<typeof resolveLibraryPlaybackPreferences>,
): number | null {
  const effectiveDefaults = resolveEffectiveWebTrackDefaults(
    item,
    readStoredPlayerWebDefaults(),
  );
  const subtitlesDisabledByClient =
    effectiveDefaults.defaultSubtitleLanguage.trim() === PLAYER_WEB_TRACK_LANGUAGE_NONE;
  const subtitlesEnabled =
    !subtitlesDisabledByClient && libraryPrefs.subtitlesEnabledByDefault;
  if (!subtitlesEnabled) return null;

  const preferredSubtitleLanguageRaw =
    effectiveDefaults.defaultSubtitleLanguage.trim() !== ""
      ? effectiveDefaults.defaultSubtitleLanguage
      : libraryPrefs.preferredSubtitleLanguage;
  const preferredSubtitleLanguage =
    normalizeLanguagePreference(preferredSubtitleLanguageRaw);
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

/** Track / stream / dock chrome — updates on item switch, WS, duration, etc. */
export type PlayerSessionContextValue = {
  playbackSession: PlaybackSession | null;
  activeItem: MediaItem | null;
  activeMode: PlaybackKind | null;
  isDockOpen: boolean;
  viewMode: PlayerViewMode;
  videoSourceUrl: string;
  playbackDurationSeconds: number;
  /** Active video session delivery; null when not playing video. */
  videoDelivery: ApiPlaybackSession["delivery"] | null;
  videoAudioIndex: number;
  /** Active burned-in embedded subtitle stream, or null. */
  burnEmbeddedSubtitleStreamIndex: number | null;
  dismissDock: () => void;
  wsConnected: boolean;
  lastEvent: string;
};

/** Queue shape and navigation — updates when queue index or list changes. */
export type PlayerQueueContextValue = {
  queue: MediaItem[];
  queueIndex: number;
  shuffle: boolean;
  repeatMode: MusicRepeatMode;
  playMedia: (item: MediaItem) => void;
  playMovie: (item: MediaItem) => void;
  playEpisode: (item: MediaItem, options?: { showKey?: string }) => void;
  playShowGroup: (items: MediaItem[], startItem?: MediaItem) => void;
  playMusicCollection: (items: MediaItem[], startItem?: MediaItem) => void;
  playNextInQueue: () => void;
  playPreviousInQueue: () => void;
  toggleShuffle: () => void;
  cycleRepeatMode: () => void;
};

/** High-frequency controls — volume, seek, play/pause; avoids queue/session churn. */
export type PlayerTransportContextValue = {
  volume: number;
  muted: boolean;
  togglePlayPause: () => void;
  seekTo: (seconds: number) => void;
  setMuted: (muted: boolean) => void;
  setVolume: (volume: number) => void;
  enterFullscreen: () => void;
  exitFullscreen: () => void;
  registerMediaElement: (
    slot: MediaElementSlot,
    element: HTMLMediaElement | null,
  ) => void;
  changeAudioTrack: (audioIndex: number) => Promise<void>;
  changeEmbeddedSubtitleBurn: (streamIndex: number | null) => Promise<void>;
};

export type PlayerContextValue = PlayerSessionContextValue &
  PlayerQueueContextValue &
  PlayerTransportContextValue;

const PlayerSessionContext = createContext<PlayerSessionContextValue | null>(
  null,
);
const PlayerQueueContext = createContext<PlayerQueueContextValue | null>(null);
const PlayerTransportContext = createContext<PlayerTransportContextValue | null>(
  null,
);

export function PlayerProvider({ children }: { children: ReactNode }) {
  const { data: libraries = [] } = useLibraries();
  const [playbackSession, setPlaybackSession] =
    useState<PlaybackSession | null>(null);
  const [videoSession, setVideoSession] = useState<VideoSessionState | null>(
    null,
  );
  const [musicBaseQueue, setMusicBaseQueue] = useState<MediaItem[]>([]);
  const [volume, setVolumeState] = useState(1);
  const [muted, setMutedState] = useState(false);
  const [lastEvent, setLastEvent] = useState("");
  const mountedRef = useRef(true);
  const volumeRef = useRef(1);
  const mutedRef = useRef(false);
  const activeVideoItemIdRef = useRef<number | null>(null);
  const videoSessionRef = useRef<VideoSessionState | null>(null);
  const playbackSessionRef = useRef<PlaybackSession | null>(null);
  const mediaElementsRef = useRef<
    Record<MediaElementSlot, HTMLMediaElement | null>
  >({
    audio: null,
    video: null,
  });
  const { wsConnected, latestEvent, eventSequence, sendCommand } = useWs();

  const activeItem = playbackSession?.queue[playbackSession.queueIndex] ?? null;
  const activeMode = playbackSession?.activeMode ?? null;
  const isDockOpen = playbackSession?.isDockOpen ?? false;
  const viewMode: PlayerViewMode = playbackSession?.viewMode ?? "window";
  const queue = useMemo(() => playbackSession?.queue ?? [], [playbackSession]);
  const queueIndex = playbackSession?.queueIndex ?? 0;
  const shuffle = playbackSession?.shuffle ?? false;
  const repeatMode = playbackSession?.repeatMode ?? "off";

  activeVideoItemIdRef.current =
    activeMode === "video" ? (activeItem?.id ?? null) : null;
  videoSessionRef.current = videoSession;
  playbackSessionRef.current = playbackSession;

  // Only expose the stream URL once the server reports "ready" (all initial segments on disk).
  // During "starting" we keep the *previous* ready URL so HLS.js continues playing the old
  // revision instead of loading a partial manifest and stalling at the transcode live-edge —
  // matching Android TV / ExoPlayer behavior.
  const lastReadyVideoUrlRef = useRef("");
  const lastReadyVideoItemRef = useRef<number | null>(null);
  const prevVideoSessionIdRef = useRef<string | null>(null);

  const activeItemId_ = activeItem?.id ?? null;
  if (activeItemId_ !== lastReadyVideoItemRef.current) {
    lastReadyVideoUrlRef.current = "";
    lastReadyVideoItemRef.current = activeItemId_;
  }

  const sessionId = videoSession?.sessionId ?? null;
  if (sessionId !== prevVideoSessionIdRef.current) {
    // A new playback session replaces the in-memory old one (e.g. subtitle burn closes the
    // prior session). The cached "ready" URL refers to the old session and must not be reused.
    lastReadyVideoUrlRef.current = "";
    prevVideoSessionIdRef.current = sessionId;
  }

  const readyUrl =
    activeMode === "video" &&
    videoSession &&
    videoSession.streamUrl &&
    videoSession.status === "ready"
      ? videoSession.streamUrl
      : "";
  if (readyUrl) {
    lastReadyVideoUrlRef.current = readyUrl;
  }

  const videoSourceUrl = readyUrl || lastReadyVideoUrlRef.current;
  const playbackDurationSeconds =
    activeMode === "video"
      ? (videoSession?.durationSeconds && videoSession.durationSeconds > 0
          ? videoSession.durationSeconds
          : Math.max(activeItem?.duration ?? 0, 0))
      : 0;
  const videoDelivery =
    activeMode === "video" && videoSession != null ? videoSession.delivery : null;
  const videoAudioIndex = activeMode === "video" ? (videoSession?.audioIndex ?? -1) : -1;
  const burnEmbeddedSubtitleStreamIndex =
    activeMode === "video"
      ? (videoSession?.burnEmbeddedSubtitleStreamIndex ?? null)
      : null;

  const sendPlaybackCommand = useCallback(
    (command: PlumWebSocketCommand) => {
      sendCommand(command);
    },
    [sendCommand],
  );

  const closeVideoSession = useCallback(
    (sessionId?: string | null) => {
      if (!sessionId) return;
      sendPlaybackCommand({ action: "detach_playback_session", sessionId });
      ignorePromiseAlwaysLogUnexpected(closePlaybackSession(sessionId), "Player:closePlaybackSession");
    },
    [sendPlaybackCommand],
  );

  const applyPlaybackSession = useCallback((session: PlaybackSessionSource) => {
    const nextSession = toVideoSessionState(session);
    setVideoSession(nextSession);
    setPlaybackSession((current) => {
      if (current == null || current.activeMode !== "video") {
        return current;
      }
      const activeQueueItem = current.queue[current.queueIndex];
      if (!activeQueueItem || activeQueueItem.id !== session.mediaId) {
        return current;
      }
      const nextQueue = [...current.queue];
      nextQueue[current.queueIndex] = mergePlaybackTracks(activeQueueItem, session);
      return {
        ...current,
        queue: nextQueue,
      };
    });
    setLastEvent(playbackStatusMessage(nextSession.status));
  }, []);

  const createClientPlaybackSession = useCallback(
    (
      item: MediaItem,
      audioIndex: number,
      options?: { burnEmbeddedSubtitleStreamIndex?: number },
    ) => {
      const payload: CreatePlaybackSessionPayload = {
        audioIndex,
        clientCapabilities: detectClientPlaybackCapabilities(),
      };
      if (options?.burnEmbeddedSubtitleStreamIndex != null) {
        payload.burnEmbeddedSubtitleStreamIndex =
          options.burnEmbeddedSubtitleStreamIndex;
      }
      return createPlaybackSession(item.id, payload);
    },
    [],
  );

  useEffect(() => {
    if (!wsConnected) return;
    const sessionId = videoSession?.sessionId;
    if (!sessionId) return;
    sendPlaybackCommand({ action: "attach_playback_session", sessionId });
  }, [sendPlaybackCommand, videoSession?.sessionId, wsConnected]);

  const pauseAllMediaElements = useCallback(() => {
    mediaElementsRef.current.audio?.pause();
    mediaElementsRef.current.video?.pause();
  }, []);

  const exitBrowserFullscreen = useCallback(() => {
    if (!document.fullscreenElement) return;
    ignorePromise(document.exitFullscreen(), "Player:exitBrowserFullscreen");
  }, []);

  const registerMediaElement = useCallback(
    (slot: MediaElementSlot, element: HTMLMediaElement | null) => {
      mediaElementsRef.current[slot] = element;
      if (!element) return;
      element.volume = volumeRef.current;
      element.muted = mutedRef.current;
    },
    [],
  );

  useEffect(() => {
    volumeRef.current = volume;
    mutedRef.current = muted;
    for (const element of Object.values(mediaElementsRef.current)) {
      if (!element) continue;
      element.volume = volume;
      element.muted = muted;
    }
  }, [muted, volume]);

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const getActiveMediaElement = useCallback(() => {
    if (activeMode === "music") return mediaElementsRef.current.audio;
    if (activeMode === "video") return mediaElementsRef.current.video;
    return null;
  }, [activeMode]);

  const dismissDock = useCallback(() => {
    if (playbackSession?.activeMode === "video") {
      closeVideoSession(videoSessionRef.current?.sessionId);
    }
    pauseAllMediaElements();
    exitBrowserFullscreen();
    setPlaybackSession(null);
    setVideoSession(null);
    setMusicBaseQueue([]);
    setLastEvent("");
  }, [
    closeVideoSession,
    exitBrowserFullscreen,
    pauseAllMediaElements,
    playbackSession?.activeMode,
  ]);

  const playVideoQueue = useCallback(
    (items: MediaItem[], startIndex = 0) => {
      if (items.length === 0) return;
      pauseAllMediaElements();
      const clampedIndex = Math.max(0, Math.min(startIndex, items.length - 1));
      const nextItem = items[clampedIndex] ?? items[0];
      setPlaybackSession((current) => ({
        activeMode: "video",
        isDockOpen: true,
        viewMode: "window",
        queue: items,
        queueIndex: clampedIndex,
        shuffle: false,
        repeatMode: current?.repeatMode ?? "off",
      }));
      closeVideoSession(videoSessionRef.current?.sessionId);
      setVideoSession(null);
      setMusicBaseQueue([]);
      setLastEvent("");
      if (!nextItem) return;
      ignorePromiseAlwaysLogUnexpected(warmEmbeddedSubtitleCaches(nextItem.id), "Player:warmEmbeddedSubtitles");
      const activeLibrary =
        libraries.find((library) => library.id === nextItem.library_id) ?? null;
      const libraryPrefs = resolveLibraryPlaybackPreferences(
        activeLibrary ?? { type: nextItem.type },
      );
      const preferredAudioLanguage = resolveEffectivePreferredAudioLanguage(nextItem, libraryPrefs);
      const burnEmbeddedSubtitleStreamIndex = resolveInitialBurnSubtitleStreamIndex(
        nextItem,
        libraryPrefs,
      );
      createClientPlaybackSession(nextItem, preferredInitialAudioIndex(nextItem, preferredAudioLanguage), {
        burnEmbeddedSubtitleStreamIndex: burnEmbeddedSubtitleStreamIndex ?? undefined,
      })
        .then((session) => {
          if (!mountedRef.current) return;
          applyPlaybackSession(session);
        })
        .catch((err) => {
          console.error("[Player] createPlaybackSession failed", err);
          setLastEvent(
            `Error: ${err instanceof Error ? err.message : "Failed to start playback"}`,
          );
        });
    },
    [
      applyPlaybackSession,
      closeVideoSession,
      createClientPlaybackSession,
      libraries,
      pauseAllMediaElements,
    ],
  );

  const playVideoQueueIndex = useCallback(
    (nextIndex: number) => {
      if (playbackSession?.activeMode !== "video" || playbackSession.queue.length === 0) return;
      const clampedIndex = Math.max(0, Math.min(nextIndex, playbackSession.queue.length - 1));
      const nextItem = playbackSession.queue[clampedIndex];
      if (!nextItem) return;

      pauseAllMediaElements();
      setPlaybackSession((current) =>
        current && current.activeMode === "video"
          ? {
              ...current,
              isDockOpen: true,
              viewMode: "window",
              queueIndex: clampedIndex,
            }
          : current,
      );
      closeVideoSession(videoSessionRef.current?.sessionId);
      setVideoSession(null);
      setMusicBaseQueue([]);
      setLastEvent("");

      const activeLibrary =
        libraries.find((library) => library.id === nextItem.library_id) ?? null;
      const libraryPrefs = resolveLibraryPlaybackPreferences(activeLibrary ?? { type: nextItem.type });
      const activeAudioTrack =
        activeItem?.embeddedAudioTracks?.find(
          (track) => track.streamIndex === videoSessionRef.current?.audioIndex,
        ) ?? null;
      const preferredAudioLanguage =
        activeAudioTrack?.language ||
        activeAudioTrack?.title ||
        resolveEffectivePreferredAudioLanguage(nextItem, libraryPrefs);
      const burnEmbeddedSubtitleStreamIndex =
        videoSessionRef.current?.burnEmbeddedSubtitleStreamIndex ??
        resolveInitialBurnSubtitleStreamIndex(nextItem, libraryPrefs);
      ignorePromiseAlwaysLogUnexpected(warmEmbeddedSubtitleCaches(nextItem.id), "Player:warmEmbeddedSubtitles");
      createClientPlaybackSession(nextItem, preferredInitialAudioIndex(nextItem, preferredAudioLanguage), {
        burnEmbeddedSubtitleStreamIndex: burnEmbeddedSubtitleStreamIndex ?? undefined,
      })
        .then((session) => {
          if (!mountedRef.current) return;
          applyPlaybackSession(session);
        })
        .catch((err) => {
          console.error("[Player] createPlaybackSession failed", err);
          setLastEvent(
            `Error: ${err instanceof Error ? err.message : "Failed to start playback"}`,
          );
        });
    },
    [
      applyPlaybackSession,
      activeItem,
      closeVideoSession,
      createClientPlaybackSession,
      libraries,
      pauseAllMediaElements,
      playbackSession,
    ],
  );

  const changeAudioTrack = useCallback(
    async (audioIndex: number) => {
      const session = videoSessionRef.current;
      if (activeMode !== "video" || !activeItem) return;
      setLastEvent("Switching audio track...");
      try {
        if (!session?.sessionId) {
          const burn = session?.burnEmbeddedSubtitleStreamIndex;
          const nextSession = await createClientPlaybackSession(
            activeItem,
            audioIndex,
            burn != null ? { burnEmbeddedSubtitleStreamIndex: burn } : undefined,
          );
          applyPlaybackSession(nextSession);
          return;
        }

        const nextSession = await updatePlaybackSessionAudio(session.sessionId, {
          audioIndex,
        });
        if (nextSession.delivery === "direct") {
          applyPlaybackSession(nextSession);
          return;
        }

        setVideoSession((current) =>
          current == null || current.sessionId !== session.sessionId
            ? current
            : {
                ...current,
                delivery: nextSession.delivery,
                desiredRevision: nextSession.revision,
                audioIndex: nextSession.audioIndex,
                status: nextSession.status,
                streamUrl:
                  nextSession.status === "starting"
                    ? current.streamUrl
                    : resolvePlaybackStreamUrl(nextSession.streamUrl),
                durationSeconds: nextSession.durationSeconds,
                error: nextSession.error ?? "",
                burnEmbeddedSubtitleStreamIndex:
                  nextSession.burnEmbeddedSubtitleStreamIndex ??
                  current.burnEmbeddedSubtitleStreamIndex,
              },
        );
      } catch (err) {
        console.error("[Player] changeAudioTrack failed", err);
        setLastEvent(
          `Error: ${err instanceof Error ? err.message : "Failed to switch audio track"}`,
        );
      }
    },
    [activeItem, activeMode, applyPlaybackSession, createClientPlaybackSession],
  );

  const changeEmbeddedSubtitleBurn = useCallback(
    async (streamIndex: number | null) => {
      if (activeMode !== "video" || !activeItem) return;
      const vs = videoSessionRef.current;
      const activeLibrary =
        libraries.find((library) => library.id === activeItem.library_id) ?? null;
      const preferredAudioLanguage = resolveLibraryPlaybackPreferences(
        activeLibrary ?? { type: activeItem.type },
      ).preferredAudioLanguage;
      const audioIndex =
        vs?.audioIndex ??
        preferredInitialAudioIndex(activeItem, preferredAudioLanguage);

      setLastEvent("Switching subtitles...");
      try {
        // Create the new session BEFORE closing the old one. If we close first,
        // the server's "closed" WebSocket broadcast can arrive while we're still
        // awaiting the create call, nulling videoSession and killing HLS.js /
        // subtitle state mid-transition. With the new session applied first,
        // the "closed" event's sessionId won't match and is safely ignored.
        const oldSessionId = vs?.sessionId ?? null;
        const nextSession = await createClientPlaybackSession(
          activeItem,
          audioIndex,
          streamIndex != null
            ? { burnEmbeddedSubtitleStreamIndex: streamIndex }
            : undefined,
        );
        if (!mountedRef.current) return;
        applyPlaybackSession(nextSession);
        closeVideoSession(oldSessionId);
      } catch (err) {
        console.error("[Player] changeEmbeddedSubtitleBurn failed", err);
        setLastEvent(
          `Error: ${err instanceof Error ? err.message : "Failed to switch subtitles"}`,
        );
      }
    },
    [
      activeItem,
      activeMode,
      applyPlaybackSession,
      closeVideoSession,
      createClientPlaybackSession,
      libraries,
    ],
  );

  const playMovie = useCallback(
    (item: MediaItem) => {
      playVideoQueue([item]);
    },
    [playVideoQueue],
  );

  const playEpisode = useCallback(
    (item: MediaItem, options?: { showKey?: string }) => {
      const libId = item.library_id;
      const explicitKey = options?.showKey?.trim();
      const derivedKey =
        (item.type === "tv" || item.type === "anime") && (item.tmdb_id || item.title)
          ? getShowKey(item)
          : undefined;
      const showKey = explicitKey || derivedKey;

      if (
        (item.type === "tv" || item.type === "anime") &&
        libId != null &&
        libId > 0 &&
        showKey
      ) {
        getShowEpisodes(libId, showKey)
          .then((res) => {
            if (!mountedRef.current) return;
            const episodes = res.seasons.flatMap((s) => s.episodes) as MediaItem[];
            if (episodes.length > 0) {
              sortEpisodes(episodes);
              const idx = episodes.findIndex((e) => e.id === item.id);
              playVideoQueue(episodes, idx >= 0 ? idx : 0);
              return;
            }
            playVideoQueue([item]);
          })
          .catch((err) => {
            console.error("[Player] getShowEpisodes failed", err);
            playVideoQueue([item]);
          });
        return;
      }
      playVideoQueue([item]);
    },
    [playVideoQueue],
  );

  const playShowGroup = useCallback(
    (items: MediaItem[], startItem?: MediaItem) => {
      if (items.length === 0) return;
      const episodes = [...items];
      sortEpisodes(episodes);
      const startIndex =
        startItem == null
          ? 0
          : Math.max(
              0,
              episodes.findIndex((episode) => episode.id === startItem.id),
            );
      playVideoQueue(episodes, startIndex);
    },
    [playVideoQueue],
  );

  const playMusicCollection = useCallback(
    (items: MediaItem[], startItem?: MediaItem) => {
      const baseQueue = sortMusicTracks(
        items.filter((item) => item.type === "music"),
      );
      if (baseQueue.length === 0) return;

      pauseAllMediaElements();

      const target = startItem ?? baseQueue[0];
      const nextShuffle = activeMode === "music" ? shuffle : false;
      const nextRepeatMode = activeMode === "music" ? repeatMode : "off";
      const orderedQueue = nextShuffle
        ? shuffleQueue(baseQueue, target.id)
        : baseQueue;
      const nextIndex = Math.max(0, indexOfQueueItem(orderedQueue, target.id));

      setMusicBaseQueue(baseQueue);
      closeVideoSession(videoSessionRef.current?.sessionId);
      setVideoSession(null);
      setLastEvent("");
      setPlaybackSession({
        activeMode: "music",
        isDockOpen: true,
        viewMode: "window",
        queue: orderedQueue,
        queueIndex: nextIndex,
        shuffle: nextShuffle,
        repeatMode: nextRepeatMode,
      });
    },
    [activeMode, closeVideoSession, pauseAllMediaElements, repeatMode, shuffle],
  );

  const playMedia = useCallback(
    (item: MediaItem) => {
      if (item.type === "music") {
        playMusicCollection([item], item);
        return;
      }
      if (item.type === "movie") {
        playMovie(item);
        return;
      }
      playEpisode(item);
    },
    [playEpisode, playMovie, playMusicCollection],
  );

  const playNextInQueue = useCallback(() => {
    if (playbackSession?.activeMode === "video") {
      if (playbackSession.queueIndex >= playbackSession.queue.length - 1) return;
      playVideoQueueIndex(playbackSession.queueIndex + 1);
      return;
    }
    setPlaybackSession((current) => {
      if (
        !current ||
        current.activeMode !== "music" ||
        current.queue.length === 0
      )
        return current;
      const atLastItem = current.queueIndex >= current.queue.length - 1;
      if (!atLastItem) {
        return {
          ...current,
          queueIndex: current.queueIndex + 1,
          isDockOpen: true,
          viewMode: "window",
        };
      }
      if (current.repeatMode === "all") {
        return {
          ...current,
          queueIndex: 0,
          isDockOpen: true,
          viewMode: "window",
        };
      }
      return current;
    });
  }, [playVideoQueueIndex, playbackSession]);

  const playPreviousInQueue = useCallback(() => {
    if (playbackSession?.activeMode === "video") {
      if (playbackSession.queueIndex <= 0) return;
      playVideoQueueIndex(playbackSession.queueIndex - 1);
      return;
    }
    setPlaybackSession((current) => {
      if (
        !current ||
        current.activeMode !== "music" ||
        current.queue.length === 0
      )
        return current;
      if (current.queueIndex > 0) {
        return {
          ...current,
          queueIndex: current.queueIndex - 1,
          isDockOpen: true,
          viewMode: "window",
        };
      }
      if (current.repeatMode === "all") {
        return {
          ...current,
          queueIndex: current.queue.length - 1,
          isDockOpen: true,
          viewMode: "window",
        };
      }
      return current;
    });
  }, [playVideoQueueIndex, playbackSession]);

  const toggleShuffle = useCallback(() => {
    setPlaybackSession((current) => {
      if (!current || current.activeMode !== "music") return current;
      const currentTrack = current.queue[current.queueIndex];
      if (!currentTrack || musicBaseQueue.length === 0) {
        return { ...current, shuffle: !current.shuffle };
      }
      const nextShuffle = !current.shuffle;
      const nextQueue = nextShuffle
        ? shuffleQueue(musicBaseQueue, currentTrack.id)
        : musicBaseQueue;
      return {
        ...current,
        shuffle: nextShuffle,
        queue: nextQueue,
        queueIndex: Math.max(0, indexOfQueueItem(nextQueue, currentTrack.id)),
      };
    });
  }, [musicBaseQueue]);

  const cycleRepeatMode = useCallback(() => {
    setPlaybackSession((current) => {
      if (!current || current.activeMode !== "music") return current;
      if (current.repeatMode === "off")
        return { ...current, repeatMode: "all" };
      if (current.repeatMode === "all")
        return { ...current, repeatMode: "one" };
      return { ...current, repeatMode: "off" };
    });
  }, []);

  const togglePlayPause = useCallback(() => {
    const active = getActiveMediaElement();
    if (!active) return;
    if (active.paused) {
      ignorePromise(active.play(), "Player:togglePlayPause");
      return;
    }
    active.pause();
  }, [getActiveMediaElement]);

  const seekTo = useCallback(
    (seconds: number) => {
      const active = getActiveMediaElement();
      if (!active) return;
      if (active instanceof HTMLVideoElement) {
        const session = videoSessionRef.current;
        const queue = playbackSessionRef.current?.queue;
        const idx = playbackSessionRef.current?.queueIndex ?? 0;
        const item = queue?.[idx];
        const delivery = session?.delivery ?? "direct";
        active.currentTime = clampVideoSeekSeconds(
          active,
          seconds,
          session?.durationSeconds ?? 0,
          item?.duration ?? 0,
          delivery,
        );
        return;
      }
      const t = Math.max(0, seconds);
      if (Number.isFinite(active.duration) && active.duration > 0) {
        active.currentTime = Math.min(t, Math.max(0, active.duration - 0.01));
      } else {
        active.currentTime = t;
      }
    },
    [getActiveMediaElement],
  );

  const setMuted = useCallback((nextMuted: boolean) => {
    setMutedState(nextMuted);
  }, []);

  const setVolume = useCallback((nextVolume: number) => {
    const clamped = clampVolume(nextVolume);
    setVolumeState(clamped);
    if (clamped > 0) {
      setMutedState(false);
    }
  }, []);

  const enterFullscreen = useCallback(() => {
    if (activeMode !== "video" || !activeItem) return;
    setPlaybackSession((current) =>
      current ? { ...current, isDockOpen: true, viewMode: "window" } : current,
    );
  }, [activeItem, activeMode]);

  const exitFullscreen = useCallback(() => {
    exitBrowserFullscreen();
  }, [exitBrowserFullscreen]);

  useEffect(() => {
    if (!latestEvent || latestEvent.type !== "playback_session_update") {
      return;
    }

    const activeVideoItemId = activeVideoItemIdRef.current;
    const currentSession = videoSessionRef.current;
    if (
      activeVideoItemId == null ||
      currentSession == null ||
      latestEvent.mediaId !== activeVideoItemId ||
      latestEvent.sessionId !== currentSession.sessionId
    ) {
      return;
    }

    if (latestEvent.status === "ready") {
      const eventRevision = latestEvent.revision ?? 0;
      const shouldActivate = eventRevision >= currentSession.desiredRevision;
      setVideoSession((current) =>
        current == null || current.sessionId !== latestEvent.sessionId
          ? current
          : {
              ...current,
              currentRevision: shouldActivate
                ? eventRevision
                : current.currentRevision,
              desiredRevision: Math.max(
                current.desiredRevision,
                eventRevision,
              ),
              delivery: latestEvent.delivery,
              audioIndex: latestEvent.audioIndex,
              status: "ready",
              streamUrl: resolvePlaybackStreamUrl(latestEvent.streamUrl),
              durationSeconds:
                latestEvent.durationSeconds > 0
                  ? latestEvent.durationSeconds
                  : current.durationSeconds,
              error: latestEvent.error ?? "",
              burnEmbeddedSubtitleStreamIndex:
                latestEvent.burnEmbeddedSubtitleStreamIndex ??
                current.burnEmbeddedSubtitleStreamIndex,
            },
      );
      if (shouldActivate) {
        setLastEvent("Stream ready");
      }
      return;
    }

    if (latestEvent.status === "error") {
      setVideoSession((current) =>
        current == null || current.sessionId !== latestEvent.sessionId
          ? current
          : {
              ...current,
              status: "error",
              error: latestEvent.error ?? "Playback session failed",
            },
      );
      setLastEvent(`Error: ${latestEvent.error || "Playback session failed"}`);
      return;
    }

    if (latestEvent.status === "closed") {
      setVideoSession((current) =>
        current?.sessionId === latestEvent.sessionId ? null : current,
      );
      setLastEvent("");
      return;
    }

    setLastEvent("Preparing stream...");
  }, [eventSequence, latestEvent]);

  const sessionValue = useMemo<PlayerSessionContextValue>(
    () => ({
      playbackSession,
      activeItem,
      activeMode,
      isDockOpen,
      viewMode,
      videoSourceUrl,
      playbackDurationSeconds,
      videoDelivery,
      videoAudioIndex,
      burnEmbeddedSubtitleStreamIndex,
      dismissDock,
      wsConnected,
      lastEvent,
    }),
    [
      playbackSession,
      activeItem,
      activeMode,
      isDockOpen,
      viewMode,
      videoSourceUrl,
      playbackDurationSeconds,
      videoDelivery,
      videoAudioIndex,
      burnEmbeddedSubtitleStreamIndex,
      dismissDock,
      wsConnected,
      lastEvent,
    ],
  );

  const queueValue = useMemo<PlayerQueueContextValue>(
    () => ({
      queue,
      queueIndex,
      shuffle,
      repeatMode,
      playMedia,
      playMovie,
      playEpisode,
      playShowGroup,
      playMusicCollection,
      playNextInQueue,
      playPreviousInQueue,
      toggleShuffle,
      cycleRepeatMode,
    }),
    [
      queue,
      queueIndex,
      shuffle,
      repeatMode,
      playMedia,
      playMovie,
      playEpisode,
      playShowGroup,
      playMusicCollection,
      playNextInQueue,
      playPreviousInQueue,
      toggleShuffle,
      cycleRepeatMode,
    ],
  );

  const transportValue = useMemo<PlayerTransportContextValue>(
    () => ({
      volume,
      muted,
      togglePlayPause,
      seekTo,
      setMuted,
      setVolume,
      enterFullscreen,
      exitFullscreen,
      registerMediaElement,
      changeAudioTrack,
      changeEmbeddedSubtitleBurn,
    }),
    [
      volume,
      muted,
      togglePlayPause,
      seekTo,
      setMuted,
      setVolume,
      enterFullscreen,
      exitFullscreen,
      registerMediaElement,
      changeAudioTrack,
      changeEmbeddedSubtitleBurn,
    ],
  );

  return (
    <PlayerSessionContext.Provider value={sessionValue}>
      <PlayerQueueContext.Provider value={queueValue}>
        <PlayerTransportContext.Provider value={transportValue}>
          {children}
        </PlayerTransportContext.Provider>
      </PlayerQueueContext.Provider>
    </PlayerSessionContext.Provider>
  );
}

export function usePlayerSession(): PlayerSessionContextValue {
  const ctx = useContext(PlayerSessionContext);
  if (!ctx) {
    throw new Error("usePlayerSession must be used within PlayerProvider");
  }
  return ctx;
}

export function usePlayerQueue(): PlayerQueueContextValue {
  const ctx = useContext(PlayerQueueContext);
  if (!ctx) {
    throw new Error("usePlayerQueue must be used within PlayerProvider");
  }
  return ctx;
}

export function usePlayerTransport(): PlayerTransportContextValue {
  const ctx = useContext(PlayerTransportContext);
  if (!ctx) {
    throw new Error("usePlayerTransport must be used within PlayerProvider");
  }
  return ctx;
}

/** Subscribes to all player slices; prefer `usePlayerSession` / `usePlayerQueue` / `usePlayerTransport` to limit re-renders. */
export function usePlayer(): PlayerContextValue {
  return {
    ...usePlayerSession(),
    ...usePlayerQueue(),
    ...usePlayerTransport(),
  };
}
