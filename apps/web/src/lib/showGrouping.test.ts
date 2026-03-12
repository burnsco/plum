import { describe, expect, it } from "vitest";
import { groupMediaByShow } from "./showGrouping";

describe("groupMediaByShow", () => {
  it("merges unmatched anime episodes into an identified show when titles normalize to the same key", () => {
    const groups = groupMediaByShow([
      {
        id: 1,
        title: "Dragon Ball - S01E01 - Secret of the Dragon Balls",
        path: "/anime/Dragon Ball/S01E01.mkv",
        duration: 1800,
        type: "anime",
        match_status: "identified",
        tmdb_id: 123,
        season: 1,
        episode: 1,
      },
      {
        id: 2,
        title: "Dragonball - S01E02 - The Emperor's Quest",
        path: "/anime/Dragonball/S01E02.mkv",
        duration: 1800,
        type: "anime",
        match_status: "unmatched",
        season: 1,
        episode: 2,
      },
    ]);

    expect(groups).toHaveLength(1);
    expect(groups[0]?.showKey).toBe("tmdb-123");
    expect(groups[0]?.episodes).toHaveLength(2);
    expect(groups[0]?.unmatchedCount).toBe(1);
  });
});
