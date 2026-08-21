import { describe, expect, it } from "vitest";

const sourceFiles = import.meta.glob("./**/*.{ts,tsx}", {
  eager: true,
  import: "default",
  query: "?raw",
}) as Record<string, string>;

describe("brand token usage", () => {
  it("keeps color literals out of component source", () => {
    const colorLiteral = new RegExp("#[0-9a-f]{3,8}\\b|rgba?\\(", "i");
    for (const [path, source] of Object.entries(sourceFiles)) {
      if (path.endsWith("brand-tokens.test.ts")) continue;
      const withoutAnchors = source.replace(/href="#[-a-z0-9_]+"/gi, "");
      expect(withoutAnchors, path).not.toMatch(colorLiteral);
    }
  });
});
