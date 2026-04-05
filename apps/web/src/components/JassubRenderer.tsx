import { useEffect } from "react";
import type JASSUB from "jassub";

type JassubInstance = InstanceType<typeof JASSUB>;

interface JassubRendererProps {
  videoElement: HTMLVideoElement | null;
  /** URL of the raw ASS/SSA file to render. Null disables the renderer. */
  assSrc: string | null;
}

/**
 * Renders ASS/SSA subtitles using JASSUB (libass WASM port).
 * JASSUB creates and manages its own canvas overlay on top of the video element.
 * Uses subContent (pre-fetched in main thread) so auth cookies are sent correctly.
 */
export function JassubRenderer({ videoElement, assSrc }: JassubRendererProps) {
  useEffect(() => {
    const video = videoElement;
    if (!video || !assSrc) return;
    const videoEl: HTMLVideoElement = video;

    let instance: JassubInstance | null = null;
    const ac = new AbortController();
    const { signal } = ac;

    async function init() {
      try {
        const response = await fetch(assSrc!, { credentials: "include", signal });
        if (!response.ok) {
          console.error("[JassubRenderer] Failed to fetch ASS:", response.status);
          return;
        }
        const subContent = await response.text();
        if (signal.aborted) {
          console.warn("[JassubRenderer] ASS fetch completed after subtitle deselected; discarding load");
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
          console.warn("[JassubRenderer] JASSUB load aborted after subtitle deselected");
          return;
        }

        instance = new JASSUB({
          video: videoEl,
          subContent,
          workerUrl,
          wasmUrl,
        });
      } catch (err) {
        if (err instanceof DOMException && err.name === "AbortError") {
          console.warn("[JassubRenderer] ASS fetch aborted (subtitle deselected or track changed)");
          return;
        }
        console.error("[JassubRenderer] Initialization failed:", err);
      }
    }

    void init();

    return () => {
      ac.abort();
      void instance?.destroy();
    };
  }, [videoElement, assSrc]);

  // JASSUB manages its own canvas; no DOM output from this component.
  return null;
}
