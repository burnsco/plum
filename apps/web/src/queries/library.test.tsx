import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { HomeDashboard } from "@/api";
import * as api from "@/api";
import { queryKeys } from "./shared";
import { useClearShowProgress } from "./library";

function buildHomeDashboardFixture(): HomeDashboard {
  return {
    continueWatching: [
      {
        kind: "show",
        show_key: "tmdb-42",
        show_title: "Target Show",
        episode_label: "S01E01",
        remaining_seconds: 1200,
        media: {
          id: 1001,
          library_id: 1,
          title: "Target Show - S01E01",
          path: "/tv/target-s01e01.mkv",
          duration: 1800,
          type: "tv",
        },
      },
      {
        kind: "show",
        show_key: "tmdb-42",
        show_title: "Target Show Other Library",
        episode_label: "S01E02",
        remaining_seconds: 900,
        media: {
          id: 2001,
          library_id: 2,
          title: "Target Show - S01E02",
          path: "/anime/target-s01e02.mkv",
          duration: 1800,
          type: "anime",
        },
      },
      {
        kind: "show",
        show_key: "tmdb-99",
        show_title: "Other Show",
        episode_label: "S01E03",
        remaining_seconds: 600,
        media: {
          id: 3001,
          library_id: 1,
          title: "Other Show - S01E03",
          path: "/tv/other-s01e03.mkv",
          duration: 1800,
          type: "tv",
        },
      },
    ],
    recentlyAdded: [],
    recentlyAddedTvEpisodes: [],
    recentlyAddedTvShows: [],
    recentlyAddedMovies: [],
    recentlyAddedAnimeEpisodes: [],
    recentlyAddedAnimeShows: [],
  };
}

function ClearShowProgressHarness() {
  const mutation = useClearShowProgress();
  return (
    <button
      type="button"
      onClick={() =>
        void mutation.mutateAsync({
          libraryId: 1,
          showKey: "tmdb-42",
        })
      }
    >
      Clear show progress
    </button>
  );
}

describe("useClearShowProgress", () => {
  it("optimistically removes only entries from the targeted library", async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    queryClient.setQueryData(queryKeys.home, buildHomeDashboardFixture());

    let resolveRequest!: () => void;
    const request = new Promise<void>((resolve) => {
      resolveRequest = resolve;
    });
    const clearSpy = vi.spyOn(api, "clearShowProgress").mockReturnValue(request);

    render(
      <QueryClientProvider client={queryClient}>
        <ClearShowProgressHarness />
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Clear show progress" }));

    await waitFor(() => {
      const home = queryClient.getQueryData<HomeDashboard>(queryKeys.home);
      expect(home?.continueWatching).toHaveLength(2);
      expect(home?.continueWatching.some((entry) => entry.media.library_id === 1 && entry.show_key === "tmdb-42")).toBe(false);
      expect(home?.continueWatching.some((entry) => entry.media.library_id === 2 && entry.show_key === "tmdb-42")).toBe(true);
      expect(home?.continueWatching.some((entry) => entry.show_key === "tmdb-99")).toBe(true);
    });

    resolveRequest();

    await waitFor(() => {
      expect(clearSpy).toHaveBeenCalledWith(1, "tmdb-42");
    });
  });
});
