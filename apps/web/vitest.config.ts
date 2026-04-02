import path from "node:path";
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: "./vitest.setup.ts",
    include: ["src/**/*.test.{ts,tsx}"],
    // Fork pool spawns one Node process per worker; each loads Vite + jsdom (multi‑GB if maxWorkers tracks CPU count).
    ...(process.env.CI ? {} : { maxWorkers: 1 }),
  },
});
