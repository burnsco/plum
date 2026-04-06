import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as api from "../api";
import { videoAutoplayStorageKey } from "../lib/playbackPreferences";
import { PlaybackDock } from "./PlaybackDock";

type MockHlsInstance = {
  handlers: Map<string, Array<(...args: unknown[]) => void>>;
  loadSource: ReturnType<typeof vi.fn>;
  attachMedia: ReturnType<typeof vi.fn>;
  destroy: ReturnType<typeof vi.fn>;
  startLoad: ReturnType<typeof vi.fn>;
  recoverMediaError: ReturnType<typeof vi.fn>;
  on: (event: string, handler: (...args: unknown[]) => void) => void;
  emit: (event: string, ...args: unknown[]) => void;
};

type MockCue = {
  startTime: number;
  endTime: number;
  text: string;
  line?: number | string;
};

type MockTextTrack = {
  mode: TextTrackMode;
  cues: MockCue[];
  addCue: (cue: MockCue) => void;
  removeCue: (cue: MockCue) => void;
};

const mockUsePlayer = vi.fn();
const mockChangeAudioTrack = vi.fn();
const mockChangeEmbeddedSubtitleBurn = vi.fn();
const { mockHlsInstances } = vi.hoisted(() => ({
  mockHlsInstances: [] as MockHlsInstance[],
}));

vi.mock("../contexts/PlayerContext", () => ({
  usePlayer: () => mockUsePlayer(),
  usePlayerSession: () => mockUsePlayer(),
  usePlayerQueue: () => mockUsePlayer(),
  usePlayerTransport: () => mockUsePlayer(),
}));

vi.mock("hls.js", () => ({
  default: class {
    static Events = {
      MANIFEST_PARSED: "manifestParsed",
      ERROR: "error",
    };

    static ErrorTypes = {
      NETWORK_ERROR: "networkError",
      MEDIA_ERROR: "mediaError",
    };

    static isSupported() {
      return true;
    }

    handlers = new Map<string, Array<(...args: unknown[]) => void>>();
    loadSource = vi.fn();
    attachMedia = vi.fn();
    destroy = vi.fn();
    startLoad = vi.fn();
    recoverMediaError = vi.fn();

    constructor() {
      mockHlsInstances.push(this);
    }

    on(event: string, handler: (...args: unknown[]) => void) {
      const handlers = this.handlers.get(event) ?? [];
      handlers.push(handler);
      this.handlers.set(event, handlers);
    }

    emit(event: string, ...args: unknown[]) {
      for (const handler of this.handlers.get(event) ?? []) {
        handler(...args);
      }
    }
  },
}));

function renderDock() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return {
    queryClient,
    ...render(
      <QueryClientProvider client={queryClient}>
        <PlaybackDock />
      </QueryClientProvider>,
    ),
  };
}

function setVideoCurrentTime(video: HTMLVideoElement, currentTime: number) {
  Object.defineProperty(video, "currentTime", {
    configurable: true,
    value: currentTime,
    writable: true,
  });
}

function setVideoDuration(video: HTMLVideoElement, duration: number) {
  Object.defineProperty(video, "duration", {
    configurable: true,
    value: duration,
    writable: true,
  });
}

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function defaultPlaybackDockUsePlayer() {
  return {
    activeItem: {
      id: 42,
      library_id: 7,
      title: "Track Test",
      path: "/movies/track-test.mkv",
      duration: 120,
      type: "movie" as const,
      embeddedAudioTracks: [
        { streamIndex: 1, language: "eng", title: "English" },
        { streamIndex: 2, language: "jpn", title: "Japanese" },
      ],
    },
    activeMode: "video" as const,
    isDockOpen: true,
    viewMode: "window" as const,
    queue: [] as api.MediaItem[],
    queueIndex: 0,
    shuffle: false,
    repeatMode: "off" as const,
    volume: 1,
    muted: false,
    videoSourceUrl:
      "http://localhost:3000/api/playback/sessions/session-1/revisions/1/index.m3u8",
    playbackDurationSeconds: 120,
    videoDelivery: "transcode" as const,
    videoAudioIndex: -1,
    burnEmbeddedSubtitleStreamIndex: null,
    wsConnected: false,
    lastEvent: "",
    registerMediaElement: vi.fn(),
    togglePlayPause: vi.fn(),
    seekTo: vi.fn(),
    setMuted: vi.fn(),
    setVolume: vi.fn(),
    enterFullscreen: vi.fn(),
    exitFullscreen: vi.fn(),
    dismissDock: vi.fn(),
    playNextInQueue: vi.fn(),
    playPreviousInQueue: vi.fn(),
    toggleShuffle: vi.fn(),
    cycleRepeatMode: vi.fn(),
    changeAudioTrack: mockChangeAudioTrack,
    changeEmbeddedSubtitleBurn: mockChangeEmbeddedSubtitleBurn,
  };
}

describe("PlaybackDock audio track selection", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    window.localStorage.clear();
    vi.spyOn(console, "error").mockImplementation(() => {});
    mockChangeAudioTrack.mockReset();
    mockChangeAudioTrack.mockResolvedValue(undefined);
    mockChangeEmbeddedSubtitleBurn.mockReset();
    mockChangeEmbeddedSubtitleBurn.mockResolvedValue(undefined);
    mockHlsInstances.length = 0;
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
    vi.spyOn(api, "refreshPlaybackTracks").mockResolvedValue({
      subtitles: [],
      embeddedSubtitles: [],
      embeddedAudioTracks: [],
    });
    vi.spyOn(api, "updateMediaProgress").mockResolvedValue();
    mockUsePlayer.mockReturnValue(defaultPlaybackDockUsePlayer());
  });

  it("switches audio tracks from the theater player and requests a matching transcode", async () => {
    const { container } = renderDock();
    const video = container.querySelector("video") as HTMLVideoElement | null;
    expect(video).toBeTruthy();
    if (!video) {
      throw new Error("Expected a video element");
    }

    const browserAudioTracks = [{ enabled: true }, { enabled: false }];
    Object.defineProperty(video, "audioTracks", {
      configurable: true,
      value: browserAudioTracks,
    });

    fireEvent.loadedMetadata(video);

    const audioButton = await screen.findByRole("button", { name: /Audio track:/i });
    fireEvent.click(audioButton);
    fireEvent.click(screen.getByRole("option", { name: /Japanese/i }));

    await waitFor(() => {
      expect(browserAudioTracks[0]?.enabled).toBe(false);
      expect(browserAudioTracks[1]?.enabled).toBe(true);
      expect(mockChangeAudioTrack).toHaveBeenCalledWith(2);
    });
  });

  it("reloads the active video element when the playback revision URL changes", async () => {
    const { queryClient, rerender } = renderDock();

    await waitFor(() => {
      expect(mockHlsInstances).toHaveLength(1);
    });

    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      videoSourceUrl:
        "http://localhost:3000/api/playback/sessions/session-1/revisions/2/index.m3u8",
    });

    rerender(
      <QueryClientProvider client={queryClient}>
        <PlaybackDock />
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(mockHlsInstances).toHaveLength(2);
    });
    expect(mockHlsInstances[1]?.loadSource).toHaveBeenCalledWith(
      "http://localhost:3000/api/playback/sessions/session-1/revisions/2/index.m3u8",
    );
  });

  it("shows a loading overlay before video playback is ready", async () => {
    const { container } = renderDock();
    const video = container.querySelector("video") as HTMLVideoElement | null;
    expect(video).toBeTruthy();
    if (!video) {
      throw new Error("Expected a video element");
    }

    expect(
      screen.getByRole("status", { name: "Preparing playback..." }),
    ).toBeTruthy();

    fireEvent.canPlay(video);

    await waitFor(() => {
      expect(
        screen.queryByRole("status", { name: "Preparing playback..." }),
      ).toBeNull();
    });
  });

  it("does not show the preparing overlay when waiting after playback has started", async () => {
    const { container } = renderDock();
    const video = container.querySelector("video") as HTMLVideoElement | null;
    expect(video).toBeTruthy();
    if (!video) {
      throw new Error("Expected a video element");
    }

    fireEvent.canPlay(video);
    await waitFor(() => {
      expect(screen.queryByRole("status", { name: "Preparing playback..." })).toBeNull();
    });

    fireEvent.playing(video);
    fireEvent.waiting(video);

    expect(screen.queryByRole("status", { name: "Preparing playback..." })).toBeNull();
  });

  it("persists initial playback progress before the periodic interval elapses", async () => {
    const { container } = renderDock();
    const video = container.querySelector("video") as HTMLVideoElement | null;
    expect(video).toBeTruthy();
    if (!video) {
      throw new Error("Expected a video element");
    }

    setVideoCurrentTime(video, 3);

    fireEvent.timeUpdate(video);

    await waitFor(() => {
      expect(api.updateMediaProgress).toHaveBeenCalledWith(42, {
        position_seconds: 3,
        duration_seconds: 120,
        completed: false,
      });
    });
  });

  it("shows and persists the full session duration when the queue item duration is missing", async () => {
    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      activeItem: {
        id: 42,
        library_id: 7,
        title: "Track Test",
        path: "/movies/track-test.mkv",
        duration: 0,
        type: "movie",
        embeddedAudioTracks: [
          { streamIndex: 1, language: "eng", title: "English" },
          { streamIndex: 2, language: "jpn", title: "Japanese" },
        ],
      },
      playbackDurationSeconds: 7200,
    });

    const { container } = renderDock();
    const video = container.querySelector("video") as HTMLVideoElement | null;
    expect(video).toBeTruthy();
    if (!video) {
      throw new Error("Expected a video element");
    }

    setVideoDuration(video, 15);
    setVideoCurrentTime(video, 3);

    fireEvent.loadedMetadata(video);
    fireEvent.timeUpdate(video);

    await waitFor(() => {
      expect(api.updateMediaProgress).toHaveBeenCalledWith(42, {
        position_seconds: 3,
        duration_seconds: 7200,
        completed: false,
      });
    });
  });

  it("persists the latest playback position when closing the player", async () => {
    const { container } = renderDock();
    const video = container.querySelector("video") as HTMLVideoElement | null;
    expect(video).toBeTruthy();
    if (!video) {
      throw new Error("Expected a video element");
    }

    setVideoCurrentTime(video, 41);
    fireEvent.timeUpdate(video);
    vi.mocked(api.updateMediaProgress).mockClear();

    fireEvent.click(screen.getByRole("button", { name: "Close player" }));

    await waitFor(() => {
      expect(api.updateMediaProgress).toHaveBeenCalledWith(42, {
        position_seconds: 41,
        duration_seconds: 120,
        completed: false,
      });
    });
  });

  it("keeps the active HLS attachment when mute state rerenders the player", async () => {
    const { queryClient, rerender } = renderDock();

    await waitFor(() => {
      expect(mockHlsInstances).toHaveLength(1);
    });

    const firstHls = mockHlsInstances[0];
    if (!firstHls) {
      throw new Error("Expected an HLS instance");
    }

    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      muted: true,
      registerMediaElement: vi.fn(),
    });

    rerender(
      <QueryClientProvider client={queryClient}>
        <PlaybackDock />
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Unmute" })).toBeTruthy();
    });
    expect(mockHlsInstances).toHaveLength(1);
    expect(firstHls.destroy).not.toHaveBeenCalled();
  });

  it("restarts loading on fatal HLS network errors", async () => {
    renderDock();

    await waitFor(() => {
      expect(mockHlsInstances).toHaveLength(1);
    });

    const hls = mockHlsInstances[0];
    if (!hls) {
      throw new Error("Expected an HLS instance");
    }

    act(() => {
      hls.emit("error", "error", {
        fatal: true,
        type: "networkError",
        details: "fragLoadError",
      });
    });

    expect(hls.startLoad).toHaveBeenCalledTimes(1);
    await waitFor(() => {
      expect(screen.getByRole("status", { name: "Reconnecting stream..." })).toBeTruthy();
    });
  });

  it("recovers media playback on fatal HLS media errors", async () => {
    renderDock();

    await waitFor(() => {
      expect(mockHlsInstances).toHaveLength(1);
    });

    const hls = mockHlsInstances[0];
    if (!hls) {
      throw new Error("Expected an HLS instance");
    }

    act(() => {
      hls.emit("error", "error", {
        fatal: true,
        type: "mediaError",
        details: "bufferStalledError",
      });
    });

    expect(hls.recoverMediaError).toHaveBeenCalledTimes(1);
    await waitFor(() => {
      expect(screen.getByRole("status", { name: "Recovering playback..." })).toBeTruthy();
    });
  });

  it("shows a stream error when the HLS failure is fatal and unrecoverable", async () => {
    renderDock();

    await waitFor(() => {
      expect(mockHlsInstances).toHaveLength(1);
    });

    const hls = mockHlsInstances[0];
    if (!hls) {
      throw new Error("Expected an HLS instance");
    }

    act(() => {
      hls.emit("error", "error", {
        fatal: true,
        type: "otherFatalError",
        details: "appendError",
      });
    });

    await waitFor(() => {
      expect(screen.getByText("Stream error: appendError")).toBeTruthy();
    });
  });

  it("prefers the library default audio language when available", async () => {
    vi.spyOn(api, "listLibraries").mockResolvedValue([
      {
        id: 7,
        name: "Anime",
        type: "anime",
        path: "/anime",
        user_id: 1,
        preferred_audio_language: "ja",
        preferred_subtitle_language: "en",
        subtitles_enabled_by_default: true,
      },
    ]);
    const { container } = renderDock();
    const video = container.querySelector("video") as HTMLVideoElement | null;
    expect(video).toBeTruthy();
    if (!video) {
      throw new Error("Expected a video element");
    }

    const browserAudioTracks = [{ enabled: true }, { enabled: false }];
    Object.defineProperty(video, "audioTracks", {
      configurable: true,
      value: browserAudioTracks,
    });

    fireEvent.loadedMetadata(video);

    await waitFor(() => {
      expect(browserAudioTracks[0]?.enabled).toBe(false);
      expect(browserAudioTracks[1]?.enabled).toBe(true);
    });
    expect(mockChangeAudioTrack).not.toHaveBeenCalled();
  });

  it("reapplies the default subtitle track when media becomes ready", async () => {
    const originalCue = window.VTTCue;
    const originalAddTextTrack = HTMLMediaElement.prototype.addTextTrack;
    const originalTextTracksDescriptor = Object.getOwnPropertyDescriptor(
      HTMLMediaElement.prototype,
      "textTracks",
    );
    const tracksByElement = new WeakMap<HTMLMediaElement, MockTextTrack[]>();

    const createTrack = (): MockTextTrack => {
      const cues: MockCue[] = [];
      return {
        mode: "disabled",
        cues,
        addCue: (cue) => {
          cues.push(cue);
        },
        removeCue: (cue) => {
          const index = cues.indexOf(cue);
          if (index >= 0) {
            cues.splice(index, 1);
          }
        },
      };
    };

    const addTextTrackMock = vi.fn(function (this: HTMLMediaElement) {
      const track = createTrack();
      const tracks = tracksByElement.get(this) ?? [];
      tracks.push(track);
      tracksByElement.set(this, tracks);
      return track as unknown as TextTrack;
    });

    Object.defineProperty(window, "VTTCue", {
      configurable: true,
      writable: true,
      value: class {
        startTime: number;
        endTime: number;
        text: string;

        constructor(startTime: number, endTime: number, text: string) {
          this.startTime = startTime;
          this.endTime = endTime;
          this.text = text;
        }
      },
    });
    Object.defineProperty(HTMLMediaElement.prototype, "addTextTrack", {
      configurable: true,
      value: addTextTrackMock,
    });
    Object.defineProperty(HTMLMediaElement.prototype, "textTracks", {
      configurable: true,
      get() {
        return tracksByElement.get(this) ?? [];
      },
    });

    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(
        new Response("WEBVTT\n\n00:00:00.000 --> 00:00:02.000\nHello world\n", { status: 200 }),
      );

    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      activeItem: {
        id: 42,
        library_id: 7,
        title: "Track Test",
        path: "/movies/track-test.mkv",
        duration: 120,
        type: "movie",
        subtitles: [{ id: 9, language: "eng", title: "English", format: "vtt" }],
        embeddedAudioTracks: [
          { streamIndex: 1, language: "eng", title: "English" },
          { streamIndex: 2, language: "jpn", title: "Japanese" },
        ],
      },
    });

    try {
      const { container } = renderDock();
      const video = container.querySelector("video") as HTMLVideoElement | null;
      expect(video).toBeTruthy();
      if (!video) {
        throw new Error("Expected a video element");
      }

      await waitFor(() => {
        expect(fetchSpy).toHaveBeenCalledTimes(1);
        expect(addTextTrackMock).toHaveBeenCalledTimes(1);
      });

      await waitFor(() => {
        const currentTracks = tracksByElement.get(video) ?? [];
        expect(currentTracks[0]?.mode).toBe("showing");
        expect(currentTracks[0]?.cues).toHaveLength(1);
      });

      tracksByElement.set(video, []);
      fireEvent.loadedMetadata(video);

      await waitFor(() => {
        expect(addTextTrackMock).toHaveBeenCalledTimes(2);
      });

      const recreatedTracks = tracksByElement.get(video) ?? [];
      expect(recreatedTracks).toHaveLength(1);
      expect(recreatedTracks[0]?.mode).toBe("showing");
      expect(recreatedTracks[0]?.cues).toHaveLength(1);
    } finally {
      fetchSpy.mockRestore();
      if (originalCue == null) {
        Reflect.deleteProperty(window as Window & { VTTCue?: typeof window.VTTCue }, "VTTCue");
      } else {
        Object.defineProperty(window, "VTTCue", {
          configurable: true,
          writable: true,
          value: originalCue,
        });
      }
      if (originalTextTracksDescriptor) {
        Object.defineProperty(
          HTMLMediaElement.prototype,
          "textTracks",
          originalTextTracksDescriptor,
        );
      } else {
        Reflect.deleteProperty(
          HTMLMediaElement.prototype as HTMLMediaElement & { textTracks?: TextTrackList },
          "textTracks",
        );
      }
      if (originalAddTextTrack) {
        Object.defineProperty(HTMLMediaElement.prototype, "addTextTrack", {
          configurable: true,
          value: originalAddTextTrack,
        });
      } else {
        Reflect.deleteProperty(
          HTMLMediaElement.prototype as HTMLMediaElement & {
            addTextTrack?: HTMLMediaElement["addTextTrack"];
          },
          "addTextTrack",
        );
      }
    }
  });

  it("keeps the subtitle picker visible even before subtitle tracks are known", async () => {
    renderDock();

    expect(await screen.findByRole("button", { name: "Subtitles" })).toBeTruthy();
  });

  it("refreshes playback tracks when opening subtitles with no usable tracks", async () => {
    const refreshSpy = vi.spyOn(api, "refreshPlaybackTracks").mockResolvedValue({
      subtitles: [{ id: 9, language: "eng", title: "English", format: "vtt" }],
      embeddedSubtitles: [],
      embeddedAudioTracks: [],
    });

    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      activeItem: {
        ...defaultPlaybackDockUsePlayer().activeItem,
        subtitles: [],
        embeddedSubtitles: [],
      },
    });

    renderDock();

    fireEvent.click(await screen.findByRole("button", { name: "Subtitles" }));

    await waitFor(() => {
      expect(refreshSpy).toHaveBeenCalledWith(42);
      expect(screen.getByRole("option", { name: /English/i })).toBeTruthy();
    });
  });

  it("shows inline subtitle loading without blocking the player overlay", async () => {
    const subtitleRequest = createDeferred<Response>();
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(() => subtitleRequest.promise);
    vi.spyOn(api, "listLibraries").mockResolvedValue([
      {
        id: 7,
        name: "Anime",
        type: "anime",
        path: "/anime",
        user_id: 1,
        preferred_audio_language: "en",
        preferred_subtitle_language: "en",
        subtitles_enabled_by_default: false,
      },
    ]);

    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      activeItem: {
        ...defaultPlaybackDockUsePlayer().activeItem,
        subtitles: [{ id: 9, language: "eng", title: "English", format: "vtt" }],
      },
    });

    try {
      const { container } = renderDock();
      const video = container.querySelector("video") as HTMLVideoElement | null;
      expect(video).toBeTruthy();
      if (!video) {
        throw new Error("Expected a video element");
      }

      fireEvent.canPlay(video);

      await waitFor(() => {
        expect(screen.queryByRole("status", { name: "Preparing playback..." })).toBeNull();
      });

      fireEvent.click(await screen.findByRole("button", { name: "Subtitles" }));
      fireEvent.click(screen.getByRole("option", { name: /English/ }));

      expect(screen.getByText("Loading subtitles...")).toBeTruthy();
      expect(screen.queryByRole("status", { name: "Loading subtitles..." })).toBeNull();

      subtitleRequest.resolve(
        new Response("WEBVTT\n\n00:00:00.000 --> 00:00:02.000\nHello world\n", { status: 200 }),
      );

      await waitFor(() => {
        expect(screen.queryByText("Loading subtitles...")).toBeNull();
      });
    } finally {
      fetchSpy.mockRestore();
    }
  });

  it("keeps a manual subtitle choice when queued video advances", async () => {
    const queue = [
      {
        ...defaultPlaybackDockUsePlayer().activeItem,
        id: 42,
        subtitles: [{ id: 9, language: "eng", title: "English", format: "vtt" }],
      },
      {
        ...defaultPlaybackDockUsePlayer().activeItem,
        id: 43,
        subtitles: [{ id: 10, language: "eng", title: "English", format: "vtt" }],
      },
    ];
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockImplementation((input) => {
      const url = String(input);
      return Promise.resolve(
        new Response(
          `WEBVTT\n\n00:00:00.000 --> 00:00:02.000\n${url.includes("/10") ? "Episode two" : "Episode one"}\n`,
          { status: 200 },
        ),
      );
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
        subtitles_enabled_by_default: false,
      },
    ]);
    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      activeItem: queue[0],
      queue,
      queueIndex: 0,
    });

    try {
      const { queryClient, rerender } = renderDock();

      fireEvent.click(await screen.findByRole("button", { name: "Subtitles" }));
      fireEvent.click(screen.getByRole("option", { name: /English/ }));

      await waitFor(() => {
        expect(fetchSpy).toHaveBeenCalledWith(
          expect.stringContaining("/api/subtitles/9"),
          expect.any(Object),
        );
      });

      mockUsePlayer.mockReturnValue({
        ...defaultPlaybackDockUsePlayer(),
        activeItem: queue[1],
        queue,
        queueIndex: 1,
      });

      rerender(
        <QueryClientProvider client={queryClient}>
          <PlaybackDock />
        </QueryClientProvider>,
      );

      await waitFor(() => {
        expect(fetchSpy).toHaveBeenCalledWith(
          expect.stringContaining("/api/subtitles/10"),
          expect.any(Object),
        );
      });
    } finally {
      fetchSpy.mockRestore();
    }
  });

  it("keeps subtitle load failures non-blocking", async () => {
    vi.spyOn(api, "listLibraries").mockResolvedValue([
      {
        id: 7,
        name: "Anime",
        type: "anime",
        path: "/anime",
        user_id: 1,
        preferred_audio_language: "en",
        preferred_subtitle_language: "en",
        subtitles_enabled_by_default: false,
      },
    ]);
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(new Response("nope", { status: 500 }));
    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      activeItem: {
        ...defaultPlaybackDockUsePlayer().activeItem,
        subtitles: [{ id: 9, language: "eng", title: "English", format: "vtt" }],
      },
    });

    try {
      renderDock();

      fireEvent.click(await screen.findByRole("button", { name: "Subtitles" }));
      fireEvent.click(screen.getByRole("option", { name: /English/ }));

      expect(screen.getByText("Loading subtitles...")).toBeTruthy();
      expect(screen.queryByRole("status", { name: "Loading subtitles..." })).toBeNull();

      await waitFor(() => {
        expect(screen.queryByText("Loading subtitles...")).toBeNull();
        expect(screen.getByText("Subtitle load failed. Try again.")).toBeTruthy();
      });
    } finally {
      fetchSpy.mockRestore();
    }
  });

  it("keeps a timed out subtitle selected and allows retrying the same track", async () => {
    const refreshSpy = vi.spyOn(api, "refreshPlaybackTracks").mockResolvedValue({
      subtitles: [],
      embeddedSubtitles: [
        {
          streamIndex: 7,
          language: "eng",
          title: "English Signs",
          supported: true,
        },
      ],
      embeddedAudioTracks: [],
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
        subtitles_enabled_by_default: false,
      },
    ]);
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockRejectedValueOnce(new Error("Subtitle request timed out"))
      .mockResolvedValueOnce(
        new Response("WEBVTT\n\n00:00:00.000 --> 00:00:02.000\nHello again\n", { status: 200 }),
      );
    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      activeItem: {
        ...defaultPlaybackDockUsePlayer().activeItem,
        id: 42,
        embeddedSubtitles: [{ streamIndex: 7, language: "eng", title: "English Signs" }],
      },
    });

    try {
      renderDock();

      fireEvent.click(await screen.findByRole("button", { name: "Subtitles" }));
      fireEvent.click(screen.getByRole("option", { name: /English Signs/i }));

      expect(screen.getByText("Loading subtitles...")).toBeTruthy();

      await waitFor(() => {
        expect(screen.queryByText("Loading subtitles...")).toBeNull();
        expect(screen.getByText("Subtitle load timed out. Try again.")).toBeTruthy();
      });

      fireEvent.click(screen.getByRole("button", { name: "Subtitles" }));
      expect(
        screen.getByRole("option", { name: /English Signs/i }),
      ).toHaveAttribute("aria-selected", "true");
      fireEvent.click(screen.getByRole("option", { name: /English Signs/i }));

      await waitFor(() => {
        expect(refreshSpy).toHaveBeenCalledWith(42);
        expect(fetchSpy).toHaveBeenCalledTimes(2);
      });
      await waitFor(() => {
        expect(screen.queryByText("Subtitle load timed out. Try again.")).toBeNull();
      });
    } finally {
      fetchSpy.mockRestore();
    }
  });

  it("shows unsupported embedded subtitles as unavailable and skips auto-selection", async () => {
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
    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      activeItem: {
        ...defaultPlaybackDockUsePlayer().activeItem,
        embeddedSubtitles: [
          {
            streamIndex: 7,
            language: "eng",
            title: "English PGS",
            supported: false,
          },
        ],
      },
    });

    renderDock();

    const subtitleButton = await screen.findByRole("button", { name: "Subtitles" });
    expect(subtitleButton.className.includes("is-active")).toBe(false);

    fireEvent.click(subtitleButton);

    const unavailableOption = screen.getByRole("option", { name: /English PGS \(Unavailable\)/i });
    expect(unavailableOption).toBeDisabled();
    expect(unavailableOption).toHaveAttribute("aria-selected", "false");
  });

  it("syncs the selected audio menu state from the active playback audio index", async () => {
    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      videoAudioIndex: 2,
    });

    renderDock();

    const audioButton = await screen.findByRole("button", { name: /Audio track: Japanese/i });
    expect(audioButton).toBeTruthy();
  });

  it("does not re-request the same audio switch after the session reports the new audio index", async () => {
    const { container, queryClient, rerender } = renderDock();
    const video = container.querySelector("video") as HTMLVideoElement | null;
    expect(video).toBeTruthy();
    if (!video) {
      throw new Error("Expected a video element");
    }

    const browserAudioTracks = [{ enabled: true }, { enabled: false }];
    Object.defineProperty(video, "audioTracks", {
      configurable: true,
      value: browserAudioTracks,
    });

    fireEvent.loadedMetadata(video);

    fireEvent.click(await screen.findByRole("button", { name: /Audio track:/i }));
    fireEvent.click(screen.getByRole("option", { name: /Japanese/i }));

    await waitFor(() => {
      expect(mockChangeAudioTrack).toHaveBeenCalledTimes(1);
      expect(mockChangeAudioTrack).toHaveBeenCalledWith(2);
    });

    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      videoAudioIndex: 2,
      activeItem: {
        ...defaultPlaybackDockUsePlayer().activeItem,
        embeddedAudioTracks: [
          { streamIndex: 1, language: "eng", title: "English" },
          { streamIndex: 2, language: "jpn", title: "Japanese" },
        ],
      },
      videoSourceUrl:
        "http://localhost:3000/api/playback/sessions/session-1/revisions/2/index.m3u8",
    });

    rerender(
      <QueryClientProvider client={queryClient}>
        <PlaybackDock />
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Audio track: Japanese/i })).toBeTruthy();
    });
    expect(mockChangeAudioTrack).toHaveBeenCalledTimes(1);
  });

  it("restarts the current video when Previous is pressed after the restart threshold", async () => {
    const seekTo = vi.fn();
    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      queue: [
        { ...defaultPlaybackDockUsePlayer().activeItem, id: 41 },
        { ...defaultPlaybackDockUsePlayer().activeItem, id: 42 },
      ],
      queueIndex: 1,
      seekTo,
    });

    const { container } = renderDock();
    const video = container.querySelector("video") as HTMLVideoElement | null;
    expect(video).toBeTruthy();
    if (!video) {
      throw new Error("Expected a video element");
    }

    setVideoCurrentTime(video, 12);
    fireEvent.click(screen.getByRole("button", { name: "Previous episode" }));

    expect(seekTo).toHaveBeenCalledWith(0);
  });

  it("goes to the previous queue item when Previous is pressed near the start", async () => {
    const playPreviousInQueue = vi.fn();
    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      queue: [
        { ...defaultPlaybackDockUsePlayer().activeItem, id: 41 },
        { ...defaultPlaybackDockUsePlayer().activeItem, id: 42 },
      ],
      queueIndex: 1,
      playPreviousInQueue,
    });

    const { container } = renderDock();
    const video = container.querySelector("video") as HTMLVideoElement | null;
    expect(video).toBeTruthy();
    if (!video) {
      throw new Error("Expected a video element");
    }

    setVideoCurrentTime(video, 2);
    fireEvent.click(screen.getByRole("button", { name: "Previous episode" }));

    expect(playPreviousInQueue).toHaveBeenCalledTimes(1);
  });

  it("advances to the next queue item from the theater controls", async () => {
    const playNextInQueue = vi.fn();
    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      queue: [
        { ...defaultPlaybackDockUsePlayer().activeItem, id: 42 },
        { ...defaultPlaybackDockUsePlayer().activeItem, id: 43 },
      ],
      queueIndex: 0,
      playNextInQueue,
    });

    renderDock();
    fireEvent.click(screen.getByRole("button", { name: "Next episode" }));

    expect(playNextInQueue).toHaveBeenCalledTimes(1);
  });

  it("shows up next on video end when autoplay next is enabled and play now advances", async () => {
    const playNextInQueue = vi.fn();
    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      queue: [
        { ...defaultPlaybackDockUsePlayer().activeItem, id: 42 },
        { ...defaultPlaybackDockUsePlayer().activeItem, id: 43 },
      ],
      queueIndex: 0,
      playNextInQueue,
    });

    const { container } = renderDock();
    const video = container.querySelector("video") as HTMLVideoElement | null;
    expect(video).toBeTruthy();
    if (!video) {
      throw new Error("Expected a video element");
    }

    fireEvent.ended(video);

    expect(await screen.findByRole("dialog", { name: "Up next" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Play now" }));

    await waitFor(() => {
      expect(playNextInQueue).toHaveBeenCalledTimes(1);
    });
  });

  it("does not advance on video end when autoplay next is disabled", async () => {
    const playNextInQueue = vi.fn();
    window.localStorage.setItem(videoAutoplayStorageKey, "false");
    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      queue: [
        { ...defaultPlaybackDockUsePlayer().activeItem, id: 42 },
        { ...defaultPlaybackDockUsePlayer().activeItem, id: 43 },
      ],
      queueIndex: 0,
      playNextInQueue,
    });

    const { container } = renderDock();
    const video = container.querySelector("video") as HTMLVideoElement | null;
    expect(video).toBeTruthy();
    if (!video) {
      throw new Error("Expected a video element");
    }

    fireEvent.ended(video);

    await waitFor(() => {
      expect(playNextInQueue).not.toHaveBeenCalled();
    });
  });

  it("persists the autoplay next preference when toggled", async () => {
    mockUsePlayer.mockReturnValue({
      ...defaultPlaybackDockUsePlayer(),
      queue: [
        { ...defaultPlaybackDockUsePlayer().activeItem, id: 42 },
        { ...defaultPlaybackDockUsePlayer().activeItem, id: 43 },
      ],
      queueIndex: 0,
    });

    renderDock();
    fireEvent.click(screen.getByRole("button", { name: "Autoplay next episode" }));

    await waitFor(() => {
      expect(window.localStorage.getItem(videoAutoplayStorageKey)).toBe("false");
    });
  });
});
