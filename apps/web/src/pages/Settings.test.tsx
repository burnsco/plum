import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/queries", () => ({
  useLibraries: vi.fn(),
  useMediaStackSettings: vi.fn(),
  useMetadataArtworkSettings: vi.fn(),
  useTranscodingSettings: vi.fn(),
  useUpdateLibraryPlaybackPreferences: vi.fn(),
  useUpdateMediaStackSettings: vi.fn(),
  useUpdateMetadataArtworkSettings: vi.fn(),
  useUpdateTranscodingSettings: vi.fn(),
  useValidateMediaStackSettings: vi.fn(),
}));

vi.mock("@/contexts/AuthContext", () => ({
  useAuthState: vi.fn(),
}));

import {
  useLibraries,
  useMediaStackSettings,
  useMetadataArtworkSettings,
  useTranscodingSettings,
  useUpdateLibraryPlaybackPreferences,
  useUpdateMediaStackSettings,
  useUpdateMetadataArtworkSettings,
  useUpdateTranscodingSettings,
  useValidateMediaStackSettings,
} from "@/queries";
import { useAuthState } from "@/contexts/AuthContext";
import { Settings } from "./Settings";

describe("Settings media stack", () => {
  const validateMediaStack = vi.fn();
  const saveMediaStack = vi.fn();

  beforeEach(() => {
    validateMediaStack.mockReset();
    saveMediaStack.mockReset();
    vi.mocked(useAuthState).mockReturnValue({
      user: { is_admin: true },
    } as never);
    vi.mocked(useLibraries).mockReturnValue({
      data: [],
      isLoading: false,
      isError: false,
    } as never);
    vi.mocked(useTranscodingSettings).mockReturnValue({
      data: {
        settings: {
          vaapiEnabled: true,
          vaapiDevicePath: "/dev/dri/renderD128",
          decodeCodecs: {
            h264: true,
            hevc: true,
            mpeg2: true,
            vc1: true,
            vp8: true,
            vp9: true,
            av1: true,
            hevc10bit: true,
            vp910bit: true,
          },
          hardwareEncodingEnabled: true,
          encodeFormats: {
            h264: true,
            hevc: false,
            av1: false,
          },
          preferredHardwareEncodeFormat: "h264",
          allowSoftwareFallback: true,
          crf: 22,
          audioBitrate: "192k",
          audioChannels: 2,
          threads: 0,
          keyframeInterval: 48,
          maxBitrate: "",
        },
        warnings: [],
      },
      isLoading: false,
      isError: false,
    } as never);
    vi.mocked(useMetadataArtworkSettings).mockReturnValue({
      data: {
        settings: {
          movies: { fanart: true, tmdb: true, tvdb: true },
          shows: { fanart: true, tmdb: true, tvdb: true },
          seasons: { fanart: true, tmdb: true, tvdb: true },
          episodes: { tmdb: true, tvdb: true, omdb: true },
        },
        provider_availability: [],
      },
      isLoading: false,
      isError: false,
    } as never);
    vi.mocked(useMediaStackSettings).mockReturnValue({
      data: {
        radarr: {
          baseUrl: "http://radarr.test",
          apiKey: "radarr-key",
          qualityProfileId: 8,
          rootFolderPath: "/storage/media/movies",
          searchOnAdd: true,
        },
        sonarrTv: {
          baseUrl: "http://sonarr.test",
          apiKey: "sonarr-key",
          qualityProfileId: 4,
          rootFolderPath: "/storage/media/tv",
          searchOnAdd: true,
        },
      },
      isLoading: false,
      isError: false,
    } as never);
    vi.mocked(useUpdateLibraryPlaybackPreferences).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as never);
    vi.mocked(useUpdateTranscodingSettings).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as never);
    vi.mocked(useUpdateMetadataArtworkSettings).mockReturnValue({
      mutateAsync: vi.fn(),
      isPending: false,
    } as never);
    vi.mocked(useValidateMediaStackSettings).mockReturnValue({
      mutateAsync: validateMediaStack,
      isPending: false,
    } as never);
    vi.mocked(useUpdateMediaStackSettings).mockReturnValue({
      mutateAsync: saveMediaStack,
      isPending: false,
    } as never);
  });

  it("loads, validates, and saves media stack settings", async () => {
    validateMediaStack.mockResolvedValue({
      radarr: {
        configured: true,
        reachable: true,
        rootFolders: [{ path: "/storage/media/movies" }],
        qualityProfiles: [{ id: 8, name: "HD Bluray + WEB" }],
      },
      sonarrTv: {
        configured: true,
        reachable: true,
        rootFolders: [{ path: "/storage/media/tv" }],
        qualityProfiles: [{ id: 4, name: "HD-1080p" }],
      },
    });
    saveMediaStack.mockResolvedValue({
      radarr: {
        baseUrl: "http://radarr.internal",
        apiKey: "radarr-key",
        qualityProfileId: 8,
        rootFolderPath: "/storage/media/movies",
        searchOnAdd: true,
      },
      sonarrTv: {
        baseUrl: "http://sonarr.test",
        apiKey: "sonarr-key",
        qualityProfileId: 4,
        rootFolderPath: "/storage/media/tv",
        searchOnAdd: true,
      },
    });

    render(<Settings />);

    const radarrInput = screen.getAllByLabelText("Base URL", { selector: "input" })[0];
    expect(radarrInput).toHaveValue("http://radarr.test");

    fireEvent.click(screen.getByRole("button", { name: "Validate & load defaults" }));
    await waitFor(() => expect(validateMediaStack).toHaveBeenCalled());

    fireEvent.change(radarrInput, { target: { value: "http://radarr.internal" } });
    fireEvent.click(screen.getAllByRole("button", { name: "Save settings" })[0]!);

    await waitFor(() =>
      expect(saveMediaStack).toHaveBeenCalledWith(
        expect.objectContaining({
          radarr: expect.objectContaining({ baseUrl: "http://radarr.internal" }),
        }),
      ),
    );
  });

  it("marks refreshed validation defaults as unsaved", async () => {
    validateMediaStack.mockResolvedValue({
      radarr: {
        configured: true,
        reachable: true,
        rootFolders: [{ path: "/storage/media/new-movies" }],
        qualityProfiles: [{ id: 18, name: "4K WEB" }],
      },
      sonarrTv: {
        configured: true,
        reachable: true,
        rootFolders: [{ path: "/storage/media/tv" }],
        qualityProfiles: [{ id: 4, name: "HD-1080p" }],
      },
    });
    saveMediaStack.mockResolvedValue({
      radarr: {
        baseUrl: "http://radarr.test",
        apiKey: "radarr-key",
        qualityProfileId: 18,
        rootFolderPath: "/storage/media/new-movies",
        searchOnAdd: true,
      },
      sonarrTv: {
        baseUrl: "http://sonarr.test",
        apiKey: "sonarr-key",
        qualityProfileId: 4,
        rootFolderPath: "/storage/media/tv",
        searchOnAdd: true,
      },
    });

    render(<Settings />);

    fireEvent.click(screen.getByRole("button", { name: "Validate & load defaults" }));

    await waitFor(() => expect(validateMediaStack).toHaveBeenCalled());
    const mediaStackSaveButton = screen.getAllByRole("button", { name: "Save settings" })[0]!;
    expect(mediaStackSaveButton).toBeEnabled();

    fireEvent.click(mediaStackSaveButton);

    await waitFor(() =>
      expect(saveMediaStack).toHaveBeenCalledWith(
        expect.objectContaining({
          radarr: expect.objectContaining({
            rootFolderPath: "/storage/media/new-movies",
            qualityProfileId: 18,
          }),
        }),
      ),
    );
  });

  it("prefers the named default quality profiles when the saved ids are invalid", async () => {
    validateMediaStack.mockResolvedValue({
      radarr: {
        configured: true,
        reachable: true,
        rootFolders: [{ path: "/storage/media/movies" }],
        qualityProfiles: [
          { id: 1, name: "HD Bluray + WEB" },
          { id: 19, name: "  uhd bluray + web  " },
          { id: 44, name: "Remux-2160p" },
        ],
      },
      sonarrTv: {
        configured: true,
        reachable: true,
        rootFolders: [{ path: "/storage/media/tv" }],
        qualityProfiles: [
          { id: 2, name: "HD-1080p" },
          { id: 28, name: "web-2160p" },
          { id: 35, name: "Anime-2160p" },
        ],
      },
    });
    saveMediaStack.mockResolvedValue({
      radarr: {
        baseUrl: "http://radarr.test",
        apiKey: "radarr-key",
        qualityProfileId: 19,
        rootFolderPath: "/storage/media/movies",
        searchOnAdd: true,
      },
      sonarrTv: {
        baseUrl: "http://sonarr.test",
        apiKey: "sonarr-key",
        qualityProfileId: 28,
        rootFolderPath: "/storage/media/tv",
        searchOnAdd: true,
      },
    });
    vi.mocked(useMediaStackSettings).mockReturnValue({
      data: {
        radarr: {
          baseUrl: "http://radarr.test",
          apiKey: "radarr-key",
          qualityProfileId: 999,
          rootFolderPath: "/storage/media/movies",
          searchOnAdd: true,
        },
        sonarrTv: {
          baseUrl: "http://sonarr.test",
          apiKey: "sonarr-key",
          qualityProfileId: 777,
          rootFolderPath: "/storage/media/tv",
          searchOnAdd: true,
        },
      },
      isLoading: false,
      isError: false,
    } as never);

    render(<Settings />);

    fireEvent.click(screen.getByRole("button", { name: "Validate & load defaults" }));
    await waitFor(() => expect(validateMediaStack).toHaveBeenCalled());

    fireEvent.click(screen.getAllByRole("button", { name: "Save settings" })[0]!);

    await waitFor(() =>
      expect(saveMediaStack).toHaveBeenCalledWith(
        expect.objectContaining({
          radarr: expect.objectContaining({ qualityProfileId: 19 }),
          sonarrTv: expect.objectContaining({ qualityProfileId: 28 }),
        }),
      ),
    );
  });

  it("keeps an already-valid saved quality profile even when it is not the preferred name", async () => {
    validateMediaStack.mockResolvedValue({
      radarr: {
        configured: true,
        reachable: true,
        rootFolders: [{ path: "/storage/media/movies" }],
        qualityProfiles: [
          { id: 8, name: "HD Bluray + WEB" },
          { id: 12, name: "UHD Bluray + Web" },
        ],
      },
      sonarrTv: {
        configured: true,
        reachable: true,
        rootFolders: [{ path: "/storage/media/tv" }],
        qualityProfiles: [
          { id: 4, name: "HD-1080p" },
          { id: 6, name: "WEB-2160p" },
        ],
      },
    });

    render(<Settings />);

    fireEvent.click(screen.getByRole("button", { name: "Validate & load defaults" }));
    await waitFor(() => expect(validateMediaStack).toHaveBeenCalled());

    const qualityProfileSelects = screen.getAllByLabelText("Quality profile", {
      selector: "select",
    });
    expect(qualityProfileSelects[0]).toHaveValue("8");
    expect(qualityProfileSelects[1]).toHaveValue("4");
    expect(screen.getAllByRole("button", { name: "Save settings" })[0]).toBeDisabled();
    expect(saveMediaStack).not.toHaveBeenCalled();
  });

  it("shows a warning when validation cannot reach the configured services", async () => {
    validateMediaStack.mockResolvedValue({
      radarr: {
        configured: true,
        reachable: false,
        errorMessage: "401 Unauthorized",
        rootFolders: [],
        qualityProfiles: [],
      },
      sonarrTv: {
        configured: true,
        reachable: false,
        errorMessage: "connection refused",
        rootFolders: [],
        qualityProfiles: [],
      },
    });

    render(<Settings />);

    fireEvent.click(screen.getByRole("button", { name: "Validate & load defaults" }));

    expect(
      await screen.findByText(
        "Unable to reach the configured media stack services. Check the connection details below.",
      ),
    ).toBeInTheDocument();
  });
});
