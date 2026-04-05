import { describe, expect, it } from "vitest";
import { clampVideoSeekSeconds, seekUpperBoundSeconds } from "./playerMedia";

function mockVideo(partial: {
  duration?: number;
  seekableRanges?: [number, number][];
}): HTMLVideoElement {
  const ranges = partial.seekableRanges ?? [];
  return {
    duration: partial.duration ?? 0,
    seekable: {
      get length() {
        return ranges.length;
      },
      start: (i: number) => ranges[i]?.[0] ?? 0,
      end: (i: number) => ranges[i]?.[1] ?? 0,
    },
  } as unknown as HTMLVideoElement;
}

describe("seekUpperBoundSeconds", () => {
  it("uses the minimum of catalog, element duration, and seekable end for transcode", () => {
    const v = mockVideo({
      duration: 400,
      seekableRanges: [[0, 380]],
    });
    expect(seekUpperBoundSeconds(v, 3600, 0, "transcode")).toBe(380);
  });

  it("matches transcode for remux delivery", () => {
    const v = mockVideo({
      duration: 400,
      seekableRanges: [[0, 380]],
    });
    expect(seekUpperBoundSeconds(v, 3600, 0, "remux")).toBe(380);
  });

  it("uses catalog and element duration when seekable is empty (transcode)", () => {
    const v = mockVideo({
      duration: 250,
      seekableRanges: [],
    });
    expect(seekUpperBoundSeconds(v, 600, 0, "transcode")).toBe(250);
  });

  it("uses max of catalog vs media for direct when both are known", () => {
    const v = mockVideo({
      duration: 90,
      seekableRanges: [[0, 90]],
    });
    expect(seekUpperBoundSeconds(v, 120, 0, "direct")).toBe(120);
  });
});

describe("clampVideoSeekSeconds", () => {
  it("clamps transcode seeks past the encoded edge", () => {
    const v = mockVideo({
      duration: 300,
      seekableRanges: [[0, 300]],
    });
    expect(clampVideoSeekSeconds(v, 9999, 7200, 0, "transcode")).toBeCloseTo(299.95, 1);
  });
});
