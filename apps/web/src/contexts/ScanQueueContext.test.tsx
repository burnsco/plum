/* oxlint-disable vitest/require-mock-type-parameters */
import { act, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Effect } from "effect";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadAuthSessionEffect } from "@plum/shared";
import * as api from "../api";
import { AuthProvider } from "./AuthContext";
import { useScanQueue } from "./ScanQueueContext";
import { ScanQueueProvider } from "./ScanQueueProvider";
import { WsProvider } from "./WsContext";

vi.mock("@plum/shared", async () => {
  const actual = await import("@plum/shared");
  return {
    ...actual,
    loadAuthSessionEffect: vi.fn(),
  };
});

/** Matches the MockWebSocket installed in vitest.setup.ts */
type GlobalWebSocketMock = {
  instances: Array<{ mockMessage: (data: string) => void }>;
  reset: () => void;
};

function getWebSocketMock(): GlobalWebSocketMock {
  return globalThis.WebSocket as unknown as GlobalWebSocketMock;
}

function ScanQueueHarness() {
  const { activeLibraryIds, activityScanStatuses } = useScanQueue();
  return (
    <div>
      <div data-testid="active-ids">{activeLibraryIds.join(",")}</div>
      <div data-testid="current-path">
        {activityScanStatuses[0]?.activity?.current?.relativePath ?? ""}
      </div>
    </div>
  );
}

describe("ScanQueueContext websocket updates", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.mocked(loadAuthSessionEffect).mockReturnValue(
      Effect.succeed({
        hasAdmin: true,
        user: {
          id: 1,
          email: "admin@example.com",
          is_admin: true,
        },
      }),
    );
    vi.spyOn(api, "getMe").mockResolvedValue({
      id: 1,
      email: "admin@example.com",
      is_admin: true,
    });
    vi.spyOn(api, "listLibraries").mockResolvedValue([
      { id: 1, name: "TV", type: "tv", path: "/tv", user_id: 1 },
    ]);
    vi.spyOn(api, "getLibraryScanStatus").mockResolvedValue({
      libraryId: 1,
      phase: "idle",
      enrichmentPhase: "idle",
      enriching: false,
      identifyPhase: "idle",
      identified: 0,
      identifyFailed: 0,
      processed: 0,
      added: 0,
      updated: 0,
      removed: 0,
      unmatched: 0,
      skipped: 0,
      identifyRequested: false,
      estimatedItems: 0,
      queuePosition: 0,
    });
    getWebSocketMock().reset();
  });

  it("applies library scan websocket updates immediately", async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <WsProvider>
            <ScanQueueProvider>
              <ScanQueueHarness />
            </ScanQueueProvider>
          </WsProvider>
        </AuthProvider>
      </QueryClientProvider>,
    );

    const wsMock = getWebSocketMock();
    await waitFor(() => {
      expect(wsMock.instances.length).toBeGreaterThan(0);
    });

    await act(async () => {
      wsMock.instances[0].mockMessage(
        JSON.stringify({
          type: "library_scan_update",
          scan: {
            libraryId: 1,
            phase: "scanning",
            enrichmentPhase: "idle",
            enriching: false,
            identifyPhase: "idle",
            identified: 0,
            identifyFailed: 0,
            processed: 3,
            added: 2,
            updated: 1,
            removed: 0,
            unmatched: 0,
            skipped: 0,
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
              ],
            },
          },
        }),
      );
      await Promise.resolve();
    });

    expect(screen.getByTestId("active-ids")).toHaveTextContent("1");
    expect(screen.getByTestId("current-path")).toHaveTextContent("Shows/Example/episode01.mkv");
  });
});
