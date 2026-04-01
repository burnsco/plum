import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/queries", () => ({
  useDownloads: vi.fn(),
}));

vi.mock("@/contexts/AuthContext", () => ({
  useAuthState: vi.fn(),
}));

import { useDownloads } from "@/queries";
import { useAuthState } from "@/contexts/AuthContext";
import { Downloads } from "./Downloads";

describe("Downloads", () => {
  beforeEach(() => {
    vi.mocked(useAuthState).mockReturnValue({
      user: { is_admin: true },
    } as never);
  });

  it("shows a setup prompt when the media stack is not configured", () => {
    vi.mocked(useDownloads).mockReturnValue({
      data: {
        configured: false,
        items: [],
      },
      error: null,
      isLoading: false,
      refetch: vi.fn(),
    } as never);

    render(
      <MemoryRouter>
        <Downloads />
      </MemoryRouter>,
    );

    expect(screen.getByText("Media stack not configured")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Open Settings" })).toBeInTheDocument();
  });

  it("renders active downloads", () => {
    vi.mocked(useDownloads).mockReturnValue({
      data: {
        configured: true,
        items: [
          {
            id: "radarr:71",
            title: "Movie Match",
            media_type: "movie",
            source: "radarr",
            status_text: "Downloading",
            progress: 75,
            size_left_bytes: 1024,
            eta_seconds: 300,
          },
        ],
      },
      error: null,
      isLoading: false,
      refetch: vi.fn(),
    } as never);

    render(
      <MemoryRouter>
        <Downloads />
      </MemoryRouter>,
    );

    expect(screen.getByText("Movie Match")).toBeInTheDocument();
    expect(screen.getByText("Downloading")).toBeInTheDocument();
    expect(screen.getByText("75%")).toBeInTheDocument();
  });
});
