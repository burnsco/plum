import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { Sidebar } from "./Sidebar";

vi.mock("@/queries", () => ({
  useLibraries: vi.fn(),
}));

vi.mock("@/contexts/IdentifyQueueContext", () => ({
  useIdentifyQueue: vi.fn(),
}));

vi.mock("@/contexts/ScanQueueContext", () => ({
  useScanQueue: vi.fn(),
}));

import { useLibraries } from "@/queries";
import { useIdentifyQueue } from "@/contexts/IdentifyQueueContext";
import { useScanQueue } from "@/contexts/ScanQueueContext";
import type { LibraryScanStatus } from "@/api";

function renderSidebar() {
  return render(
    <MemoryRouter initialEntries={["/library/2"]}>
      <Routes>
        <Route path="/library/:libraryId" element={<Sidebar />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe("Sidebar", () => {
  it("shows queued identify separately from active identify", () => {
    const queuedStatus: LibraryScanStatus = {
      libraryId: 2,
      phase: "completed",
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
    };
    const activeStatus: LibraryScanStatus = {
      libraryId: 3,
      phase: "completed",
      enriching: false,
      identifyPhase: "identifying",
      identified: 0,
      identifyFailed: 0,
      processed: 227,
      added: 227,
      updated: 0,
      removed: 0,
      unmatched: 0,
      skipped: 0,
      identifyRequested: true,
      estimatedItems: 227,
      queuePosition: 0,
    };

    vi.mocked(useLibraries).mockReturnValue({
      data: [
        { id: 2, name: "Movies", type: "movie", path: "/movies", user_id: 1 },
        { id: 3, name: "Anime", type: "anime", path: "/anime", user_id: 1 },
      ],
      isLoading: false,
    } as unknown as ReturnType<typeof useLibraries>);
    vi.mocked(useIdentifyQueue).mockReturnValue({
      getLibraryPhase: vi.fn((libraryId: number | null) =>
        libraryId === 3 ? "identifying" : undefined,
      ),
      identifyPhases: {},
      queueLibraryIdentify: vi.fn(),
    });
    vi.mocked(useScanQueue).mockReturnValue({
      getLibraryScanStatus: vi.fn((libraryId: number | null) => {
        if (libraryId === 2) return queuedStatus;
        if (libraryId === 3) return activeStatus;
        return undefined;
      }),
      activeLibraryIds: [2, 3],
      activityScanStatuses: [],
      recentLibraryActivities: [],
      scanStatuses: {},
      hasLibraryScanStatus: vi.fn(),
      queueLibraryScan: vi.fn(),
    });

    renderSidebar();

    expect(
      within(screen.getByRole("link", { name: /Movies/i })).getByTestId("library-identifying-2"),
    ).toBeVisible();
    expect(
      within(screen.getByRole("link", { name: /Anime/i })).getByTestId("library-identifying-3"),
    ).toBeVisible();
    expect(screen.queryByText("Queued for identify")).not.toBeInTheDocument();
    expect(screen.queryByText("Identifying")).not.toBeInTheDocument();
  });
});
