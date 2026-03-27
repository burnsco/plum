import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// In Docker dev, the browser may need a host-facing backend URL while the Vite
// server inside the container needs to reach the backend over the Docker network.
const browserBackendUrl = process.env.VITE_BACKEND_URL;
const proxyTarget =
  process.env.BACKEND_INTERNAL_URL ||
  browserBackendUrl ||
  "http://localhost:8080";

export default defineConfig({
  cacheDir: ".vite",
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  server: {
    host: true, // Needed for Docker
    ...(browserBackendUrl
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
