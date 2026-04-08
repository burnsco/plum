import { renderHook, act, waitFor } from "@testing-library/react";
import type { Dispatch, SetStateAction } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SubtitleTrackOption } from "../../lib/playback/playerMedia";
import { useSubtitleTransport } from "./useSubtitleTransport";

function track(overrides: Partial<SubtitleTrackOption> = {}): SubtitleTrackOption {
  return {
    key: "ext:1",
    logicalId: "ext:1",
    origin: "external",
    label: "English",
    src: "/api/subtitles/1",
    srcLang: "en",
    supported: true,
    deliveryModes: [{ mode: "direct_vtt", requiresReload: false }],
    preferredWebDeliveryMode: "direct_vtt",
    ...overrides,
  };
}

type LoadedTrack = SubtitleTrackOption & { body: string };

function renderTransport(trackRequests: SubtitleTrackOption[]) {
  const subtitleLoadControllersRef = { current: new Map<string, AbortController>() };
  const blockedSubtitleRetryKeysRef = { current: new Set<string>() };
  const setPendingSubtitleKey = vi.fn<Dispatch<SetStateAction<string | null>>>();
  const setLoadedSubtitleTracks = vi.fn<Dispatch<SetStateAction<LoadedTrack[]>>>();
  const setSubtitleStatusMessage = vi.fn<Dispatch<SetStateAction<string>>>();

  const hook = renderHook(() =>
    useSubtitleTransport({
      activeMediaId: 42,
      loadedSubtitleTracks: [],
      subtitleTrackRequests: trackRequests,
      subtitleLoadControllersRef,
      blockedSubtitleRetryKeysRef,
      setPendingSubtitleKey,
      setLoadedSubtitleTracks,
      setSubtitleStatusMessage,
    }),
  );

  return {
    ...hook,
    subtitleLoadControllersRef,
    blockedSubtitleRetryKeysRef,
    setPendingSubtitleKey,
    setLoadedSubtitleTracks,
    setSubtitleStatusMessage,
  };
}

describe("useSubtitleTransport", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("marks unsupported tracks as unavailable without fetching", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");
    const { result, setSubtitleStatusMessage } = renderTransport([track({ supported: false })]);

    await act(async () => {
      await result.current.ensureSubtitleTrackLoaded("ext:1");
    });

    expect(fetchSpy).not.toHaveBeenCalled();
    expect(result.current.subtitleLoadStateByKey["ext:1"]).toBe("error");
    expect(setSubtitleStatusMessage).toHaveBeenCalledWith("This subtitle track is unavailable.");
  });

  it("consumes streamed VTT bodies into loaded track state", async () => {
    const encoder = new TextEncoder();
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        new ReadableStream({
          start(controller) {
            controller.enqueue(
              encoder.encode("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello\n\n"),
            );
            setTimeout(() => {
              controller.enqueue(encoder.encode("00:00:03.000 --> 00:00:04.000\nWorld\n"));
              controller.close();
            }, 0);
          },
        }),
        {
          headers: { "Content-Type": "text/vtt" },
          status: 200,
        },
      ),
    );
    const { result, setLoadedSubtitleTracks } = renderTransport([track()]);

    await act(async () => {
      await result.current.ensureSubtitleTrackLoaded("ext:1");
    });

    expect(result.current.subtitleLoadStateByKey["ext:1"]).toBe("ready");
    expect(setLoadedSubtitleTracks.mock.calls.length).toBeGreaterThanOrEqual(1);
    const finalUpdater = setLoadedSubtitleTracks.mock.calls.at(-1)?.[0] as (
      current: LoadedTrack[],
    ) => LoadedTrack[];
    const finalTracks = finalUpdater([]);
    expect(finalTracks[0]?.body).toContain("Hello");
    expect(finalTracks[0]?.body).toContain("World");
  });

  it("marks timed out requests as blocked and timeout", async () => {
    vi.useFakeTimers();
    vi.spyOn(globalThis, "fetch").mockImplementation(
      (_input, init) =>
        new Promise<Response>((_resolve, reject) => {
          init?.signal?.addEventListener("abort", () => {
            reject(new DOMException("Aborted", "AbortError"));
          });
        }),
    );
    const { result, blockedSubtitleRetryKeysRef, setSubtitleStatusMessage } = renderTransport([
      track({
        key: "emb:7",
        logicalId: "emb:7",
        origin: "embedded",
        src: "/api/media/42/subtitles/embedded/7",
      }),
    ]);

    const pending = act(async () => {
      const promise = result.current.ensureSubtitleTrackLoaded("emb:7");
      await vi.advanceTimersByTimeAsync(600_000);
      await promise;
    });
    await pending;

    expect(result.current.subtitleLoadStateByKey["emb:7"]).toBe("timeout");
    expect(blockedSubtitleRetryKeysRef.current.has("emb:7")).toBe(true);
    expect(setSubtitleStatusMessage).toHaveBeenCalledWith("Subtitle load timed out. Try again.");
  });

  it("returns blocked state for keys that previously timed out or failed", async () => {
    const { result, blockedSubtitleRetryKeysRef } = renderTransport([track()]);
    blockedSubtitleRetryKeysRef.current.add("ext:1");

    await act(async () => {
      await result.current.ensureSubtitleTrackLoaded("ext:1");
    });

    await waitFor(() => {
      expect(result.current.subtitleLoadStateByKey["ext:1"]).toBe("blocked");
    });
  });
});
