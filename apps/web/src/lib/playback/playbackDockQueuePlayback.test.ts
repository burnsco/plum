import { describe, expect, it } from "vitest";
import type { MediaItem } from "../../api";
import {
  hasNextQueueItem,
  musicPlaybackShouldLoopSameTrack,
  nextQueueItemAfterIndex,
  resolveUpNextItemOnVideoEnd,
  shouldRestartCurrentVideoOnPrevious,
} from "./playbackDockQueuePlayback";

function item(id: number): MediaItem {
  return {
    id,
    library_id: 1,
    title: `Item ${id}`,
    path: `/x/${id}`,
    duration: 60,
    type: "movie",
  } as MediaItem;
}

describe("playbackDockQueuePlayback", () => {
  describe("hasNextQueueItem", () => {
    it("is false at end of queue", () => {
      expect(hasNextQueueItem(3, 2)).toBe(false);
    });

    it("is true when another item follows", () => {
      expect(hasNextQueueItem(3, 0)).toBe(true);
    });
  });

  describe("nextQueueItemAfterIndex", () => {
    it("returns null for last index", () => {
      expect(nextQueueItemAfterIndex([item(1), item(2)], 1)).toBeNull();
    });

    it("returns the following item", () => {
      const q = [item(1), item(2), item(3)];
      expect(nextQueueItemAfterIndex(q, 0)?.id).toBe(2);
    });
  });

  describe("resolveUpNextItemOnVideoEnd", () => {
    it("returns null when autoplay is disabled", () => {
      expect(
        resolveUpNextItemOnVideoEnd({
          videoAutoplayEnabled: false,
          queue: [item(1), item(2)],
          queueIndex: 0,
        }),
      ).toBeNull();
    });

    it("returns the next item when autoplay is on", () => {
      const q = [item(1), item(2)];
      expect(
        resolveUpNextItemOnVideoEnd({
          videoAutoplayEnabled: true,
          queue: q,
          queueIndex: 0,
        })?.id,
      ).toBe(2);
    });
  });

  describe("shouldRestartCurrentVideoOnPrevious", () => {
    it("seeks to start when past threshold", () => {
      expect(shouldRestartCurrentVideoOnPrevious(6, 5)).toBe(true);
    });

    it("goes to previous queue item at or before threshold", () => {
      expect(shouldRestartCurrentVideoOnPrevious(5, 5)).toBe(false);
    });
  });

  describe("musicPlaybackShouldLoopSameTrack", () => {
    it("loops only for repeat one", () => {
      expect(musicPlaybackShouldLoopSameTrack("one")).toBe(true);
      expect(musicPlaybackShouldLoopSameTrack("off")).toBe(false);
      expect(musicPlaybackShouldLoopSameTrack("all")).toBe(false);
    });
  });
});
