import {
  useCallback,
  useMemo,
  useState,
  type Dispatch,
  type MutableRefObject,
  type SetStateAction,
} from "react";
import { getShowEpisodes, warmEmbeddedSubtitleCaches, type MediaItem } from "../api";
import {
  indexOfQueueItem,
  preferredInitialAudioIndex,
  shuffleQueue,
} from "../lib/playback/playerQueue";
import { ignorePromiseAlwaysLogUnexpected } from "../lib/ignorePromise";
import { sortMusicTracks } from "../lib/musicGrouping";
import { getShowKey, sortEpisodes } from "../lib/showGrouping";
import type { PlaybackKind, PlaybackSession } from "./playerTypes";
import type { PlaybackSessionSource, VideoSessionState } from "./usePlaybackSession";
import type { PlaybackPreferencesApi } from "./usePlaybackPreferences";

export type PlaybackSessionVideoApi = {
  videoSessionRef: MutableRefObject<VideoSessionState | null>;
  setVideoSession: Dispatch<SetStateAction<VideoSessionState | null>>;
  closeVideoSession: (sessionId?: string | null) => void;
  applyPlaybackSession: (session: PlaybackSessionSource) => void;
  createClientPlaybackSession: (
    item: MediaItem,
    audioIndex: number,
    options?: { burnEmbeddedSubtitleStreamIndex?: number },
  ) => Promise<PlaybackSessionSource>;
};

export type UsePlaybackQueueArgs = {
  playbackSession: PlaybackSession | null;
  setPlaybackSession: Dispatch<SetStateAction<PlaybackSession | null>>;
  activeItem: MediaItem | null;
  activeMode: PlaybackKind | null;
  playbackPreferences: PlaybackPreferencesApi;
  mountedRef: MutableRefObject<boolean>;
  setLastEvent: Dispatch<SetStateAction<string>>;
  pauseAllMediaElements: () => void;
  video: PlaybackSessionVideoApi;
};

export function usePlaybackQueue({
  playbackSession,
  setPlaybackSession,
  activeItem,
  activeMode,
  playbackPreferences,
  mountedRef,
  setLastEvent,
  pauseAllMediaElements,
  video,
}: UsePlaybackQueueArgs) {
  const {
    videoSessionRef,
    setVideoSession,
    closeVideoSession,
    applyPlaybackSession,
    createClientPlaybackSession,
  } = video;

  const [musicBaseQueue, setMusicBaseQueue] = useState<MediaItem[]>([]);

  const queue = useMemo(() => playbackSession?.queue ?? [], [playbackSession]);
  const queueIndex = playbackSession?.queueIndex ?? 0;
  const shuffle = playbackSession?.shuffle ?? false;
  const repeatMode = playbackSession?.repeatMode ?? "off";

  const clearMusicBaseQueue = useCallback(() => {
    setMusicBaseQueue([]);
  }, []);

  const playVideoQueue = useCallback(
    (
      items: MediaItem[],
      startIndex = 0,
      options?: { resumeIntent?: "continue_watching" },
    ) => {
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
        resumeIntent: options?.resumeIntent,
      }));
      closeVideoSession(videoSessionRef.current?.sessionId);
      setVideoSession(null);
      setMusicBaseQueue([]);
      setLastEvent("");
      if (!nextItem) return;
      ignorePromiseAlwaysLogUnexpected(warmEmbeddedSubtitleCaches(nextItem.id), "Player:warmEmbeddedSubtitles");
      const burnEmbeddedSubtitleStreamIndex =
        playbackPreferences.initialBurnEmbeddedSubtitleStreamIndex(nextItem);
      createClientPlaybackSession(nextItem, playbackPreferences.initialAudioStreamIndex(nextItem), {
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
      mountedRef,
      pauseAllMediaElements,
      playbackPreferences,
      setLastEvent,
      setPlaybackSession,
      setVideoSession,
      videoSessionRef,
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
              resumeIntent: undefined,
            }
          : current,
      );
      closeVideoSession(videoSessionRef.current?.sessionId);
      setVideoSession(null);
      setMusicBaseQueue([]);
      setLastEvent("");

      const activeAudioTrack =
        activeItem?.embeddedAudioTracks?.find(
          (track) => track.streamIndex === videoSessionRef.current?.audioIndex,
        ) ?? null;
      const preferredAudioLanguage =
        activeAudioTrack?.language ||
        activeAudioTrack?.title ||
        playbackPreferences.effectivePreferredAudioLanguage(nextItem);
      const burnEmbeddedSubtitleStreamIndex =
        videoSessionRef.current?.burnEmbeddedSubtitleStreamIndex ??
        playbackPreferences.initialBurnEmbeddedSubtitleStreamIndex(nextItem);
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
      activeItem,
      applyPlaybackSession,
      closeVideoSession,
      createClientPlaybackSession,
      mountedRef,
      pauseAllMediaElements,
      playbackPreferences,
      playbackSession,
      setLastEvent,
      setPlaybackSession,
      setVideoSession,
      videoSessionRef,
    ],
  );

  const playMovie = useCallback(
    (item: MediaItem, options?: { resumeIntent?: "continue_watching" }) => {
      playVideoQueue([item], 0, options);
    },
    [playVideoQueue],
  );

  const playEpisode = useCallback(
    (item: MediaItem, options?: { showKey?: string; resumeIntent?: "continue_watching" }) => {
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
              playVideoQueue(episodes, idx >= 0 ? idx : 0, {
                resumeIntent: options?.resumeIntent,
              });
              return;
            }
            playVideoQueue([item], 0, { resumeIntent: options?.resumeIntent });
          })
          .catch((err) => {
            console.error("[Player] getShowEpisodes failed", err);
            playVideoQueue([item], 0, { resumeIntent: options?.resumeIntent });
          });
        return;
      }
      playVideoQueue([item], 0, { resumeIntent: options?.resumeIntent });
    },
    [mountedRef, playVideoQueue],
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
    [
      activeMode,
      closeVideoSession,
      pauseAllMediaElements,
      repeatMode,
      setLastEvent,
      setPlaybackSession,
      setVideoSession,
      shuffle,
      videoSessionRef,
    ],
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
  }, [playVideoQueueIndex, playbackSession, setPlaybackSession]);

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
  }, [playVideoQueueIndex, playbackSession, setPlaybackSession]);

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
  }, [musicBaseQueue, setPlaybackSession]);

  const cycleRepeatMode = useCallback(() => {
    setPlaybackSession((current) => {
      if (!current || current.activeMode !== "music") return current;
      if (current.repeatMode === "off")
        return { ...current, repeatMode: "all" };
      if (current.repeatMode === "all")
        return { ...current, repeatMode: "one" };
      return { ...current, repeatMode: "off" };
    });
  }, [setPlaybackSession]);

  return {
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
  };
}
