import { describe, expect, it } from "vitest";
import type { SubtitleTrackOption } from "./playerMedia";
import {
  getPreferredSubtitleKey,
  sortSubtitleTrackOptions,
} from "./playerMedia";

function track(
  overrides: Partial<SubtitleTrackOption> & { key: string; srcLang: string },
): SubtitleTrackOption {
  return {
    label: overrides.label ?? overrides.srcLang,
    src: "",
    supported: true,
    ...overrides,
  };
}

describe("getPreferredSubtitleKey", () => {
  it("returns 'off' when subtitles are disabled", () => {
    const tracks = [track({ key: "emb-0", srcLang: "eng", label: "English" })];
    expect(getPreferredSubtitleKey(tracks, "en", false)).toBe("off");
  });

  it("returns 'off' when no language preference", () => {
    const tracks = [track({ key: "emb-0", srcLang: "eng", label: "English" })];
    expect(getPreferredSubtitleKey(tracks, "", true)).toBe("off");
  });

  it("returns 'off' when no language match exists", () => {
    const tracks = [track({ key: "emb-0", srcLang: "jpn", label: "Japanese" })];
    expect(getPreferredSubtitleKey(tracks, "en", true)).toBe("off");
  });

  it("prefers non-burn text track over burn-in PGS for the same language", () => {
    const tracks = [
      track({
        key: "emb-0",
        srcLang: "eng",
        label: "English",
        requiresBurn: true,
      }),
      track({ key: "emb-1", srcLang: "eng", label: "English" }),
    ];
    expect(getPreferredSubtitleKey(tracks, "en", true)).toBe("emb-1");
  });

  it("prefers non-burn even when burn track comes first", () => {
    const tracks = [
      track({
        key: "emb-0",
        srcLang: "eng",
        label: "English PGS",
        requiresBurn: true,
      }),
      track({ key: "emb-1", srcLang: "eng", label: "English SRT" }),
    ];
    expect(getPreferredSubtitleKey(tracks, "en", true)).toBe("emb-1");
  });

  it("falls back to burn-in track when no non-burn match exists", () => {
    const tracks = [
      track({
        key: "emb-0",
        srcLang: "eng",
        label: "English PGS",
        requiresBurn: true,
      }),
      track({ key: "emb-1", srcLang: "jpn", label: "Japanese" }),
    ];
    expect(getPreferredSubtitleKey(tracks, "en", true)).toBe("emb-0");
  });

  it("returns the first non-burn match when multiple non-burn tracks match", () => {
    const tracks = [
      track({ key: "emb-0", srcLang: "eng", label: "English SDH" }),
      track({ key: "emb-1", srcLang: "eng", label: "English" }),
    ];
    expect(getPreferredSubtitleKey(tracks, "en", true)).toBe("emb-0");
  });

  describe("with label hint", () => {
    it("prefers non-burn hint match over burn hint match", () => {
      const tracks = [
        track({
          key: "emb-0",
          srcLang: "eng",
          label: "English PGS",
          requiresBurn: true,
        }),
        track({ key: "emb-1", srcLang: "eng", label: "English SRT" }),
        track({ key: "emb-2", srcLang: "eng", label: "English SDH" }),
      ];
      expect(getPreferredSubtitleKey(tracks, "en", true, "SRT")).toBe("emb-1");
    });

    it("falls back to burn hint match when no non-burn hint match exists", () => {
      const tracks = [
        track({
          key: "emb-0",
          srcLang: "eng",
          label: "English PGS",
          requiresBurn: true,
        }),
        track({ key: "emb-1", srcLang: "eng", label: "English SDH" }),
      ];
      expect(getPreferredSubtitleKey(tracks, "en", true, "PGS")).toBe("emb-0");
    });

    it("falls back to non-hint language match when hint matches nothing", () => {
      const tracks = [
        track({
          key: "emb-0",
          srcLang: "eng",
          label: "English",
          requiresBurn: true,
        }),
        track({ key: "emb-1", srcLang: "eng", label: "English SRT" }),
      ];
      expect(getPreferredSubtitleKey(tracks, "en", true, "Commentary")).toBe(
        "emb-1",
      );
    });
  });

  it("skips unsupported tracks", () => {
    const tracks = [
      track({
        key: "emb-0",
        srcLang: "eng",
        label: "English",
        supported: false,
      }),
      track({
        key: "emb-1",
        srcLang: "eng",
        label: "English PGS",
        requiresBurn: true,
      }),
    ];
    expect(getPreferredSubtitleKey(tracks, "en", true)).toBe("emb-1");
  });

  it("handles burn-only catalog (all tracks require burn)", () => {
    const tracks = [
      track({
        key: "emb-0",
        srcLang: "eng",
        label: "English PGS",
        requiresBurn: true,
      }),
      track({
        key: "emb-1",
        srcLang: "eng",
        label: "English PGS 2",
        requiresBurn: true,
      }),
    ];
    expect(getPreferredSubtitleKey(tracks, "en", true)).toBe("emb-0");
  });

  it("prefers normal text over hearing-impaired when no hint is present", () => {
    const tracks = [
      track({
        key: "emb-0",
        srcLang: "eng",
        label: "English • SDH",
        hearingImpaired: true,
      }),
      track({ key: "emb-1", srcLang: "eng", label: "English" }),
    ];
    expect(getPreferredSubtitleKey(tracks, "en", true)).toBe("emb-1");
  });

  it("prefers hearing-impaired when the hint requests it", () => {
    const tracks = [
      track({
        key: "emb-0",
        srcLang: "eng",
        label: "English • SDH",
        hearingImpaired: true,
      }),
      track({ key: "emb-1", srcLang: "eng", label: "English" }),
    ];
    expect(getPreferredSubtitleKey(tracks, "en", true, "SDH")).toBe("emb-0");
  });

  it("prefers forced subtitles only when the hint requests them", () => {
    const tracks = [
      track({
        key: "emb-0",
        srcLang: "eng",
        label: "English • Forced",
        forced: true,
      }),
      track({ key: "emb-1", srcLang: "eng", label: "English" }),
    ];
    expect(getPreferredSubtitleKey(tracks, "en", true)).toBe("emb-1");
    expect(getPreferredSubtitleKey(tracks, "en", true, "Forced")).toBe("emb-0");
  });
});

describe("sortSubtitleTrackOptions", () => {
  it("orders normal text before forced, hearing-impaired, and burn-in tracks", () => {
    const tracks = sortSubtitleTrackOptions([
      track({
        key: "emb-3",
        srcLang: "eng",
        label: "English PGS",
        requiresBurn: true,
      }),
      track({
        key: "emb-2",
        srcLang: "eng",
        label: "English • SDH",
        hearingImpaired: true,
      }),
      track({
        key: "emb-1",
        srcLang: "eng",
        label: "English • Forced",
        forced: true,
      }),
      track({ key: "emb-0", srcLang: "eng", label: "English" }),
    ]);
    expect(tracks.map((entry) => entry.key)).toEqual([
      "emb-0",
      "emb-1",
      "emb-2",
      "emb-3",
    ]);
  });
});
