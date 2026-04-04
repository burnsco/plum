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

  it("uses the real season token so junk like - Sno does not shorten the show key (server ListShowEpisodeRefs must match)", () => {
    expect(
      getShowKey({
        id: 1,
        title: "Black Spot (Zone Blanche) S01 - Hardcoded Eng Subs - Sno - S01E01 - Pilot",
        path: "/tv/Black Spot/S01E01.mkv",
        duration: 1800,
        type: "tv",
        match_status: "unmatched",
        season: 1,
        episode: 1,
      }),
    ).toBe("title-blackspotzoneblanches01hardcodedengsubssno");
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

  it("does not count episodes as local when TMDb id is present (match_status may lag)", () => {
    const groups = groupMediaByShow([
      {
        id: 1,
        title: "Show - S01E01 - Pilot",
        path: "/anime/Show/S01E01.mkv",
        duration: 1800,
        type: "anime",
        match_status: "local",
        tmdb_id: 999,
        season: 1,
        episode: 1,
      },
    ]);

    expect(groups[0]?.localCount).toBe(0);
    expect(groups[0]?.identifiedCount).toBe(1);
  });

  it("prefers canonical show TMDb scores when episode scores are missing", () => {
    const groups = groupMediaByShow([
      {
        id: 1,
        title: "Slow Horses - S01E01 - Failure's Contagious",
        path: "/tv/Slow Horses/S01E01.mkv",
        duration: 1800,
        type: "tv",
        match_status: "identified",
        tmdb_id: 321,
        show_vote_average: 8.2,
        season: 1,
        episode: 1,
      },
    ]);

    expect(groups[0]?.showVoteAverage).toBe(8.2);
    expect(groups[0]?.showImdbRating).toBeUndefined();
  });
});
