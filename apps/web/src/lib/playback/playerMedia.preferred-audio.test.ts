import { describe, expect, it } from "vitest";
import type { AudioTrackOption } from "./playerMedia";
import { getPreferredAudioKey } from "./playerMedia";

function aud(
  overrides: Partial<AudioTrackOption> & { key: string },
): AudioTrackOption {
  const { key, label = "Audio", streamIndex = 0, language = "", ...rest } = overrides;
  return { ...rest, key, label, streamIndex, language };
}

describe("getPreferredAudioKey", () => {
  it("returns empty when no language preference", () => {
    const tracks = [aud({ key: "aud-1", language: "eng" })];
    expect(getPreferredAudioKey(tracks, "")).toBe("");
  });

  it("picks a track matching preferred language", () => {
    const tracks = [
      aud({ key: "aud-1", language: "eng", label: "English" }),
      aud({ key: "aud-2", language: "jpn", label: "Japanese", streamIndex: 2 }),
    ];
    expect(getPreferredAudioKey(tracks, "ja")).toBe("aud-2");
  });

  it("returns the first match when multiple tracks fit the preference", () => {
    const tracks = [
      aud({ key: "aud-1", language: "eng", label: "English" }),
      aud({ key: "aud-2", language: "en-US", label: "English (US)", streamIndex: 2 }),
    ];
    expect(getPreferredAudioKey(tracks, "en")).toBe("aud-1");
  });
});
