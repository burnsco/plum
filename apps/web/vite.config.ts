import dns from "node:dns";
import path from "node:path";
import { defineConfig } from "vite";

// Align browser "localhost" resolution with the address Vite binds to. Without
// this, Node can reorder DNS results and the HMR WebSocket may fail while HTTP
// still works (see https://vite.dev/config/server-options.html#server-host).
dns.setDefaultResultOrder("verbatim");
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
  // Pre-bundle effect once into .vite/deps to avoid repeated optimize churn in dev.
  optimizeDeps: {
    include: ["effect"],
  },
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
    // Avoid two React copies (workspace deps / prebundle) breaking context hooks.
    dedupe: ["react", "react-dom"],
  },
  server: {
    host: true, // Needed for Docker
    watch: {
      ignored: [
        "**/dist/**",
        "**/coverage/**",
        "**/*.tsbuildinfo",
        // Repo-root tmp used when rotating an unwritable Vite cache (run-web-dev.sh)
        "**/tmp/**",
      ],
    },
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
