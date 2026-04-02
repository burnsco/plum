import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/queries", () => ({
  useAddDiscoverTitle: vi.fn(),
  useDiscoverGenres: vi.fn(),
  useDiscover: vi.fn(),
  useDiscoverSearch: vi.fn(),
}));

vi.mock("@/contexts/AuthContext", () => ({
  useAuthState: vi.fn(),
}));

import { useAddDiscoverTitle, useDiscover, useDiscoverGenres, useDiscoverSearch } from "@/queries";
import { useAuthState } from "@/contexts/AuthContext";
import { Discover } from "./Discover";

describe("Discover", () => {
  beforeEach(() => {
    vi.mocked(useAuthState).mockReturnValue({
      user: { is_admin: true },
    } as never);
    vi.mocked(useDiscoverSearch).mockReturnValue({
      data: undefined,
      error: null,
      isLoading: false,
      refetch: vi.fn(),
    } as never);
    vi.mocked(useDiscoverGenres).mockReturnValue({
      data: {
        movie_genres: [{ id: 28, name: "Action" }],
        tv_genres: [{ id: 18, name: "Drama" }],
      },
      error: null,
      refetch: vi.fn(),
    } as never);
  });

  it("renders an add action and calls the mutation", () => {
    const mutate = vi.fn();
    vi.mocked(useAddDiscoverTitle).mockReturnValue({
      mutate,
      isPending: false,
      variables: undefined,
    } as never);
    vi.mocked(useDiscover).mockReturnValue({
      data: {
        shelves: [
          {
            id: "trending",
            title: "Trending",
            items: [
              {
                media_type: "movie",
                tmdb_id: 101,
                title: "Movie Match",
                vote_average: 8.1,
                acquisition: {
                  state: "not_added",
                  can_add: true,
                  is_configured: true,
                  source: "radarr",
                },
              },
            ],
          },
        ],
      },
      error: null,
      isLoading: false,
      refetch: vi.fn(),
    } as never);

    render(
      <MemoryRouter>
        <Discover />
      </MemoryRouter>,
    );

    expect(screen.getByRole("link", { name: "Movie Match" })).toHaveClass(
      "show-card-hit-area--with-inline-action",
    );
    fireEvent.click(screen.getByRole("button", { name: "Add" }));
    expect(mutate).toHaveBeenCalledWith({ mediaType: "movie", tmdbId: 101 });
  });

  it("shows a pending add state for the matching title", () => {
    vi.mocked(useAddDiscoverTitle).mockReturnValue({
      mutate: vi.fn(),
      isPending: true,
      variables: { mediaType: "movie", tmdbId: 101 },
    } as never);
    vi.mocked(useDiscover).mockReturnValue({
      data: {
        shelves: [
          {
            id: "trending",
            title: "Trending",
            items: [
              {
                media_type: "movie",
                tmdb_id: 101,
                title: "Movie Match",
                acquisition: {
                  state: "not_added",
                  can_add: true,
                  is_configured: true,
                  source: "radarr",
                },
              },
            ],
          },
        ],
      },
      error: null,
      isLoading: false,
      refetch: vi.fn(),
    } as never);

    render(
      <MemoryRouter>
        <Discover />
      </MemoryRouter>,
    );

    expect(screen.getByRole("button", { name: "Adding..." })).toBeDisabled();
  });

  it("filters shelves by media type and renders view-all links", () => {
    vi.mocked(useAddDiscoverTitle).mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
      variables: undefined,
    } as never);
    vi.mocked(useDiscover).mockReturnValue({
      data: {
        shelves: [
          {
            id: "trending",
            title: "Trending",
            items: [
              { media_type: "movie", tmdb_id: 101, title: "Movie Match" },
              { media_type: "tv", tmdb_id: 202, title: "TV Match" },
            ],
          },
        ],
      },
      error: null,
      isLoading: false,
      refetch: vi.fn(),
    } as never);

    render(
      <MemoryRouter>
        <Discover />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Movies" }));
    expect(screen.getByRole("link", { name: "Movie Match" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "TV Match" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "View all" })).toHaveAttribute(
      "href",
      "/discover/browse?category=trending&mediaType=movie",
    );
  });

  it("renders genre links", () => {
    vi.mocked(useAddDiscoverTitle).mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
      variables: undefined,
    } as never);
    vi.mocked(useDiscover).mockReturnValue({
      data: { shelves: [] },
      error: null,
      isLoading: false,
      refetch: vi.fn(),
    } as never);

    render(
      <MemoryRouter>
        <Discover />
      </MemoryRouter>,
    );

    expect(screen.getByRole("link", { name: "Action" })).toHaveAttribute(
      "href",
      "/discover/browse?mediaType=movie&genre=28",
    );
    expect(screen.getByRole("link", { name: "Drama" })).toHaveAttribute(
      "href",
      "/discover/browse?mediaType=tv&genre=18",
    );
  });
});
