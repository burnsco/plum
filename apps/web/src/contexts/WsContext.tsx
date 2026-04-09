import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  buildPlumWebSocketUrl,
  parsePlumWebSocketEvent,
  serializePlumWebSocketCommand,
  type PlumWebSocketCommand,
  type PlumWebSocketEvent,
} from "@plum/shared";
import { BASE_URL } from "../api";
import { useAuthActions, useAuthState } from "./AuthContext";

export type WsConnectionContextValue = {
  wsConnected: boolean;
  sendCommand: (command: PlumWebSocketCommand) => boolean;
};

export type WsEventContextValue = {
  latestEvent: PlumWebSocketEvent | null;
  eventSequence: number;
};

/** @deprecated Prefer `useWsConnection` / `useWsEvent` so components that only need connection state are not tied to every WS message. */
export type WsContextValue = WsConnectionContextValue & WsEventContextValue;

const WsConnectionContext = createContext<WsConnectionContextValue | null>(null);
const WsEventContext = createContext<WsEventContextValue | null>(null);

export function WsProvider({ children }: { children: ReactNode }) {
  const { user, loading } = useAuthState();
  const { refreshMe } = useAuthActions();
  const userId = user?.id ?? null;
  const [wsConnected, setWsConnected] = useState(false);
  const [latestEvent, setLatestEvent] = useState<PlumWebSocketEvent | null>(
    null,
  );
  const [eventSequence, setEventSequence] = useState(0);
  const wsRef = useRef<WebSocket | null>(null);
  const connectTimeoutRef = useRef<ReturnType<typeof setTimeout>>(0);
  const reconnectTimeoutRef = useRef<ReturnType<typeof setTimeout>>(0);
  const mountedRef = useRef(true);

  const sendCommand = useCallback((command: PlumWebSocketCommand) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      return false;
    }
    ws.send(serializePlumWebSocketCommand(command));
    return true;
  }, []);

  useEffect(() => {
    mountedRef.current = true;

    if (loading || userId == null) {
      setWsConnected(false);
      clearTimeout(connectTimeoutRef.current);
      connectTimeoutRef.current = 0;
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = 0;
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      return () => {
        mountedRef.current = false;
      };
    }

    let cancelled = false;

    const connect = () => {
      if (!mountedRef.current || cancelled || wsRef.current != null) return;
      let opened = false;
      const ws = new WebSocket(
        buildPlumWebSocketUrl(BASE_URL, window.location.origin),
      );
      wsRef.current = ws;
      ws.addEventListener("open", () => {
        opened = true;
        if (mountedRef.current) {
          setWsConnected(true);
        }
      });
      ws.addEventListener("close", () => {
        if (!mountedRef.current || cancelled) return;
        if (wsRef.current === ws) {
          wsRef.current = null;
        }
        setWsConnected(false);
        const reconnect = () => {
          if (!mountedRef.current || cancelled || wsRef.current != null) return;
          reconnectTimeoutRef.current = setTimeout(connect, 3000);
        };
        void refreshMe()
          .then((nextUser) => {
            if (nextUser == null) {
              return;
            }
            reconnect();
          })
          .catch(() => {
            if (!opened) {
              return;
            }
            reconnect();
          });
      });
      ws.addEventListener("message", (event) => {
        if (!mountedRef.current) return;
        const rawData =
          typeof event.data === "string" ? event.data : String(event.data);
        const data = parsePlumWebSocketEvent(rawData);
        if (!data) return;
        setLatestEvent(data);
        setEventSequence((value) => value + 1);
      });
    };

    connectTimeoutRef.current = setTimeout(connect, 0);
    return () => {
      cancelled = true;
      mountedRef.current = false;
      clearTimeout(connectTimeoutRef.current);
      connectTimeoutRef.current = 0;
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = 0;
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
    };
  }, [loading, refreshMe, userId]);

  const connectionValue = useMemo<WsConnectionContextValue>(
    () => ({
      wsConnected,
      sendCommand,
    }),
    [sendCommand, wsConnected],
  );

  const eventValue = useMemo<WsEventContextValue>(
    () => ({
      latestEvent,
      eventSequence,
    }),
    [eventSequence, latestEvent],
  );

  return (
    <WsConnectionContext.Provider value={connectionValue}>
      <WsEventContext.Provider value={eventValue}>{children}</WsEventContext.Provider>
    </WsConnectionContext.Provider>
  );
}

export function useWsConnection(): WsConnectionContextValue {
  const ctx = useContext(WsConnectionContext);
  if (!ctx) {
    throw new Error("useWsConnection must be used within WsProvider");
  }
  return ctx;
}

export function useWsEvent(): WsEventContextValue {
  const ctx = useContext(WsEventContext);
  if (!ctx) {
    throw new Error("useWsEvent must be used within WsProvider");
  }
  return ctx;
}

/** Subscribes to both connection and event streams; re-renders on every WS message. Prefer `useWsConnection` / `useWsEvent` when you only need one. */
export function useWs(): WsContextValue {
  const { wsConnected, sendCommand } = useWsConnection();
  const { latestEvent, eventSequence } = useWsEvent();
  return useMemo(
    () => ({
      wsConnected,
      sendCommand,
      latestEvent,
      eventSequence,
    }),
    [eventSequence, latestEvent, sendCommand, wsConnected],
  );
}
