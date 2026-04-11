import { describe, expect, it } from "vitest";
import {
  buildAssFontUrls,
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

  it("prefers direct vtt over ass for embedded subrip when session omits preferred mode", () => {
    const tracks = buildSubtitleTrackRequests({
      mediaId: 99,
      embeddedSubtitles: [
        {
          streamIndex: 3,
          language: "en",
          title: "English",
          codec: "subrip",
          assEligible: true,
        },
      ],
    });
    const emb = tracks.find((t) => t.key === "emb:3");
    expect(emb?.preferredWebDeliveryMode).toBe("direct_vtt");
    expect(emb?.assSrc).toMatch(/\/ass$/);
  });

  it("prefers ass for native embedded ass when session omits preferred mode", () => {
    const tracks = buildSubtitleTrackRequests({
      mediaId: 99,
      embeddedSubtitles: [
        {
          streamIndex: 5,
          language: "en",
          title: "Styled",
          codec: "ass",
          assEligible: true,
        },
      ],
    });
    const emb = tracks.find((t) => t.key === "emb:5");
    expect(emb?.preferredWebDeliveryMode).toBe("ass");
  });
});

describe("buildAssFontUrls", () => {
  it("keeps ASS font attachments by MIME type or extension", () => {
    const urls = buildAssFontUrls("http://plum.test", [
      {
        streamIndex: 7,
        fileName: "Fancy Font.otf",
        mimeType: "",
        codec: "otf",
        deliveryUrl: "/api/media/42/attachments/7",
      },
      {
        streamIndex: 8,
        fileName: "cover.jpg",
        mimeType: "image/jpeg",
        codec: "mjpeg",
        deliveryUrl: "/api/media/42/attachments/8",
      },
      {
        streamIndex: 9,
        fileName: "font.bin",
        mimeType: "FONT/WOFF2",
        codec: "woff2",
        deliveryUrl: "/api/media/42/attachments/9",
      },
    ]);

    expect(urls).toEqual([
      "http://plum.test/api/media/42/attachments/7",
      "http://plum.test/api/media/42/attachments/9",
    ]);
  });
});

describe("embeddedStreamIndexFromKey", () => {
  it("resolves embedded stream indexes from logical ids", () => {
    expect(embeddedStreamIndexFromKey("emb:9")).toBe(9);
    expect(embeddedStreamIndexFromKey("ext:9")).toBeNull();
  });
});
