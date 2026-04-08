import { describe, expect, it } from "vitest";
import { resolveWebSubtitleSelection } from "./useSubtitleController";
import type { SubtitleTrackOption } from "../../lib/playback/playerMedia";

function track(overrides: Partial<SubtitleTrackOption> = {}): SubtitleTrackOption {
  return {
    key: "ext:1",
    logicalId: "ext:1",
    origin: "external",
    label: "English",
    src: "/api/subtitles/1",
    srcLang: "en",
    supported: true,
    deliveryModes: [{ mode: "direct_vtt", requiresReload: false }],
    preferredWebDeliveryMode: "direct_vtt",
    ...overrides,
  };
}

describe("resolveWebSubtitleSelection", () => {
  it("returns none for off", () => {
    expect(
      resolveWebSubtitleSelection({
        selectedSubtitleKey: "off",
        subtitleTrackRequests: [track()],
        subtitleLoadStateByKey: {},
        burnEmbeddedSubtitleStreamIndex: null,
        videoSourceIsHls: false,
        hls: null,
        resolutionVersion: 0,
      }),
    ).toMatchObject({
      renderer: "none",
      selectedDeliveryMode: null,
      manualTrackKey: null,
    });
  });

  it("prefers hls_native when the HLS track is present", () => {
    const hls = {
      subtitleTracks: [{ url: "/api/playback/sessions/s/revisions/1/plum_subs_6578743a31.m3u8" }],
    } as { subtitleTracks: Array<{ url: string }> };
    expect(
      resolveWebSubtitleSelection({
        selectedSubtitleKey: "ext:1",
        subtitleTrackRequests: [track()],
        subtitleLoadStateByKey: {},
        burnEmbeddedSubtitleStreamIndex: null,
        videoSourceIsHls: true,
        hls: hls as never,
        resolutionVersion: 1,
      }),
    ).toMatchObject({
      renderer: "hls_native",
      selectedDeliveryMode: "hls_vtt",
      manualTrackKey: null,
    });
  });

  it("falls back to manual_vtt when no HLS subtitle track exists", () => {
    expect(
      resolveWebSubtitleSelection({
        selectedSubtitleKey: "ext:1",
        subtitleTrackRequests: [track()],
        subtitleLoadStateByKey: { "ext:1": "loading" },
        burnEmbeddedSubtitleStreamIndex: null,
        videoSourceIsHls: true,
        hls: { subtitleTracks: [] } as never,
        resolutionVersion: 1,
      }),
    ).toMatchObject({
      renderer: "manual_vtt",
      selectedDeliveryMode: "direct_vtt",
      manualTrackKey: "ext:1",
      loadState: "loading",
    });
  });

  it("selects ass renderer for ASS tracks", () => {
    expect(
      resolveWebSubtitleSelection({
        selectedSubtitleKey: "ext:1",
        subtitleTrackRequests: [
          track({
            assEligible: true,
            assSrc: "/api/subtitles/1/ass",
            deliveryModes: [
              { mode: "direct_vtt", requiresReload: false },
              { mode: "ass", requiresReload: false },
            ],
            preferredWebDeliveryMode: "ass",
          }),
        ],
        subtitleLoadStateByKey: {},
        burnEmbeddedSubtitleStreamIndex: null,
        videoSourceIsHls: false,
        hls: null,
        resolutionVersion: 0,
      }),
    ).toMatchObject({
      renderer: "ass",
      selectedDeliveryMode: "ass",
      activeAssSource: "/api/subtitles/1/ass",
    });
  });

  it("selects burn_in for burn-only embedded tracks", () => {
    expect(
      resolveWebSubtitleSelection({
        selectedSubtitleKey: "emb:7",
        subtitleTrackRequests: [
          track({
            key: "emb:7",
            logicalId: "emb:7",
            origin: "embedded",
            requiresBurn: true,
            deliveryModes: [{ mode: "burn_in", requiresReload: true }],
            preferredWebDeliveryMode: "burn_in",
          }),
        ],
        subtitleLoadStateByKey: {},
        burnEmbeddedSubtitleStreamIndex: 7,
        videoSourceIsHls: true,
        hls: { subtitleTracks: [] } as never,
        resolutionVersion: 1,
      }),
    ).toMatchObject({
      renderer: "burn_in",
      selectedDeliveryMode: "burn_in",
      manualTrackKey: null,
    });
  });
});
