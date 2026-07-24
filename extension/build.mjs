// @ts-check
/**
 * Build entry: produces the browser-loadable MV3 bundles in `dist/`.
 * Cleans stale output first so removed source files can't linger.
 */
import { rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { runBuild } from "./esbuild.config.mjs";

const dist = fileURLToPath(new URL("dist", import.meta.url));

await rm(dist, { recursive: true, force: true });
await runBuild({ write: true, logLevel: "info" });
