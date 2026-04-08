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

  it("threads embedded font attachments into ASS tracks", () => {
    const tracks = buildSubtitleTrackRequests({
      mediaId: 42,
      embeddedSubtitles: [
        {
          streamIndex: 3,
          logicalId: "emb:3",
          language: "ja",
          title: "Japanese",
          codec: "ass",
          assEligible: true,
        },
      ],
      embeddedFontAttachments: [
        { index: 0, streamIndex: 9, filename: "GandhiSans-Regular.ttf" },
      ],
    });

    expect(tracks[0]?.fontUrls).toEqual(["/api/media/42/attachments/0"]);
  });

  it("threads embedded font attachments into external ASS tracks", () => {
    const tracks = buildSubtitleTrackRequests({
      mediaId: 42,
      subtitles: [
        {
          id: 7,
          logicalId: "ext:7",
          language: "en",
          title: "English",
          format: "ass",
        },
      ],
      embeddedFontAttachments: [
        { index: 0, streamIndex: 9, filename: "GandhiSans-Regular.ttf" },
      ],
    });

    expect(tracks[0]?.fontUrls).toEqual(["/api/media/42/attachments/0"]);
  });
});

describe("embeddedStreamIndexFromKey", () => {
  it("resolves embedded stream indexes from logical ids", () => {
    expect(embeddedStreamIndexFromKey("emb:9")).toBe(9);
    expect(embeddedStreamIndexFromKey("ext:9")).toBeNull();
  });
});
