import { act, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as api from "../api";
import { ScanQueueProvider, useScanQueue } from "./ScanQueueContext";
import { WsProvider } from "./WsContext";

type MockWebSocketHandle = {
  close: (code?: number, reason?: string) => void;
  mockMessage: (data: string) => void;
};

type MockWebSocketClass = {
  instances: MockWebSocketHandle[];
  reset: () => void;
};

class MockWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  static instances: MockWebSocket[] = [];

  static reset() {
    MockWebSocket.instances = [];
  }

  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  readyState = MockWebSocket.CONNECTING;

  private listeners: Record<string, Set<(event: Event) => void>> = {
    open: new Set(),
    message: new Set(),
    error: new Set(),
    close: new Set(),
  };

  constructor(_url: string) {
    MockWebSocket.instances.push(this);
    setTimeout(() => {
      this.readyState = MockWebSocket.OPEN;
      this.emit("open", new Event("open"));
    }, 0);
  }

  send() {}

  close(code?: number, reason?: string) {
    this.readyState = MockWebSocket.CLOSED;
    this.emit("close", {
      code: code ?? 1000,
      reason: reason ?? "",
      wasClean: true,
    } as CloseEvent);
  }

  addEventListener(type: string, listener: EventListener) {
    this.listeners[type]?.add(listener as (event: Event) => void);
  }

  removeEventListener(type: string, listener: EventListener) {
    this.listeners[type]?.delete(listener as (event: Event) => void);
  }

  dispatchEvent() {
    return true;
  }

  mockMessage(data: string) {
    this.emit("message", { data } as MessageEvent);
  }

  private emit(type: string, event: Event) {
    if (type === "open") this.onopen?.(event);
    if (type === "message") this.onmessage?.(event as MessageEvent);
    if (type === "close") this.onclose?.(event as CloseEvent);
    if (type === "error") this.onerror?.(event);
    for (const listener of this.listeners[type] ?? []) {
      listener(event);
    }
  }
}

(globalThis as typeof globalThis & { WebSocket: typeof WebSocket }).WebSocket =
  MockWebSocket as unknown as typeof WebSocket;

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
    vi.spyOn(api, "listLibraries").mockResolvedValue([
      { id: 1, name: "TV", type: "tv", path: "/tv", user_id: 1 },
    ]);
    vi.spyOn(api, "getLibraryScanStatus").mockResolvedValue({
      libraryId: 1,
      phase: "idle",
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
    (globalThis.WebSocket as unknown as MockWebSocketClass).reset();
  });

  it("applies library scan websocket updates immediately", async () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
      },
    });

    render(
      <QueryClientProvider client={queryClient}>
        <WsProvider>
          <ScanQueueProvider>
            <ScanQueueHarness />
          </ScanQueueProvider>
        </WsProvider>
      </QueryClientProvider>,
    );

    const MockWebSocketCtor = globalThis.WebSocket as unknown as MockWebSocketClass;
    await waitFor(() => {
      expect(MockWebSocketCtor.instances.length).toBeGreaterThan(0);
    });

    await act(async () => {
      MockWebSocketCtor.instances[0].mockMessage(
        JSON.stringify({
          type: "library_scan_update",
          scan: {
            libraryId: 1,
            phase: "scanning",
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
