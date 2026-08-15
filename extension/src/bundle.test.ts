import { describe, expect, it } from "vitest";
import { runBuild } from "../esbuild.config.mjs";

/**
 * Verifies the shipped bundles are what MV3 actually loads:
 *  1. self-contained — no unresolved bare `@auto-applier/fill-mappings` import
 *     survives (Chrome can't resolve bare specifiers);
 *  2. the shared engine is genuinely inlined (not stubbed away);
 *  3. Prime Directive holds *after bundling* — the emitted code contains no
 *     form-submission path. The source-level scan (safety.test.ts) guards the
 *     inputs; this guards the build output an attacker/regression would ship.
 */
async function buildOutputs(): Promise<Record<string, string>> {
  const result = await runBuild({ write: false });
  const files: Record<string, string> = {};
  for (const out of result.outputFiles ?? []) {
    const name = out.path.split("/").pop() ?? out.path;
    files[name] = out.text;
  }
  return files;
}

// The guard bundle is the explicit defense layer and necessarily contains these
// API names. Scan the production fill/content/background bundles instead.
const FORBIDDEN_SUBMIT = [
  ".submit(",
  "requestSubmit",
  ".click(",
  "SubmitEvent",
  "new Event('submit'",
  'new Event("submit"',
  "type:'submit'",
  'type:"submit"',
];

describe("extension bundle output", () => {
  it("emits the MV3 entry bundles", async () => {
    const files = await buildOutputs();
    expect(Object.keys(files).sort()).toEqual([
      "background.js",
      "content-entry.js",
      "guard-entry.js",
    ]);
  });

  it("leaves no unresolved bare engine import (browser-loadable)", async () => {
    const files = await buildOutputs();
    for (const [name, text] of Object.entries(files)) {
      expect(
        text,
        `${name} still imports the bare engine specifier`,
      ).not.toMatch(/from\s*["']@auto-applier\/fill-mappings["']/);
      expect(text, `${name} still requires the bare engine specifier`).not.toContain(
        'require("@auto-applier/fill-mappings")',
      );
    }
  });

  it("inlines the shared fill engine into the content bundle", async () => {
    const files = await buildOutputs();
    // Portal host strings only exist in the fill-mappings maps; their presence
    // proves the engine was bundled in rather than left as an external import.
    expect(files["content-entry.js"]).toContain("greenhouse.io");
  });

  it("ships no form-submission path (Prime Directive after bundling)", async () => {
    const files = await buildOutputs();
    for (const name of ["background.js", "content-entry.js"]) {
      const text = files[name];
      if (text === undefined) throw new Error(`missing ${name} bundle`);
      for (const token of FORBIDDEN_SUBMIT) {
        expect(text, `${name} bundle contains forbidden token ${token}`).not.toContain(
          token,
        );
      }
      // Any Event constructed in shipped code must be a fill event, never
      // something that could trip a submit handler.
      const events = [...text.matchAll(/new Event\(\s*["']([^"']+)["']/g)].map(
        (m) => m[1],
      );
      for (const type of events) {
        expect(["input", "change"], `${name} dispatches Event('${type}')`).toContain(
          type,
        );
      }
    }
  });

  it("keeps submission API names isolated to the behavior-tested guard bundle", async () => {
    const files = await buildOutputs();
    expect(files["guard-entry.js"]).toContain("requestSubmit");
    expect(files["guard-entry.js"]).toContain("submit");
    expect(files["guard-entry.js"]).toContain("click");
  });
});
