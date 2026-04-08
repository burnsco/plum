/* oxlint-disable vitest/require-mock-type-parameters */
import { act, render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAuthActions, useAuthState } from "./AuthContext";
import { WsProvider } from "./WsContext";

vi.mock("./AuthContext", () => ({
  useAuthActions: vi.fn(),
  useAuthState: vi.fn(),
}));

type MockWebSocketHandle = {
  close: (code?: number, reason?: string) => void;
};

type MockWebSocketClass = {
  instances: MockWebSocketHandle[];
  reset: () => void;
};

async function flushMicrotasks() {
  await Promise.resolve();
  await Promise.resolve();
}

describe("WsProvider", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    (globalThis.WebSocket as unknown as MockWebSocketClass).reset();
    vi.mocked(useAuthState).mockReturnValue({
      user: {
        id: 1,
        email: "test@test.com",
        is_admin: true,
      },
      hasAdmin: true,
      loading: false,
      error: null,
    });
    vi.mocked(useAuthActions).mockReturnValue({
      login: vi.fn(),
      logout: vi.fn(),
      refreshMe: vi.fn().mockResolvedValue({
        id: 1,
        email: "test@test.com",
        is_admin: true,
      }),
      refreshSetupStatus: vi.fn(),
      clearError: vi.fn(),
    });
  });

  it("waits for auth before opening a websocket", async () => {
    vi.useFakeTimers();
    vi.mocked(useAuthState).mockReturnValue({
      user: null,
      hasAdmin: true,
      loading: true,
      error: null,
    });

    try {
      render(
        <WsProvider>
          <div>ready</div>
        </WsProvider>,
      );

      await act(async () => {
        vi.advanceTimersByTime(0);
        await flushMicrotasks();
      });

      const MockWebSocket = globalThis.WebSocket as unknown as MockWebSocketClass;
      expect(MockWebSocket.instances).toHaveLength(0);
    } finally {
      vi.useRealTimers();
    }
  });

  it("stops reconnecting when session refresh returns null", async () => {
    vi.useFakeTimers();
    const refreshMe = vi.fn().mockResolvedValue(null);
    vi.mocked(useAuthActions).mockReturnValue({
      login: vi.fn(),
      logout: vi.fn(),
      refreshMe,
      refreshSetupStatus: vi.fn(),
      clearError: vi.fn(),
    });

    try {
      render(
        <WsProvider>
          <div>ready</div>
        </WsProvider>,
      );

      await act(async () => {
        vi.advanceTimersByTime(0);
        await flushMicrotasks();
      });

      const MockWebSocket = globalThis.WebSocket as unknown as MockWebSocketClass;
      expect(MockWebSocket.instances).toHaveLength(1);

      act(() => {
        MockWebSocket.instances[0]?.close(1006, "server restarted");
      });

      await act(async () => {
        await flushMicrotasks();
      });

      await act(async () => {
        vi.advanceTimersByTime(5_000);
        await flushMicrotasks();
      });

      expect(refreshMe).toHaveBeenCalledTimes(1);
      expect(MockWebSocket.instances).toHaveLength(1);
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not retry a never-opened socket when auth refresh errors", async () => {
    vi.useFakeTimers();
    const refreshMe = vi.fn().mockRejectedValue(new Error("network"));
    vi.mocked(useAuthActions).mockReturnValue({
      login: vi.fn(),
      logout: vi.fn(),
      refreshMe,
      refreshSetupStatus: vi.fn(),
      clearError: vi.fn(),
    });

    try {
      render(
        <WsProvider>
          <div>ready</div>
        </WsProvider>,
      );

      await act(async () => {
        vi.advanceTimersByTime(0);
        await flushMicrotasks();
      });

      const MockWebSocket = globalThis.WebSocket as unknown as MockWebSocketClass;
      expect(MockWebSocket.instances).toHaveLength(1);

      act(() => {
        MockWebSocket.instances[0]?.close(1006, "handshake failed");
      });

      await act(async () => {
        await flushMicrotasks();
      });

      await act(async () => {
        vi.advanceTimersByTime(5_000);
        await flushMicrotasks();
      });

      expect(refreshMe).toHaveBeenCalledTimes(1);
      expect(MockWebSocket.instances).toHaveLength(1);
    } finally {
      vi.useRealTimers();
    }
  });
});
