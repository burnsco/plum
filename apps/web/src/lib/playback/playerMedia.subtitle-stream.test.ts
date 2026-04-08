import { describe, expect, it, vi } from "vitest";
import type Hls from "hls.js";
import {
  consumeSubtitleResponseWithPartialUpdates,
  findHlsSubtitleTrackIndexForPlumKey,
  plumHlsSubtitlePlaylistFileForTrackKey,
  streamingVttPrefixForParse,
} from "./playerMedia";

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

describe("plumHlsSubtitlePlaylistFileForTrackKey", () => {
  it("maps emb and ext keys to virtual playlist filenames", () => {
    expect(plumHlsSubtitlePlaylistFileForTrackKey("emb-3")).toBe(
      "plum_subs_emb_3.m3u8",
    );
    expect(plumHlsSubtitlePlaylistFileForTrackKey("ext-12")).toBe(
      "plum_subs_ext_12.m3u8",
    );
    expect(plumHlsSubtitlePlaylistFileForTrackKey("off")).toBeNull();
  });
});

describe("findHlsSubtitleTrackIndexForPlumKey", () => {
  it("matches hls subtitle track url containing the virtual playlist name", () => {
    const hls = {
      subtitleTracks: [
        { url: "https://x.test/sessions/a/revisions/1/plum_subs_emb_2.m3u8" },
      ],
    } as unknown as Hls;
    expect(findHlsSubtitleTrackIndexForPlumKey(hls, "emb-2")).toBe(0);
    expect(findHlsSubtitleTrackIndexForPlumKey(hls, "emb-9")).toBe(-1);
  });
});

describe("consumeSubtitleResponseWithPartialUpdates", () => {
  it("streams chunks and ends with the full normalized body", async () => {
    const encoder = new TextEncoder();
    const chunk1 = encoder.encode(
      "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nA\n\n",
    );
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
      text: vi
        .fn<() => Promise<string>>()
        .mockResolvedValue("WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nX\n"),
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
