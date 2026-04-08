import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type Dispatch,
  type MutableRefObject,
  type SetStateAction,
} from "react";
import { buildBackendUrl } from "@plum/shared";
import {
  BASE_URL,
  PLAYBACK_STREAM_BASE_URL,
  closePlaybackSession,
  createPlaybackSession,
  type CreatePlaybackSessionPayload,
  type MediaItem,
  type PlaybackSession as ApiPlaybackSession,
  type PlumWebSocketCommand,
  updatePlaybackSessionAudio,
} from "../api";
import { detectClientPlaybackCapabilities } from "../lib/playback/playerMedia";
import { ignorePromiseAlwaysLogUnexpected } from "../lib/ignorePromise";
import { useWs } from "./WsContext";
import type { PlaybackKind, PlaybackSession } from "./playerTypes";
import type { PlaybackPreferencesApi } from "./usePlaybackPreferences";

export type VideoSessionState = {
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
  burnEmbeddedSubtitleStreamIndex: number | null;
};

export type PlaybackSessionSource =
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

function mergePlaybackTracks(item: MediaItem, session: ApiPlaybackSession): MediaItem {
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

export type UsePlaybackSessionArgs = {
  setPlaybackSession: Dispatch<SetStateAction<PlaybackSession | null>>;
  activeItem: MediaItem | null;
  activeMode: PlaybackKind | null;
  playbackPreferences: PlaybackPreferencesApi;
  setLastEvent: Dispatch<SetStateAction<string>>;
  mountedRef: MutableRefObject<boolean>;
};

export function usePlaybackSession({
  setPlaybackSession,
  activeItem,
  activeMode,
  playbackPreferences,
  setLastEvent,
  mountedRef,
}: UsePlaybackSessionArgs) {
  const [videoSession, setVideoSession] = useState<VideoSessionState | null>(null);
  const activeVideoItemIdRef = useRef<number | null>(null);
  const videoSessionRef = useRef<VideoSessionState | null>(null);
  const lastReadyVideoUrlRef = useRef("");
  const lastReadyVideoItemRef = useRef<number | null>(null);
  const prevVideoSessionIdRef = useRef<string | null>(null);

  const { wsConnected, latestEvent, eventSequence, sendCommand } = useWs();

  activeVideoItemIdRef.current = activeMode === "video" ? (activeItem?.id ?? null) : null;
  videoSessionRef.current = videoSession;

  // Only expose the stream URL once the server reports "ready" (all initial segments on disk).
  // During "starting" we keep the *previous* ready URL so HLS.js continues playing the old
  // revision instead of loading a partial manifest and stalling at the transcode live-edge —
  // matching Android TV / ExoPlayer behavior.
  const activeItemId_ = activeItem?.id ?? null;
  if (activeItemId_ !== lastReadyVideoItemRef.current) {
    lastReadyVideoUrlRef.current = "";
    lastReadyVideoItemRef.current = activeItemId_;
  }

  const sessionId = videoSession?.sessionId ?? null;
  if (sessionId !== prevVideoSessionIdRef.current) {
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
      ? videoSession?.durationSeconds && videoSession.durationSeconds > 0
        ? videoSession.durationSeconds
        : Math.max(activeItem?.duration ?? 0, 0)
      : 0;
  const videoDelivery =
    activeMode === "video" && videoSession != null ? videoSession.delivery : null;
  const videoAudioIndex = activeMode === "video" ? (videoSession?.audioIndex ?? -1) : -1;
  const burnEmbeddedSubtitleStreamIndex =
    activeMode === "video" ? (videoSession?.burnEmbeddedSubtitleStreamIndex ?? null) : null;

  const sendPlaybackCommand = useCallback(
    (command: PlumWebSocketCommand) => {
      sendCommand(command);
    },
    [sendCommand],
  );

  const closeVideoSession = useCallback(
    (closeSessionId?: string | null) => {
      if (!closeSessionId) return;
      sendPlaybackCommand({ action: "detach_playback_session", sessionId: closeSessionId });
      ignorePromiseAlwaysLogUnexpected(
        closePlaybackSession(closeSessionId),
        "Player:closePlaybackSession",
      );
    },
    [sendPlaybackCommand],
  );

  const applyPlaybackSession = useCallback(
    (session: PlaybackSessionSource) => {
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
    },
    [setLastEvent, setPlaybackSession],
  );

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
        payload.burnEmbeddedSubtitleStreamIndex = options.burnEmbeddedSubtitleStreamIndex;
      }
      return createPlaybackSession(item.id, payload);
    },
    [],
  );

  useEffect(() => {
    if (!wsConnected) return;
    const sid = videoSession?.sessionId;
    if (!sid) return;
    sendPlaybackCommand({ action: "attach_playback_session", sessionId: sid });
  }, [sendPlaybackCommand, videoSession?.sessionId, wsConnected]);

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
    [activeItem, activeMode, applyPlaybackSession, createClientPlaybackSession, setLastEvent],
  );

  const changeEmbeddedSubtitleBurn = useCallback(
    async (streamIndex: number | null) => {
      if (activeMode !== "video" || !activeItem) return;
      const vs = videoSessionRef.current;
      const audioIndex = playbackPreferences.audioIndexForSubtitleBurnChange(activeItem, vs);

      setLastEvent("Switching subtitles...");
      try {
        const oldSessionId = vs?.sessionId ?? null;
        const nextSession = await createClientPlaybackSession(
          activeItem,
          audioIndex,
          streamIndex != null ? { burnEmbeddedSubtitleStreamIndex: streamIndex } : undefined,
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
      mountedRef,
      playbackPreferences,
      setLastEvent,
    ],
  );

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
              currentRevision: shouldActivate ? eventRevision : current.currentRevision,
              desiredRevision: Math.max(current.desiredRevision, eventRevision),
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
  }, [eventSequence, latestEvent, setLastEvent]);

  return {
    setVideoSession,
    videoSessionRef,
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
  };
}
