import { useEffect } from "react";
import { devWarn } from "@/lib/devConsole";
import type JASSUB from "jassub";

type JassubInstance = InstanceType<typeof JASSUB>;

// Trigger dynamic imports eagerly so modules are cached by the time a subtitle is selected.
// Errors are intentionally swallowed — the init() function will retry and handle them.
void Promise.all([
  import("jassub"),
  // @ts-ignore — ?url import resolved by Vite
  import("jassub/dist/wasm/jassub-worker.js?url"),
  // @ts-ignore — ?url import resolved by Vite
  import("jassub/dist/wasm/jassub-worker.wasm?url"),
]).catch(() => {});

interface JassubRendererProps {
  videoElement: HTMLVideoElement | null;
  /** URL of the raw ASS/SSA file to render. Null disables the renderer. */
  assSrc: string | null;
  onStatusChange?: (status: "loading" | "ready" | "error" | "timeout") => void;
}

/**
 * Renders ASS/SSA subtitles using JASSUB (libass WASM port).
 * JASSUB creates and manages its own canvas overlay on top of the video element.
 * Uses subContent (pre-fetched in main thread) so auth cookies are sent correctly.
 */
const ASS_LOAD_TIMEOUT_MS = 45_000;
const EMBEDDED_ASS_LOAD_TIMEOUT_MS = 600_000;

export function JassubRenderer({
  videoElement,
  assSrc,
  onStatusChange,
}: JassubRendererProps) {
  useEffect(() => {
    const video = videoElement;
    if (!video || !assSrc) return;
    const videoEl: HTMLVideoElement = video;

    let instance: JassubInstance | null = null;
    const ac = new AbortController();
    const { signal } = ac;
    let timedOut = false;
    const timeoutMs = assSrc.includes("/subtitles/embedded/")
      ? EMBEDDED_ASS_LOAD_TIMEOUT_MS
      : ASS_LOAD_TIMEOUT_MS;
    const timeoutId = window.setTimeout(() => {
      timedOut = true;
      ac.abort();
    }, timeoutMs);

    async function init() {
      try {
        onStatusChange?.("loading");
        const response = await fetch(assSrc!, {
          credentials: "include",
          signal,
        });
        if (!response.ok) {
          console.error(
            "[JassubRenderer] Failed to fetch ASS:",
            response.status,
          );
          onStatusChange?.("error");
          return;
        }
        const subContent = await response.text();
        if (signal.aborted) {
          devWarn(
            "[JassubRenderer] ASS fetch completed after subtitle deselected; discarding load",
          );
          return;
        }

        const [
          { default: JASSUB },
          { default: workerUrl },
          { default: wasmUrl },
        ] = await Promise.all([
          import("jassub"),
          // eslint-disable-next-line @typescript-eslint/ban-ts-comment
          // @ts-ignore — ?url import resolved by Vite
          import("jassub/dist/wasm/jassub-worker.js?url"),
          // eslint-disable-next-line @typescript-eslint/ban-ts-comment
          // @ts-ignore — ?url import resolved by Vite
          import("jassub/dist/wasm/jassub-worker.wasm?url"),
        ]);
        if (signal.aborted) {
          devWarn(
            "[JassubRenderer] JASSUB load aborted after subtitle deselected",
          );
          return;
        }

        instance = new JASSUB({
          video: videoEl,
          subContent,
          workerUrl,
          wasmUrl,
        });
        onStatusChange?.("ready");
      } catch (err) {
        if (err instanceof DOMException && err.name === "AbortError") {
          if (timedOut) {
            onStatusChange?.("timeout");
          }
          devWarn(
            "[JassubRenderer] ASS fetch aborted (subtitle deselected or track changed)",
          );
          return;
        }
        onStatusChange?.("error");
        console.error("[JassubRenderer] Initialization failed:", err);
      }
    }

    void init();

    return () => {
      window.clearTimeout(timeoutId);
      ac.abort();
      void instance?.destroy();
    };
  }, [assSrc, onStatusChange, videoElement]);

  // JASSUB manages its own canvas; no DOM output from this component.
  return null;
}
