import { defineConfig } from "vitest/config";
import { fileURLToPath } from "node:url";

export default defineConfig({
  resolve: {
    alias: {
      "@auto-applier/fill-mappings": fileURLToPath(
        new URL("../packages/fill-mappings/src/index.ts", import.meta.url),
      ),
    },
  },
  test: {
    environment: "happy-dom",
  },
});
