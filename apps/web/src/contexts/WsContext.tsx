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

type WsContextValue = {
  wsConnected: boolean;
  latestEvent: PlumWebSocketEvent | null;
  eventSequence: number;
  sendCommand: (command: PlumWebSocketCommand) => boolean;
};

const WsContext = createContext<WsContextValue | null>(null);

export function WsProvider({ children }: { children: ReactNode }) {
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
    if (!BASE_URL) return;
    mountedRef.current = true;

    const connect = () => {
      if (!mountedRef.current) return;
      const ws = new WebSocket(
        buildPlumWebSocketUrl(BASE_URL, window.location.origin),
      );
      wsRef.current = ws;
      ws.addEventListener("open", () => {
        if (mountedRef.current) {
          setWsConnected(true);
        }
      });
      ws.addEventListener("close", () => {
        if (!mountedRef.current) return;
        if (wsRef.current === ws) {
          wsRef.current = null;
        }
        setWsConnected(false);
        reconnectTimeoutRef.current = setTimeout(connect, 3000);
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
  }, []);

  const value = useMemo<WsContextValue>(
    () => ({
      wsConnected,
      latestEvent,
      eventSequence,
      sendCommand,
    }),
    [eventSequence, latestEvent, sendCommand, wsConnected],
  );

  return <WsContext.Provider value={value}>{children}</WsContext.Provider>;
}

export function useWs() {
  return useContext(WsContext);
}
