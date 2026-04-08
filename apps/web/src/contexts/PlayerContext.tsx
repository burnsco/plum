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
import { type MediaItem, type PlaybackSession as ApiPlaybackSession } from "../api";
import { clampVolume } from "../lib/playback/playerQueue";
import { clampVideoSeekSeconds } from "../lib/playback/playerMedia";
import { ignorePromise } from "../lib/ignorePromise";
import { useLibraries } from "../queries";
import {
  type MediaElementSlot,
  type MusicRepeatMode,
  type PlaybackKind,
  type PlaybackSession,
  type PlayerViewMode,
} from "./playerTypes";
import { PlaybackPreferencesProvider } from "./playbackPreferencesContext";
import { usePlaybackPreferences } from "./usePlaybackPreferences";
import { usePlaybackQueue } from "./usePlaybackQueue";
import { usePlaybackSession } from "./usePlaybackSession";

export type {
  MediaElementSlot,
  MusicRepeatMode,
  PlaybackKind,
  PlaybackSession,
  PlayerViewMode,
} from "./playerTypes";

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
  const librariesQuery = useLibraries();
  const libraries = librariesQuery.data ?? [];
  const playbackPreferences = usePlaybackPreferences(libraries);
  const [playbackSession, setPlaybackSession] =
    useState<PlaybackSession | null>(null);
  const [volume, setVolumeState] = useState(1);
  const [muted, setMutedState] = useState(false);
  const [lastEvent, setLastEvent] = useState("");
  const mountedRef = useRef(true);
  const volumeRef = useRef(1);
  const mutedRef = useRef(false);
  const playbackSessionRef = useRef<PlaybackSession | null>(null);
  const mediaElementsRef = useRef<
    Record<MediaElementSlot, HTMLMediaElement | null>
  >({
    audio: null,
    video: null,
  });

  const pauseAllMediaElements = useCallback(() => {
    mediaElementsRef.current.audio?.pause();
    mediaElementsRef.current.video?.pause();
  }, []);

  const activeItem = playbackSession?.queue[playbackSession.queueIndex] ?? null;
  const activeMode = playbackSession?.activeMode ?? null;
  const isDockOpen = playbackSession?.isDockOpen ?? false;
  const viewMode: PlayerViewMode = playbackSession?.viewMode ?? "window";
  playbackSessionRef.current = playbackSession;

  const {
    videoSessionRef,
    setVideoSession,
    videoSourceUrl,
    playbackDurationSeconds,
    videoDelivery,
    videoAudioIndex,
    burnEmbeddedSubtitleStreamIndex,
    closeVideoSession,
    applyPlaybackSession,
    createClientPlaybackSession,
    changeAudioTrack,
    changeEmbeddedSubtitleBurn,
    wsConnected,
  } = usePlaybackSession({
    setPlaybackSession,
    activeItem,
    activeMode,
    playbackPreferences,
    setLastEvent,
    mountedRef,
  });

  const {
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
    clearMusicBaseQueue,
  } = usePlaybackQueue({
    playbackSession,
    setPlaybackSession,
    activeItem,
    activeMode,
    playbackPreferences,
    mountedRef,
    setLastEvent,
    pauseAllMediaElements,
    video: {
      videoSessionRef,
      setVideoSession,
      closeVideoSession,
      applyPlaybackSession,
      createClientPlaybackSession,
    },
  });

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
    clearMusicBaseQueue();
    setLastEvent("");
  }, [
    clearMusicBaseQueue,
    closeVideoSession,
    exitBrowserFullscreen,
    pauseAllMediaElements,
    playbackSession?.activeMode,
    setVideoSession,
    videoSessionRef,
  ]);

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
    [getActiveMediaElement, videoSessionRef],
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
    <PlaybackPreferencesProvider
      value={{
        api: playbackPreferences,
        librariesFetched: librariesQuery.isFetched,
      }}
    >
      <PlayerSessionContext.Provider value={sessionValue}>
        <PlayerQueueContext.Provider value={queueValue}>
          <PlayerTransportContext.Provider value={transportValue}>
            {children}
          </PlayerTransportContext.Provider>
        </PlayerQueueContext.Provider>
      </PlayerSessionContext.Provider>
    </PlaybackPreferencesProvider>
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

export { usePlayerPlaybackPreferences } from "./playbackPreferencesContext";
