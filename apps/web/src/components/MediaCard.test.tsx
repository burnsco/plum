import { render } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../api", async () => {
  return {
    BASE_URL: "http://backend.test",
  };
});

import MediaCard from "./MediaCard";

describe("MediaCard artwork URLs", () => {
  it("resolves backend-relative movie poster URLs against the backend base", () => {
    const { container } = render(
      <MediaCard
        item={{
          key: "movie-99",
          title: "Movie Poster",
          subtitle: "2025",
          posterUrl: "/api/media/99/artwork/poster",
          onClick: () => {},
        }}
      />,
    );

    const poster = container.querySelector("img");
    expect(poster).toHaveAttribute("src", "http://backend.test/api/media/99/artwork/poster");
  });

  it("resolves backend-relative grouped show poster URLs against the backend base", () => {
    const { container } = render(
      <MediaCard
        item={{
          key: "show-tmdb-123",
          title: "Slow Horses",
          subtitle: "2 episodes",
          posterUrl: "/api/libraries/1/shows/tmdb-123/artwork/poster",
          ratingLabel: "TMDB",
          ratingValue: 8.7,
          onClick: () => {},
        }}
      />,
    );

    const poster = container.querySelector("img");
    expect(poster).toHaveAttribute(
      "src",
      "http://backend.test/api/libraries/1/shows/tmdb-123/artwork/poster",
    );
  });
});
