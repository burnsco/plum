import { describe, expect, it } from "vitest";
import {
  buildRecentlyAddedToastLabel,
  buildRecentlyAddedToastMessage,
  recentlyAddedEntryKey,
} from "./libraryReadyNotifications";

describe("libraryReadyNotifications", () => {
  it("uses show keys for grouped show entries", () => {
    expect(
      recentlyAddedEntryKey({
        kind: "show",
        show_key: "tmdb-42",
        show_title: "Example Show",
        episode_label: "S1 E2",
        media: {
          id: 7,
          library_id: 3,
          title: "Example Show S1 E2",
          path: "/tv/example-show/episode-2.mkv",
          duration: 1200,
          type: "tv",
        },
      }),
    ).toBe("show:tmdb-42");
  });

  it("builds a friendly show label with the newest episode", () => {
    expect(
      buildRecentlyAddedToastLabel({
        kind: "show",
        show_key: "tmdb-42",
        show_title: "Example Show",
        episode_label: "S1 E2",
        media: {
          id: 7,
          library_id: 3,
          title: "Example Show S1 E2",
          path: "/tv/example-show/episode-2.mkv",
          duration: 1200,
          type: "tv",
        },
      }),
    ).toBe("Example Show (S1 E2)");
  });

  it("includes the library name when available", () => {
    expect(
      buildRecentlyAddedToastMessage(
        {
          kind: "movie",
          show_key: "",
          show_title: "",
          episode_label: "",
          media: {
            id: 9,
            library_id: 4,
            title: "Dune",
            path: "/movies/dune.mkv",
            duration: 7200,
            type: "movie",
          },
        },
        [
          {
            id: 4,
            name: "Movies",
            path: "/movies",
            type: "movie",
            user_id: 1,
            subtitles_enabled_by_default: true,
            watcher_enabled: true,
            watcher_mode: "auto",
            scan_interval_minutes: 0,
          },
        ],
      ),
    ).toBe("Ready to play in Movies: Dune");
  });
});

