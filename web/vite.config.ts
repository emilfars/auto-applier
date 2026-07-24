import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

// Minimal declaration so the config typechecks without pulling in @types/node.
declare const process: { env: Record<string, string | undefined> };

// The dev server proxies API calls to the Go backend so the SPA and API share
// an origin during local development. Override the target with VITE_API_TARGET.
const API_TARGET = process.env.VITE_API_TARGET ?? "http://localhost:8080";
const API_PREFIXES = ["/feed", "/auth", "/cv", "/profile", "/account", "/healthz"];

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: Object.fromEntries(
      API_PREFIXES.map((prefix) => [
        prefix,
        { target: API_TARGET, changeOrigin: true },
      ]),
    ),
  },
  test: {
    globals: true,
    environment: "jsdom",
  },
});
