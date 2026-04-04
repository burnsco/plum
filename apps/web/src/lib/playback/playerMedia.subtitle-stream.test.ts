import { describe, expect, it, vi } from "vitest";
import { consumeSubtitleResponseWithPartialUpdates, streamingVttPrefixForParse } from "./playerMedia";

describe("streamingVttPrefixForParse", () => {
  it("returns the full accumulated body while streaming (first cue may lack trailing blank line)", () => {
    expect(streamingVttPrefixForParse("WEBVTT", false)).toBe("WEBVTT");
    const midCue = "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello";
    expect(streamingVttPrefixForParse(midCue, false)).toBe(midCue);
  });

  it("keeps later cues in the buffer even when the last cue is still open", () => {
    const partial =
      "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHello\n\n00:00:03.000 --> 00:00:04.000\nWor";
    expect(streamingVttPrefixForParse(partial, false)).toBe(partial);
  });

  it("returns full body when the stream is done", () => {
    const full = "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nHi\n";
    expect(streamingVttPrefixForParse(full, true)).toBe(full);
  });
});

describe("consumeSubtitleResponseWithPartialUpdates", () => {
  it("streams chunks and ends with the full normalized body", async () => {
    const encoder = new TextEncoder();
    const chunk1 = encoder.encode("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nA\n\n");
    const chunk2 = encoder.encode("00:00:03.000 --> 00:00:04.000\nB\n\n");
    const stream = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(chunk1);
        controller.enqueue(chunk2);
        controller.close();
      },
    });
    const response = new Response(stream, { status: 200 });
    const partials: string[] = [];
    const signal = new AbortController().signal;
    const finalText = await consumeSubtitleResponseWithPartialUpdates(
      response,
      signal,
      (body, done) => {
        partials.push(`${done ? "done" : "partial"}:${body.length}`);
      },
      0,
    );
    expect(finalText).toContain("00:00:01.000");
    expect(finalText).toContain("00:00:03.000");
    expect(partials.some((p) => p.startsWith("done:"))).toBe(true);
  });

  it("falls back to response.text when body is null", async () => {
    const response = {
      ok: true,
      body: null,
      text: vi.fn().mockResolvedValue("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nX\n"),
    } as unknown as Response;
    const signal = new AbortController().signal;
    let last = "";
    const out = await consumeSubtitleResponseWithPartialUpdates(
      response,
      signal,
      (b) => {
        last = b;
      },
      0,
    );
    expect(response.text).toHaveBeenCalledOnce();
    expect(out).toContain("00:00:01.000");
    expect(last).toContain("WEBVTT");
  });
});
