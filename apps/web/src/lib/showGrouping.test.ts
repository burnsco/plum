import { describe, expect, it } from "vitest";
import { getShowKey, groupMediaByShow } from "./showGrouping";

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

  it("keeps year-qualified unmatched shows on separate fallback keys", () => {
    expect(
      getShowKey({
        id: 1,
        title: "Battlestar Galactica (1978) - S01E01 - Saga of a Star World",
        path: "/tv/Battlestar Galactica (1978)/S01E01.mkv",
        duration: 1800,
        type: "tv",
        match_status: "unmatched",
        season: 1,
        episode: 1,
      }),
    ).toBe("title-battlestargalactica1978");

    expect(
      getShowKey({
        id: 2,
        title: "Battlestar Galactica (2004) - S01E01 - 33",
        path: "/tv/Battlestar Galactica (2004)/S01E01.mkv",
        duration: 1800,
        type: "tv",
        match_status: "unmatched",
        season: 1,
        episode: 1,
      }),
    ).toBe("title-battlestargalactica2004");
  });
});
