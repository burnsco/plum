import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { LibraryActivityCenter } from "./LibraryActivityCenter";

vi.mock("@/queries", () => ({
  useLibraries: vi.fn(),
}));

vi.mock("@/contexts/ScanQueueContext", () => ({
  useScanQueue: vi.fn(),
}));

import { useLibraries } from "@/queries";
import { useScanQueue } from "@/contexts/ScanQueueContext";

describe("LibraryActivityCenter", () => {
  it("shows the active badge and popup details", async () => {
    vi.mocked(useLibraries).mockReturnValue({
      data: [{ id: 1, name: "TV", type: "tv", path: "/tv", user_id: 1 }],
    } as unknown as ReturnType<typeof useLibraries>);
    vi.mocked(useScanQueue).mockReturnValue({
      activeLibraryIds: [1],
      activityScanStatuses: [
        {
          libraryId: 1,
          phase: "scanning",
          enrichmentPhase: "idle",
          enriching: false,
          identifyPhase: "idle",
          identified: 0,
          identifyFailed: 0,
          processed: 4,
          added: 2,
          updated: 1,
          removed: 0,
          unmatched: 1,
          skipped: 1,
          identifyRequested: false,
          estimatedItems: 12,
          queuePosition: 0,
          activity: {
            stage: "discovery",
            current: {
              phase: "discovery",
              target: "file",
              relativePath: "Shows/Example/episode01.mkv",
              at: "2026-03-27T13:00:00Z",
            },
            recent: [
              {
                phase: "discovery",
                target: "file",
                relativePath: "Shows/Example/episode01.mkv",
                at: "2026-03-27T13:00:00Z",
              },
              {
                phase: "discovery",
                target: "directory",
                relativePath: "Shows/Example",
                at: "2026-03-27T12:59:59Z",
              },
            ],
          },
        },
      ],
      recentLibraryActivities: [],
      scanStatuses: {},
      getLibraryScanStatus: vi.fn(),
      hasLibraryScanStatus: vi.fn(),
      queueLibraryScan: vi.fn(),
    });

    render(<LibraryActivityCenter />);

    expect(screen.getByTestId("library-activity-badge")).toHaveTextContent("1");

    fireEvent.pointerDown(
      screen.getByRole("button", { name: /Server activity/i }),
    );

    expect(await screen.findByText("Server activity")).toBeVisible();
    expect(screen.getByText("Now")).toBeVisible();
    expect(screen.getByTestId("library-activity-status-1")).toHaveTextContent("TV");
    expect(screen.getByTestId("library-activity-status-1")).toHaveTextContent("Importing");
    expect(screen.getByText("Shows/Example/episode01.mkv")).toBeVisible();
  });

  it("shows the empty state when nothing is active", async () => {
    vi.mocked(useLibraries).mockReturnValue({
      data: [],
    } as unknown as ReturnType<typeof useLibraries>);
    vi.mocked(useScanQueue).mockReturnValue({
      activeLibraryIds: [],
      activityScanStatuses: [],
      recentLibraryActivities: [],
      scanStatuses: {},
      getLibraryScanStatus: vi.fn(),
      hasLibraryScanStatus: vi.fn(),
      queueLibraryScan: vi.fn(),
    });

    render(<LibraryActivityCenter />);

    fireEvent.pointerDown(
      screen.getByRole("button", { name: /Server activity/i }),
    );

    expect(
      await screen.findByText("Nothing is happening right now."),
    ).toBeVisible();
  });

  it("shows queued identify as waiting work", async () => {
    vi.mocked(useLibraries).mockReturnValue({
      data: [
        { id: 2, name: "Movies", type: "movie", path: "/movies", user_id: 1 },
      ],
    } as unknown as ReturnType<typeof useLibraries>);
    vi.mocked(useScanQueue).mockReturnValue({
      activeLibraryIds: [2],
      activityScanStatuses: [
        {
          libraryId: 2,
          phase: "completed",
          enrichmentPhase: "idle",
          enriching: false,
          identifyPhase: "queued",
          identified: 0,
          identifyFailed: 0,
          processed: 95,
          added: 95,
          updated: 0,
          removed: 0,
          unmatched: 0,
          skipped: 0,
          identifyRequested: true,
          estimatedItems: 95,
          queuePosition: 0,
        },
      ],
      recentLibraryActivities: [],
      scanStatuses: {},
      getLibraryScanStatus: vi.fn(),
      hasLibraryScanStatus: vi.fn(),
      queueLibraryScan: vi.fn(),
    });

    render(<LibraryActivityCenter />);

    fireEvent.pointerDown(
      screen.getByRole("button", { name: /Server activity/i }),
    );

    const statusCard = await screen.findByTestId("library-activity-status-2");
    expect(within(statusCard).getByText("Waiting for identify worker")).toBeVisible();
    expect(
      within(statusCard).queryByText("Identifying"),
    ).not.toBeInTheDocument();
  });

  it("updates from identifying to failed without stale activity labels", async () => {
    vi.mocked(useLibraries).mockReturnValue({
      data: [
        { id: 4, name: "Movies", type: "movie", path: "/movies", user_id: 1 },
      ],
    } as unknown as ReturnType<typeof useLibraries>);

    vi.mocked(useScanQueue).mockReturnValue({
      activeLibraryIds: [4],
      activityScanStatuses: [
        {
          libraryId: 4,
          phase: "completed",
          enrichmentPhase: "idle",
          enriching: false,
          identifyPhase: "identifying",
          identified: 12,
          identifyFailed: 0,
          processed: 95,
          added: 95,
          updated: 0,
          removed: 0,
          unmatched: 0,
          skipped: 0,
          identifyRequested: true,
          estimatedItems: 95,
          queuePosition: 0,
        },
      ],
      recentLibraryActivities: [],
      scanStatuses: {},
      getLibraryScanStatus: vi.fn(),
      hasLibraryScanStatus: vi.fn(),
      queueLibraryScan: vi.fn(),
    });

    const view = render(<LibraryActivityCenter />);

    fireEvent.pointerDown(
      screen.getByRole("button", { name: /Server activity/i }),
    );

    let statusCard = await screen.findByTestId("library-activity-status-4");
    expect(within(statusCard).getByText("Identifying")).toBeVisible();

    vi.mocked(useScanQueue).mockReturnValue({
      activeLibraryIds: [],
      activityScanStatuses: [],
      recentLibraryActivities: [
        {
          libraryId: 4,
          status: "failed",
          summary: "Failed",
          detail: "18 item(s) could not be identified automatically",
          finishedAt: "2026-03-27T13:05:00Z",
        },
      ],
      scanStatuses: {},
      getLibraryScanStatus: vi.fn(),
      hasLibraryScanStatus: vi.fn(),
      queueLibraryScan: vi.fn(),
    });

    view.rerender(<LibraryActivityCenter />);

    expect(await screen.findByText("Just finished")).toBeVisible();
    expect(screen.getByText("Failed")).toBeVisible();
    expect(screen.getByText("18 item(s) could not be identified automatically")).toBeVisible();
    expect(screen.queryByText("Identifying")).not.toBeInTheDocument();
  });

  it("shows queued enrichment as waiting work", async () => {
    vi.mocked(useLibraries).mockReturnValue({
      data: [{ id: 5, name: "Anime", type: "anime", path: "/anime", user_id: 1 }],
    } as unknown as ReturnType<typeof useLibraries>);
    vi.mocked(useScanQueue).mockReturnValue({
      activeLibraryIds: [5],
      activityScanStatuses: [
        {
          libraryId: 5,
          phase: "completed",
          enrichmentPhase: "queued",
          enriching: false,
          identifyPhase: "idle",
          identified: 0,
          identifyFailed: 0,
          processed: 12,
          added: 12,
          updated: 0,
          removed: 0,
          unmatched: 0,
          skipped: 0,
          identifyRequested: false,
          estimatedItems: 12,
          queuePosition: 0,
        },
      ],
      recentLibraryActivities: [],
      scanStatuses: {},
      getLibraryScanStatus: vi.fn(),
      hasLibraryScanStatus: vi.fn(),
      queueLibraryScan: vi.fn(),
    });

    render(<LibraryActivityCenter />);

    fireEvent.pointerDown(screen.getByRole("button", { name: /Server activity/i }));

    const statusCard = await screen.findByTestId("library-activity-status-5");
    expect(within(statusCard).getByText("Waiting for analyzer")).toBeVisible();
    expect(within(statusCard).queryByText("Analyzing media")).not.toBeInTheDocument();
  });
});
