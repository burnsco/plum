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
    if (!videoElement || !assSrc) return;

    let instance: JassubInstance | null = null;
    let aborted = false;

    async function init() {
      try {
        const response = await fetch(assSrc!, { credentials: "include" });
        if (!response.ok) {
          console.error("[JassubRenderer] Failed to fetch ASS:", response.status);
          return;
        }
        const subContent = await response.text();
        if (aborted) return;

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
        if (aborted) return;

        instance = new JASSUB({
          video: videoElement,
          subContent,
          workerUrl,
          wasmUrl,
        });
      } catch (err) {
        console.error("[JassubRenderer] Initialization failed:", err);
      }
    }

    void init();

    return () => {
      aborted = true;
      void instance?.destroy();
    };
  }, [videoElement, assSrc]);

  // JASSUB manages its own canvas; no DOM output from this component.
  return null;
}
