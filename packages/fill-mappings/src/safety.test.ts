import { describe, it, expect } from "vitest";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));

/** All non-test .ts source files under src/. */
function sourceFiles(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      out.push(...sourceFiles(full));
      continue;
    }
    if (!entry.endsWith(".ts")) continue;
    if (entry.endsWith(".test.ts") || entry.endsWith(".spec.ts")) continue;
    out.push(full);
  }
  return out;
}

/**
 * AC-SAFE-1 / Prime Directive: the shared fill layer must never contain code
 * that could submit an application. This is an allowlist-free static scan over
 * the runtime source — a fill path that introduces any submit-triggering API
 * fails the gate.
 */
const FORBIDDEN: { pattern: RegExp; why: string }[] = [
  { pattern: /\.submit\s*\(/, why: "form.submit()" },
  { pattern: /requestSubmit\s*\(/, why: "form.requestSubmit()" },
  { pattern: /\.click\s*\(/, why: ".click() on a control" },
  { pattern: /SubmitEvent/, why: "synthetic SubmitEvent" },
  { pattern: /dispatchEvent\s*\(/, why: "dispatchEvent (could fire submit)" },
  { pattern: /HTMLFormElement/, why: "direct form-element manipulation" },
];

describe("AC-SAFE-1: no submit-triggering APIs in the fill layer", () => {
  const files = sourceFiles(here);

  it("scans a non-trivial set of source files", () => {
    expect(files.length).toBeGreaterThan(3);
  });

  for (const file of files) {
    it(`${file.slice(here.length + 1)} contains no submit APIs`, () => {
      const code = readFileSync(file, "utf8");
      for (const { pattern, why } of FORBIDDEN) {
        expect(pattern.test(code), `forbidden ${why} found in ${file}`).toBe(false);
      }
    });
  }
});
