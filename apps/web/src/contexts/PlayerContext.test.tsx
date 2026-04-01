import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { MediaItem } from "../api";
import * as api from "../api";
import { PlayerProvider, usePlayer } from "./PlayerContext";
import { WsProvider } from "./WsContext";

type MockWebSocketHandle = {
  close: (code?: number, reason?: string) => void;
  mockMessage: (data: string) => void;
  sentMessages: string[];
};

type MockWebSocketClass = {
  instances: MockWebSocketHandle[];
  reset: () => void;
};

class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  static instances: MockWebSocket[] = [];

  static reset() {
    MockWebSocket.instances = [];
  }

  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;

  private listeners: Record<string, Set<(event: Event) => void>> = {
    open: new Set(),
    message: new Set(),
    error: new Set(),
    close: new Set(),
  };

  readyState: number = MockWebSocket.CONNECTING;
  sentMessages: string[] = [];
  url: string;

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
    setTimeout(() => {
      this.readyState = MockWebSocket.OPEN;
      this.emit("open", new Event("open"));
    }, 0);
  }

  send(data: string | ArrayBufferLike | Blob | ArrayBufferView) {
    if (typeof data === "string") {
      this.sentMessages.push(data);
    }
  }

  close(code?: number, reason?: string) {
    this.readyState = MockWebSocket.CLOSED;
    this.emit("close", {
      code: code ?? 1000,
      reason: reason ?? "",
      wasClean: true,
    } as CloseEvent);
  }

  private emit(type: string, event: Event) {
    if (type === "open") {
      this.onopen?.(event);
    } else if (type === "message") {
      this.onmessage?.(event as MessageEvent);
    } else if (type === "error") {
      this.onerror?.(event);
    } else if (type === "close") {
      this.onclose?.(event as CloseEvent);
    }

    const listeners = this.listeners[type];
    if (!listeners) return;
    for (const listener of listeners) {
      listener(event);
    }
  }

  addEventListener(type: string, listener: EventListener) {
    const listeners = this.listeners[type];
    if (!listeners) return;
    listeners.add(listener as (event: Event) => void);
  }

  removeEventListener(type: string, listener: EventListener) {
    const listeners = this.listeners[type];
    if (!listeners) return;
    listeners.delete(listener as (event: Event) => void);
  }

  dispatchEvent() {
    return true;
  }

  mockMessage(data: string) {
    this.emit("message", {
      data,
    } as MessageEvent);
  }
}

(globalThis as typeof globalThis & { WebSocket: typeof WebSocket }).WebSocket =
  MockWebSocket as unknown as typeof WebSocket;

const movie: MediaItem = {
  id: 99,
  title: "Die My Love",
  path: "/movies/Die My Love (2025)/Die My Love.mp4",
  duration: 7200,
  type: "movie",
};

const episodeOne: MediaItem = {
  id: 201,
  title: "Episode One",
  path: "/shows/Example/Season 1/Episode One.mkv",
  duration: 1800,
  type: "tv",
  season: 1,
  episode: 1,
  embeddedAudioTracks: [
    { streamIndex: 1, language: "eng", title: "English" },
    { streamIndex: 2, language: "jpn", title: "Japanese" },
  ],
};

const episodeTwo: MediaItem = {
  id: 202,
  title: "Episode Two",
  path: "/shows/Example/Season 1/Episode Two.mkv",
  duration: 1800,
  type: "tv",
  season: 1,
  episode: 2,
  library_id: 7,
  embeddedAudioTracks: [
    { streamIndex: 3, language: "eng", title: "English" },
    { streamIndex: 5, language: "jpn", title: "Japanese" },
  ],
};

async function flushMicrotasks() {
  await Promise.resolve();
  await Promise.resolve();
}

function PlayerHarness() {
  const { activeItem, dismissDock, lastEvent, playMovie, videoSourceUrl } =
    usePlayer();

  return (
    <div>
      <button type="button" onClick={() => playMovie(movie)}>
        Play
      </button>
      <button type="button" onClick={() => dismissDock()}>
        Dismiss
      </button>
      <div data-testid="active-media-id">{activeItem?.id ?? ""}</div>
      <div data-testid="last-event">{lastEvent}</div>
      <div data-testid="video-source-url">{videoSourceUrl}</div>
    </div>
  );
}

function VideoQueueHarness() {
  const {
    activeItem,
    playShowGroup,
    playNextInQueue,
    playPreviousInQueue,
  } = usePlayer();

  return (
    <div>
      <button type="button" onClick={() => playShowGroup([episodeTwo, episodeOne], episodeTwo)}>
        Play Show
      </button>
      <button type="button" onClick={() => playNextInQueue()}>
        Next
      </button>
      <button type="button" onClick={() => playPreviousInQueue()}>
        Previous
      </button>
      <div data-testid="queue-media-id">{activeItem?.id ?? ""}</div>
    </div>
  );
}

function PlaybackTrackHydrationHarness() {
  const { activeItem, playShowGroup } = usePlayer();
  const bareEpisodeOne: MediaItem = {
    id: 301,
    title: "Hydration Episode One",
    path: "/shows/Hydration/Season 1/Episode One.mkv",
    duration: 1800,
    type: "anime",
    season: 1,
    episode: 1,
    library_id: 7,
  };
  const bareEpisodeTwo: MediaItem = {
    id: 302,
    title: "Hydration Episode Two",
    path: "/shows/Hydration/Season 1/Episode Two.mkv",
    duration: 1800,
    type: "anime",
    season: 1,
    episode: 2,
    library_id: 7,
  };

  return (
    <div>
      <button
        type="button"
        onClick={() => playShowGroup([bareEpisodeTwo, bareEpisodeOne], bareEpisodeTwo)}
      >
        Play Bare Show
      </button>
      <div data-testid="hydrated-audio-count">
        {activeItem?.embeddedAudioTracks?.length ?? 0}
      </div>
      <div data-testid="hydrated-subtitle-count">
        {(activeItem?.embeddedSubtitles?.length ?? 0) + (activeItem?.subtitles?.length ?? 0)}
      </div>
    </div>
  );
}

describe("PlayerContext playback session updates", () => {
  beforeEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.spyOn(api, "getMe").mockResolvedValue({
      id: 1,
      email: "admin@example.com",
      is_admin: true,
    });
    vi.spyOn(api, "listLibraries").mockResolvedValue([]);
    vi.spyOn(api, "createPlaybackSession").mockResolvedValue({
      sessionId: "session-99",
      delivery: "transcode",
      mediaId: 99,
      revision: 1,
      audioIndex: -1,
      status: "starting",
      streamUrl: "/api/playback/sessions/session-99/revisions/1/index.m3u8",
      durationSeconds: 7200,
    });
    vi.spyOn(api, "closePlaybackSession").mockResolvedValue();
    vi.spyOn(api, "updatePlaybackSessionAudio").mockResolvedValue({
      sessionId: "session-99",
      delivery: "transcode",
      mediaId: 99,
      revision: 2,
      audioIndex: 1,
      status: "starting",
      streamUrl: "/api/playback/sessions/session-99/revisions/2/index.m3u8",
      durationSeconds: 7200,
    });
    (globalThis.WebSocket as unknown as MockWebSocketClass).reset();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("ignores unrelated playback events and applies the active session revision", async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <WsProvider>
          <PlayerProvider>
            <PlayerHarness />
          </PlayerProvider>
        </WsProvider>
      </QueryClientProvider>,
    );

    const MockWebSocket = globalThis.WebSocket as unknown as MockWebSocketClass;
    await waitFor(() => {
      expect(MockWebSocket.instances.length).toBeGreaterThan(0);
    });
    const socket = MockWebSocket.instances[0];
    if (!socket) {
      throw new Error("Expected a mock WebSocket instance");
    }

    fireEvent.click(screen.getByRole("button", { name: "Play" }));

    await waitFor(() => {
      expect(api.createPlaybackSession).toHaveBeenCalledWith(
        99,
        expect.objectContaining({
          audioIndex: -1,
          clientCapabilities: expect.any(Object),
        }),
      );
      expect(screen.getByTestId("active-media-id")).toHaveTextContent("99");
      expect(screen.getByTestId("last-event")).toHaveTextContent(
        "Preparing stream...",
      );
      expect(socket.sentMessages).toContain(
        JSON.stringify({
          action: "attach_playback_session",
          sessionId: "session-99",
        }),
      );
    });

    act(() => {
      socket.mockMessage(
        JSON.stringify({
          type: "playback_session_update",
          sessionId: "session-22",
          delivery: "transcode",
          mediaId: 22,
          revision: 1,
          audioIndex: -1,
          status: "ready",
          streamUrl:
            "/api/playback/sessions/session-22/revisions/1/index.m3u8",
          durationSeconds: 1800,
        }),
      );
    });

    expect(screen.getByTestId("video-source-url")).toHaveTextContent("");

    act(() => {
      socket.mockMessage(
        JSON.stringify({
          type: "playback_session_update",
          sessionId: "session-99",
          delivery: "transcode",
          mediaId: 99,
          revision: 1,
          audioIndex: -1,
          status: "ready",
          streamUrl:
            "/api/playback/sessions/session-99/revisions/1/index.m3u8",
          durationSeconds: 7200,
        }),
      );
    });

    await waitFor(() => {
      expect(screen.getByTestId("last-event")).toHaveTextContent(
        "Stream ready",
      );
      expect(screen.getByTestId("video-source-url")).toHaveTextContent(
        "/api/playback/sessions/session-99/revisions/1/index.m3u8",
      );
    });
  });

  it("reattaches the active playback session after websocket reconnect", async () => {
    vi.useFakeTimers();

    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <WsProvider>
          <PlayerProvider>
            <PlayerHarness />
          </PlayerProvider>
        </WsProvider>
      </QueryClientProvider>,
    );

    const MockWebSocket = globalThis.WebSocket as unknown as MockWebSocketClass;
    await act(async () => {
      await vi.runOnlyPendingTimersAsync();
      await vi.runOnlyPendingTimersAsync();
    });
    expect(MockWebSocket.instances.length).toBeGreaterThan(0);

    const firstSocket = MockWebSocket.instances[0];
    if (!firstSocket) {
      throw new Error("Expected a mock WebSocket instance");
    }

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Play" }));
      await flushMicrotasks();
    });
    expect(firstSocket.sentMessages).toContain(
      JSON.stringify({
        action: "attach_playback_session",
        sessionId: "session-99",
      }),
    );

    act(() => {
      firstSocket.close();
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3000);
      await vi.runOnlyPendingTimersAsync();
      await vi.runOnlyPendingTimersAsync();
      await flushMicrotasks();
    });

    expect(MockWebSocket.instances.length).toBeGreaterThan(1);

    const secondSocket = MockWebSocket.instances[1];
    if (!secondSocket) {
      throw new Error("Expected a reconnected mock WebSocket instance");
    }

    expect(secondSocket.sentMessages).toContain(
      JSON.stringify({
        action: "attach_playback_session",
        sessionId: "session-99",
      }),
    );
  });

  it("detaches the playback session before closing it from the player", async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <WsProvider>
          <PlayerProvider>
            <PlayerHarness />
          </PlayerProvider>
        </WsProvider>
      </QueryClientProvider>,
    );

    const MockWebSocket = globalThis.WebSocket as unknown as MockWebSocketClass;
    await waitFor(() => {
      expect(MockWebSocket.instances.length).toBeGreaterThan(0);
    });

    const socket = MockWebSocket.instances[0];
    if (!socket) {
      throw new Error("Expected a mock WebSocket instance");
    }

    fireEvent.click(screen.getByRole("button", { name: "Play" }));

    await waitFor(() => {
      expect(api.createPlaybackSession).toHaveBeenCalled();
    });

    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));

    await waitFor(() => {
      expect(socket.sentMessages).toContain(
        JSON.stringify({
          action: "detach_playback_session",
          sessionId: "session-99",
        }),
      );
      expect(api.closePlaybackSession).toHaveBeenCalledWith("session-99");
    });
  });

  it("plays a show queue from the requested episode after sorting episodes", async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    vi.spyOn(api, "createPlaybackSession").mockImplementation(async (mediaId) => ({
      sessionId: `session-${mediaId}`,
      delivery: "transcode",
      mediaId,
      revision: 1,
      audioIndex: -1,
      status: "starting",
      streamUrl: `/api/playback/sessions/session-${mediaId}/revisions/1/index.m3u8`,
      durationSeconds: 1800,
    }));

    render(
      <QueryClientProvider client={queryClient}>
        <WsProvider>
          <PlayerProvider>
            <VideoQueueHarness />
          </PlayerProvider>
        </WsProvider>
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Play Show" }));

    await waitFor(() => {
      expect(screen.getByTestId("queue-media-id")).toHaveTextContent("202");
      expect(api.createPlaybackSession).toHaveBeenCalledWith(
        202,
        expect.objectContaining({ clientCapabilities: expect.any(Object) }),
      );
    });
  });

  it("moves queued video forward and backward without wrapping", async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    vi.spyOn(api, "createPlaybackSession").mockImplementation(async (mediaId) => ({
      sessionId: `session-${mediaId}`,
      delivery: "transcode",
      mediaId,
      revision: 1,
      audioIndex: -1,
      status: "starting",
      streamUrl: `/api/playback/sessions/session-${mediaId}/revisions/1/index.m3u8`,
      durationSeconds: 1800,
    }));

    render(
      <QueryClientProvider client={queryClient}>
        <WsProvider>
          <PlayerProvider>
            <VideoQueueHarness />
          </PlayerProvider>
        </WsProvider>
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Play Show" }));

    await waitFor(() => {
      expect(screen.getByTestId("queue-media-id")).toHaveTextContent("202");
    });

    fireEvent.click(screen.getByRole("button", { name: "Next" }));

    await waitFor(() => {
      expect(screen.getByTestId("queue-media-id")).toHaveTextContent("202");
    });

    fireEvent.click(screen.getByRole("button", { name: "Previous" }));

    await waitFor(() => {
      expect(screen.getByTestId("queue-media-id")).toHaveTextContent("201");
      expect(api.createPlaybackSession).toHaveBeenLastCalledWith(
        201,
        expect.objectContaining({ clientCapabilities: expect.any(Object) }),
      );
    });
  });

  it("keeps the active audio language when moving between queued episodes", async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    vi.spyOn(api, "listLibraries").mockResolvedValue([
      {
        id: 7,
        name: "Anime",
        type: "anime",
        path: "/anime",
        user_id: 1,
        preferred_audio_language: "en",
        preferred_subtitle_language: "en",
        subtitles_enabled_by_default: true,
      },
    ]);
    vi.spyOn(api, "createPlaybackSession").mockImplementation(async (mediaId) => ({
      sessionId: `session-${mediaId}`,
      delivery: "transcode",
      mediaId,
      revision: 1,
      audioIndex: mediaId === 202 ? 5 : 2,
      status: "starting",
      streamUrl: `/api/playback/sessions/session-${mediaId}/revisions/1/index.m3u8`,
      durationSeconds: 1800,
    }));

    render(
      <QueryClientProvider client={queryClient}>
        <WsProvider>
          <PlayerProvider>
            <VideoQueueHarness />
          </PlayerProvider>
        </WsProvider>
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Play Show" }));

    await waitFor(() => {
      expect(screen.getByTestId("queue-media-id")).toHaveTextContent("202");
    });

    fireEvent.click(screen.getByRole("button", { name: "Previous" }));

    await waitFor(() => {
      expect(api.createPlaybackSession).toHaveBeenLastCalledWith(
        201,
        expect.objectContaining({
          audioIndex: 2,
          clientCapabilities: expect.any(Object),
        }),
      );
    });
  });

  it("hydrates episode track metadata from the playback session response", async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });

    vi.spyOn(api, "createPlaybackSession").mockImplementation(async (mediaId) => ({
      sessionId: `session-${mediaId}`,
      delivery: "transcode",
      mediaId,
      revision: 1,
      audioIndex: 5,
      status: "starting",
      streamUrl: `/api/playback/sessions/session-${mediaId}/revisions/1/index.m3u8`,
      durationSeconds: 1800,
      embeddedSubtitles: [{ streamIndex: 7, language: "eng", title: "English Signs" }],
      embeddedAudioTracks: [{ streamIndex: 5, language: "jpn", title: "Japanese" }],
    }));

    render(
      <QueryClientProvider client={queryClient}>
        <WsProvider>
          <PlayerProvider>
            <PlaybackTrackHydrationHarness />
          </PlayerProvider>
        </WsProvider>
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Play Bare Show" }));

    await waitFor(() => {
      expect(screen.getByTestId("hydrated-audio-count")).toHaveTextContent("1");
      expect(screen.getByTestId("hydrated-subtitle-count")).toHaveTextContent("1");
    });
  });
});
