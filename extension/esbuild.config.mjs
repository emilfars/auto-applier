// @ts-check
/**
 * Shared esbuild configuration for the MV3 extension.
 *
 * tsc typechecks (`tsc --noEmit`) but does NOT emit the shipped bundles: the
 * source imports the shared `@auto-applier/fill-mappings` engine as a bare
 * specifier, which a browser cannot resolve. esbuild bundles each MV3 entry
 * point into a single self-contained ES module (inlining the shared engine
 * from source) so `dist/background.js` and `dist/content-entry.js` load
 * directly in Chrome.
 *
 * This only bundles — it never adds submission behaviour (Prime Directive).
 */
import { fileURLToPath } from "node:url";
import * as path from "node:path";
import * as esbuild from "esbuild";

// This file lives at the extension package root. Resolve sibling paths against
// its own directory. `import.meta.url` is a real file URL under Node, but some
// test runners (vitest) load this module under a non-file scheme — fall back to
// the current working directory, which is the package root in every runner.
const here =
  typeof import.meta.dirname === "string"
    ? import.meta.dirname
    : import.meta.url.startsWith("file:")
      ? fileURLToPath(new URL(".", import.meta.url))
      : process.cwd();
const resolve = (p) => path.resolve(here, p);

/** MV3 entry points, matching the `js` paths declared in manifest.json. */
export const entryPoints = [
  resolve("src/background.ts"),
  resolve("src/content-entry.ts"),
  resolve("src/guard-entry.ts"),
];

/** Base esbuild options shared by the build script and the bundle test. */
export const baseOptions = {
  entryPoints,
  outdir: resolve("dist"),
  bundle: true,
  format: "esm",
  target: "es2022",
  platform: "browser",
  // The shared engine is referenced by a bare specifier; resolve it to source
  // so esbuild inlines it (it is not published to node_modules).
  alias: {
    "@auto-applier/fill-mappings": resolve(
      "../packages/fill-mappings/src/index.ts",
    ),
  },
  logLevel: "silent",
};

/**
 * Run esbuild with the base options plus any overrides.
 * Pass `{ write: false }` to get in-memory `outputFiles` (used by the test).
 * @param {import('esbuild').BuildOptions} [overrides]
 */
export async function runBuild(overrides = {}) {
  return esbuild.build({ ...baseOptions, ...overrides });
}
