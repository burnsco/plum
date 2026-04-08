import { createContext, useCallback, useContext, useMemo, type ReactNode } from "react";
import { useSearchParams } from "react-router-dom";
import { normalizeDiscoverOriginKey } from "@plum/shared";

export type DiscoverOriginContextValue = {
  originCountry: string;
  setOriginCountry: (code: string) => void;
};

const DiscoverOriginContext = createContext<DiscoverOriginContextValue | null>(null);

/**
 * Production country from `?origin=` for Discover hub and `/discover/browse` (shared URL param).
 */
export function DiscoverOriginProvider({ children }: { children: ReactNode }) {
  const [searchParams, setSearchParams] = useSearchParams();
  const originCountry = normalizeDiscoverOriginKey(searchParams.get("origin"));
  const setOriginCountry = useCallback(
    (code: string) => {
      const normalized = normalizeDiscoverOriginKey(code);
      const next = new URLSearchParams(searchParams);
      if (normalized) {
        next.set("origin", normalized);
      } else {
        next.delete("origin");
      }
      setSearchParams(next, { replace: true });
    },
    [searchParams, setSearchParams],
  );
  const value = useMemo(
    () => ({ originCountry, setOriginCountry }),
    [originCountry, setOriginCountry],
  );
  return <DiscoverOriginContext.Provider value={value}>{children}</DiscoverOriginContext.Provider>;
}

export function useDiscoverOrigin(): DiscoverOriginContextValue {
  const ctx = useContext(DiscoverOriginContext);
  if (!ctx) {
    throw new Error("useDiscoverOrigin must be used within DiscoverOriginProvider");
  }
  return ctx;
}
