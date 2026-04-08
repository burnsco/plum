import { describe, expect, it } from "vitest";
import {
  buildSubtitleTrackRequests,
  embeddedStreamIndexFromKey,
} from "./playbackDockSubtitleTracks";

describe("buildSubtitleTrackRequests", () => {
  it("uses wire logical ids as subtitle keys", () => {
    const tracks = buildSubtitleTrackRequests({
      mediaId: 42,
      subtitles: [
        {
          id: 7,
          logicalId: "ext:7",
          language: "en",
          title: "English",
          format: "srt",
        },
      ],
      embeddedSubtitles: [
        {
          streamIndex: 3,
          logicalId: "emb:3",
          language: "ja",
          title: "Japanese",
          codec: "subrip",
        },
      ],
    });

    expect(tracks.map((track) => track.key)).toContain("ext:7");
    expect(tracks.map((track) => track.key)).toContain("emb:3");
  });
});

describe("embeddedStreamIndexFromKey", () => {
  it("resolves embedded stream indexes from logical ids", () => {
    expect(embeddedStreamIndexFromKey("emb:9")).toBe(9);
    expect(embeddedStreamIndexFromKey("ext:9")).toBeNull();
  });
});
