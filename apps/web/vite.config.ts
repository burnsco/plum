import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// When VITE_BACKEND_URL is set, the browser talks to that backend directly for
// both HTTP and WebSocket traffic. When it is unset, Vite proxies `/api` and
// `/ws` to the internal backend target instead.
const browserBackendUrl = process.env.VITE_BACKEND_URL?.trim() || undefined;
const proxyTarget =
  process.env.BACKEND_INTERNAL_URL ||
  browserBackendUrl ||
  "http://localhost:8080";
const useDirectBackendUrl = browserBackendUrl != null;

export default defineConfig({
  cacheDir: ".vite",
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  server: {
    host: true, // Needed for Docker
    ...(useDirectBackendUrl
      ? {}
      : {
          proxy: {
            "/api": {
              target: proxyTarget,
              changeOrigin: true,
            },
            "/ws": {
              target: proxyTarget,
              ws: true,
              changeOrigin: true,
            },
          },
        }),
  },
});
