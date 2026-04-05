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

const mockUseLibraries = useLibraries as unknown as ReturnType<typeof vi.fn>;
const mockUseScanQueue = useScanQueue as unknown as ReturnType<typeof vi.fn>;

describe("LibraryActivityCenter", () => {
  it("shows identifying progress instead of stale file details", async () => {
    const staleAt = new Date(Date.now() - 20_000).toISOString();
    mockUseLibraries.mockReturnValue({
      data: [{ id: 3, name: "TV", type: "tv", path: "/tv", user_id: 1 }],
    } as unknown as ReturnType<typeof useLibraries>);
    mockUseScanQueue.mockReturnValue({
      activeLibraryIds: [3],
      activityScanStatuses: [
        {
          libraryId: 3,
          phase: "completed",
          enrichmentPhase: "idle",
          enriching: false,
          identifyPhase: "identifying",
          identified: 8,
          identifyFailed: 0,
          processed: 42,
          added: 42,
          updated: 0,
          removed: 0,
          unmatched: 0,
          skipped: 0,
          identifyRequested: true,
          estimatedItems: 42,
          queuePosition: 0,
          activity: {
            stage: "identify",
            current: {
              phase: "identify",
              target: "file",
              relativePath: "Shows/Example/episode01.mkv",
              at: staleAt,
            },
            recent: [],
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

    fireEvent.pointerDown(screen.getByRole("button", { name: /Server activity/i }));

    const statusCard = await screen.findByTestId("library-activity-status-3");
    expect(within(statusCard).getByText("Identifying")).toBeVisible();
    expect(within(statusCard).getByText("Identified 8 items so far")).toBeVisible();
    expect(within(statusCard).queryByText(/Shows\/Example\/episode01/)).not.toBeInTheDocument();
  });

  it("shows the active badge and popup details", async () => {
    const freshAt = new Date().toISOString();
    mockUseLibraries.mockReturnValue({
      data: [{ id: 1, name: "TV", type: "tv", path: "/tv", user_id: 1 }],
    } as unknown as ReturnType<typeof useLibraries>);
    mockUseScanQueue.mockReturnValue({
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
              at: freshAt,
            },
            recent: [
              {
                phase: "discovery",
                target: "file",
                relativePath: "Shows/Example/episode01.mkv",
                at: freshAt,
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
    expect(screen.queryByText("What Plum is doing now, and what just finished.")).not.toBeInTheDocument();
    expect(screen.queryByText("Now")).not.toBeInTheDocument();
    expect(screen.getByTestId("library-activity-status-1")).toHaveTextContent("TV");
    expect(screen.getByTestId("library-activity-status-1")).toHaveTextContent("Importing");
    expect(screen.getByText(/Importing: Shows\/Example\/episode01\.mkv/)).toBeVisible();
  });

  it("shows the empty state when nothing is active", async () => {
    mockUseLibraries.mockReturnValue({
      data: [],
    } as unknown as ReturnType<typeof useLibraries>);
    mockUseScanQueue.mockReturnValue({
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

  it("shows queued identify in the activity list", async () => {
    mockUseLibraries.mockReturnValue({
      data: [
        { id: 2, name: "Movies", type: "movie", path: "/movies", user_id: 1 },
      ],
    } as unknown as ReturnType<typeof useLibraries>);
    mockUseScanQueue.mockReturnValue({
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
    expect(within(statusCard).getByText("Identify queued")).toBeVisible();
    expect(within(statusCard).getByText(/Metadata matching will start/i)).toBeVisible();
  });

  it("updates from identifying to failed without stale activity labels", async () => {
    mockUseLibraries.mockReturnValue({
      data: [
        { id: 4, name: "Movies", type: "movie", path: "/movies", user_id: 1 },
      ],
    } as unknown as ReturnType<typeof useLibraries>);

    mockUseScanQueue.mockReturnValue({
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

    mockUseScanQueue.mockReturnValue({
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

    expect(await screen.findByText("Failed")).toBeVisible();
    expect(screen.queryByText("Just finished")).not.toBeInTheDocument();
    expect(screen.getByText("Failed")).toBeVisible();
    expect(screen.getByText("18 item(s) could not be identified automatically")).toBeVisible();
    expect(screen.queryByText("Identifying")).not.toBeInTheDocument();
  });

  it("shows queued enrichment in the activity list", async () => {
    mockUseLibraries.mockReturnValue({
      data: [{ id: 5, name: "Anime", type: "anime", path: "/anime", user_id: 1 }],
    } as unknown as ReturnType<typeof useLibraries>);
    mockUseScanQueue.mockReturnValue({
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
    expect(within(statusCard).getByText("Analyze queued")).toBeVisible();
    expect(within(statusCard).getByText(/Analysis will run after import/i)).toBeVisible();
  });
});
