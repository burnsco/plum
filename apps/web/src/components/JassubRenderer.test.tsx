import { render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { JassubRenderer } from "./JassubRenderer";

const { mockInstance, JASSUB_CONSTRUCTOR } = vi.hoisted(() => {
  const mockInstance = {
    destroy: vi.fn<() => void>(),
  };
  const JASSUB_CONSTRUCTOR = vi.fn<(_opts: unknown) => typeof mockInstance>(
    function (_opts: unknown) {
      return mockInstance;
    },
  );
  return { mockInstance, JASSUB_CONSTRUCTOR };
});

vi.mock("jassub", () => ({
  default: JASSUB_CONSTRUCTOR,
}));

vi.mock("jassub/dist/wasm/jassub-worker.js?url", () => ({
  default: "worker-url",
}));

vi.mock("jassub/dist/wasm/jassub-worker.wasm?url", () => ({
  default: "wasm-url",
}));

describe("JassubRenderer", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  afterEach(() => {
    mockInstance.destroy.mockReset();
    JASSUB_CONSTRUCTOR.mockClear();
  });

  it("enables font lookup for styled anime ASS subtitles", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response("Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,Hello", {
        status: 200,
      }),
    );

    render(
      <JassubRenderer
        videoElement={document.createElement("video")}
        assSrc="/api/subtitles/1/ass"
        fontUrls={["/api/media/1/attachments/0"]}
      />,
    );

    await waitFor(() => {
      expect(JASSUB_CONSTRUCTOR).toHaveBeenCalled();
    });

    expect(JASSUB_CONSTRUCTOR).toHaveBeenCalledWith(
      expect.objectContaining({
        queryFonts: "localandremote",
        fonts: ["/api/media/1/attachments/0"],
        workerUrl: "worker-url",
        wasmUrl: "wasm-url",
      }),
    );
  });
});
