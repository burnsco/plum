import { render, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { JassubRenderer } from "./JassubRenderer";

vi.mock("jassub", () => ({
  default: vi.fn<() => { destroy: () => void }>().mockImplementation(() => ({
    destroy: vi.fn<() => void>(),
  })),
}));

vi.mock("jassub/dist/wasm/jassub-worker.js?url", () => ({
  default: "/assets/jassub-worker.js",
}));

vi.mock("jassub/dist/wasm/jassub-worker.wasm?url", () => ({
  default: "/assets/jassub-worker.wasm",
}));

function pendingFetchMock() {
  return vi.fn<
    (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>
  >((_input, init) => {
    const signal = init?.signal;
    return new Promise<Response>((_resolve, reject) => {
      signal?.addEventListener(
        "abort",
        () => reject(new DOMException("Aborted", "AbortError")),
        { once: true },
      );
    });
  });
}

describe("JassubRenderer", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("keeps an active ASS fetch when equivalent font URLs are passed on rerender", async () => {
    const fetchMock = pendingFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    const video = document.createElement("video");

    const firstStatusChange =
      vi.fn<(status: "loading" | "ready" | "error" | "timeout") => void>();
    const { rerender, unmount } = render(
      <JassubRenderer
        videoElement={video}
        assSrc="/api/media/1/subtitles/embedded/2/ass"
        fontUrls={["/api/media/1/attachments/font.ttf"]}
        onStatusChange={firstStatusChange}
      />,
    );

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    const firstSignal = fetchMock.mock.calls[0]?.[1]?.signal;
    expect(firstSignal).toBeInstanceOf(AbortSignal);
    expect(firstSignal?.aborted).toBe(false);

    rerender(
      <JassubRenderer
        videoElement={video}
        assSrc="/api/media/1/subtitles/embedded/2/ass"
        fontUrls={["/api/media/1/attachments/font.ttf"]}
        onStatusChange={vi.fn<
          (status: "loading" | "ready" | "error" | "timeout") => void
        >()}
      />,
    );

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(firstSignal?.aborted).toBe(false);

    unmount();
    expect(firstSignal?.aborted).toBe(true);
  });
});
