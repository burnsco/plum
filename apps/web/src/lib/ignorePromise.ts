/**
 * Swallows promise rejections that are often expected (autoplay policy, fullscreen state, abort).
 * In development, logs other rejections so unexpected failures are visible.
 */

const DEFAULT_QUIET_ERROR_NAMES = new Set<string>(["AbortError", "NotAllowedError", "InvalidStateError"]);

export type IgnorePromiseOptions = {
  /** If the rejection is an `Error`/`DOMException` with this `name`, do not log in dev. */
  quietErrorNames?: ReadonlySet<string>;
};

export function ignorePromise(
  promise: PromiseLike<unknown>,
  label: string,
  options?: IgnorePromiseOptions,
): void {
  const quiet = options?.quietErrorNames ?? DEFAULT_QUIET_ERROR_NAMES;
  void Promise.resolve(promise).catch((err: unknown) => {
    if (!import.meta.env.DEV) return;
    const name =
      err instanceof Error || (typeof DOMException !== "undefined" && err instanceof DOMException)
        ? err.name
        : "";
    if (quiet.has(name)) return;
    console.warn(`[${label}]`, err);
  });
}

/** Progress/network failures: log any rejection in dev (nothing filtered). */
export function ignorePromiseAlwaysLogUnexpected(promise: PromiseLike<unknown>, label: string): void {
  void Promise.resolve(promise).catch((err: unknown) => {
    if (import.meta.env.DEV) {
      console.warn(`[${label}]`, err);
    }
  });
}
