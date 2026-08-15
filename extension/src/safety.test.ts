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
 * AC-SAFE-1 / Prime Directive: the extension writes into forms, so — unlike the
 * pure engine — it legitimately dispatches `input`/`change` for framework-
 * controlled inputs. It must still never contain a submit path. This scan
 * forbids every submit-triggering API and any dispatch of a submit event, while
 * allowing input/change dispatch.
 */
const FORBIDDEN: { pattern: RegExp; why: string }[] = [
  { pattern: /\.submit\s*\(/, why: "form.submit()" },
  { pattern: /requestSubmit\s*\(/, why: "form.requestSubmit()" },
  { pattern: /\.click\s*\(/, why: ".click() on a control" },
  { pattern: /SubmitEvent/, why: "synthetic SubmitEvent" },
  { pattern: /new\s+Event\s*\(\s*['"`]submit['"`]/, why: "new Event('submit')" },
  { pattern: /dispatchEvent\s*\([^)]*submit/i, why: "dispatch of a submit event" },
  { pattern: /type:\s*['"`]submit['"`]/, why: "submit-typed event init" },
];

/** Event types the extension is allowed to dispatch. */
const ALLOWED_DISPATCH = new Set(["input", "change"]);

describe("AC-SAFE-1: the extension has no submit path", () => {
  const files = sourceFiles(here).filter(
    (file) => !["guard.ts", "guard-entry.ts"].includes(file.slice(here.length + 1)),
  );

  it("scans a non-trivial set of source files", () => {
    expect(files.length).toBeGreaterThan(1);
    for (const required of ["apply.ts", "content.ts", "content-entry.ts", "background.ts"]) {
      expect(files.some((file) => file.endsWith(`/${required}`)), required).toBe(true);
    }
  });

  for (const file of files) {
    const rel = file.slice(here.length + 1);

    it(`${rel} contains no submit-triggering APIs`, () => {
      const code = readFileSync(file, "utf8");
      for (const { pattern, why } of FORBIDDEN) {
        expect(pattern.test(code), `forbidden ${why} found in ${rel}`).toBe(
          false,
        );
      }
    });

    it(`${rel} only dispatches input/change events`, () => {
      const code = readFileSync(file, "utf8");
      const re = /new\s+Event\s*\(\s*['"`]([^'"`]+)['"`]/g;
      for (const m of code.matchAll(re)) {
        const type = m[1] ?? "";
        expect(
          ALLOWED_DISPATCH.has(type),
          `disallowed dispatched event '${type}' in ${rel}`,
        ).toBe(true);
      }
    });
  }
});
