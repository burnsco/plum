/** Subtitle / player race warnings: noisy in production; keep in dev for debugging. */
export function devWarn(...args: unknown[]): void {
  if (import.meta.env.DEV) {
    console.warn(...args);
  }
}
