import { describe, expect, it } from "vitest";
import type { MantineTheme } from "@mantine/core";
import { BRAND_VARIABLE_MAP, brandCssVariablesResolver, brandTheme } from "./mantine-theme";

describe("mantine brand theme", () => {
  it("routes every Mantine override through a brand token", () => {
    const out = brandCssVariablesResolver({} as MantineTheme);
    expect(out.variables).toEqual(BRAND_VARIABLE_MAP);
    const entries = Object.entries(out.variables);
    expect(entries.length).toBeGreaterThan(0);
    for (const [name, value] of entries) {
      expect(value, name).toMatch(/^var\(--brand-[a-z0-9-]+\)$/);
    }
  });

  it("wires the primary variant and surfaces to the brand tokens", () => {
    expect(BRAND_VARIABLE_MAP["--mantine-primary-color-filled"]).toBe("var(--brand-primary)");
    expect(BRAND_VARIABLE_MAP["--mantine-color-body"]).toBe("var(--brand-bg)");
    expect(BRAND_VARIABLE_MAP["--mantine-color-text"]).toBe("var(--brand-text)");
  });

  it("keeps a real palette in the primary slot for Mantine's shade math", () => {
    expect(brandTheme.primaryColor).toBe("blue");
  });
});
