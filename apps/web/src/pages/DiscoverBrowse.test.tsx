import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/queries", () => ({
  useAddDiscoverTitle: vi.fn(),
  useDiscoverBrowse: vi.fn(),
  useDiscoverGenres: vi.fn(),
}));

vi.mock("@/contexts/AuthContext", () => ({
  useAuthState: vi.fn(),
}));

import { useAuthState } from "@/contexts/AuthContext";
import { useAddDiscoverTitle, useDiscoverBrowse, useDiscoverGenres } from "@/queries";
import { DiscoverBrowse } from "./DiscoverBrowse";

describe("DiscoverBrowse", () => {
  beforeEach(() => {
    vi.mocked(useAuthState).mockReturnValue({
      user: { is_admin: true },
    } as never);
    vi.mocked(useAddDiscoverTitle).mockReturnValue({
      mutate: vi.fn(),
      isPending: false,
      variables: undefined,
    } as never);
    vi.mocked(useDiscoverGenres).mockReturnValue({
      data: {
        movie_genres: [{ id: 28, name: "Action" }],
        tv_genres: [{ id: 18, name: "Drama" }],
      },
    } as never);
  });

  it("renders browse results without a manual load more button", () => {
    vi.mocked(useDiscoverBrowse).mockReturnValue({
      data: {
        pages: [
          {
            items: [{ media_type: "movie", tmdb_id: 101, title: "Browse Movie" }],
            page: 1,
            total_pages: 3,
            total_results: 60,
            media_type: "movie",
            category: "popular-movies",
          },
        ],
      },
      error: null,
      isLoading: false,
      refetch: vi.fn(),
      hasNextPage: true,
      fetchNextPage: vi.fn(),
      isFetchingNextPage: false,
    } as never);

    render(
      <MemoryRouter initialEntries={["/discover/browse?category=popular-movies&mediaType=movie"]}>
        <DiscoverBrowse />
      </MemoryRouter>,
    );

    expect(screen.getByText("60 titles available")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Browse Movie" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Load more" })).not.toBeInTheDocument();
  });

  it("renders clear filters and genre links", () => {
    vi.mocked(useDiscoverBrowse).mockReturnValue({
      data: {
        pages: [
          {
            items: [],
            page: 1,
            total_pages: 1,
            total_results: 0,
            media_type: "movie",
            genre: { id: 28, name: "Action" },
          },
        ],
      },
      error: null,
      isLoading: false,
      refetch: vi.fn(),
      hasNextPage: false,
      fetchNextPage: vi.fn(),
      isFetchingNextPage: false,
    } as never);

    render(
      <MemoryRouter initialEntries={["/discover/browse?mediaType=movie&genre=28"]}>
        <DiscoverBrowse />
      </MemoryRouter>,
    );

    expect(screen.getByRole("link", { name: "Clear filters" })).toHaveAttribute(
      "href",
      "/discover/browse",
    );
    expect(screen.getByRole("link", { name: "Action" })).toHaveAttribute(
      "href",
      "/discover/browse?mediaType=movie&genre=28",
    );
  });
});
