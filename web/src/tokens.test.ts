import { describe, expect, it } from "vitest";
import type { MantineTheme } from "@mantine/core";
import styles from "./styles.css?raw";
import { BRAND_VARIABLE_MAP, brandCssVariablesResolver } from "./mantine-theme";

type Theme = Record<string, string>;

function block(selector: string): Theme {
  const start = styles.indexOf(`${selector} {`);
  if (start < 0) throw new Error(`no ${selector} block in styles.css`);
  const body = styles.slice(start, styles.indexOf("}", start));
  return Object.fromEntries([...body.matchAll(/(--brand-[a-z0-9-]+):\s*([^;]+);/g)].map((m) => [m[1], m[2].trim()]));
}

const dark = block(":root");
const light = { ...dark, ...block('[data-mantine-color-scheme="light"]') };

function resolve(theme: Theme, token: string): [number, number, number] {
  let value = theme[token];
  for (let hop = 0; hop < 5 && value?.startsWith("var("); hop++) value = theme[value.slice(4, -1)];
  const hex = /^#([0-9a-f]{6})$/i.exec(value ?? "");
  if (!hex) throw new Error(`${token} does not resolve to a hex color: ${value}`);
  return [0, 2, 4].map((i) => parseInt(hex[1].slice(i, i + 2), 16)) as [number, number, number];
}

function contrast(theme: Theme, foreground: string, background: string): number {
  const luminance = (token: string) =>
    resolve(theme, token).reduce((sum, channel, index) => {
      const value = channel / 255;
      const linear = value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
      return sum + linear * [0.2126, 0.7152, 0.0722][index];
    }, 0);
  const [a, b] = [luminance(foreground), luminance(background)];
  return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}

const SURFACES = ["--brand-bg", "--brand-surface", "--brand-surface-2"];
const TEXT = ["--brand-text", "--brand-muted", "--brand-link", "--brand-success", "--brand-warning", "--brand-danger"];

describe.each([
  ["dark", dark],
  ["light", light],
])("brand tokens, %s theme (AC-WEB-4, E-A11Y)", (_name, theme) => {
  it.each(TEXT.flatMap((text) => SURFACES.map((surface) => [text, surface])))(
    "%s on %s is at least 4.5:1",
    (text, surface) => {
      expect(contrast(theme, text, surface)).toBeGreaterThanOrEqual(4.5);
    },
  );

  it("filled buttons keep 4.5:1 for their label, at rest and on hover", () => {
    expect(contrast(theme, "--brand-on-primary", "--brand-primary-strong")).toBeGreaterThanOrEqual(4.5);
    expect(contrast(theme, "--brand-on-primary", "--brand-primary-hover")).toBeGreaterThanOrEqual(4.5);
  });

  it.each(["--brand-success", "--brand-warning", "--brand-danger"])("text on a filled %s surface is at least 4.5:1", (state) => {
    expect(contrast(theme, "--brand-on-state", state)).toBeGreaterThanOrEqual(4.5);
  });

  it.each(SURFACES)("input borders and the focus ring reach 3:1 on %s", (surface) => {
    expect(contrast(theme, "--brand-border-strong", surface)).toBeGreaterThanOrEqual(3);
    expect(contrast(theme, "--brand-link", surface)).toBeGreaterThanOrEqual(3);
  });
});

describe("Mantine variable resolver", () => {
  it("emits the brand map per color scheme so it outranks Mantine's scheme rules", () => {
    const out = brandCssVariablesResolver({} as MantineTheme);
    expect(out.dark).toMatchObject(BRAND_VARIABLE_MAP);
    expect(out.light).toMatchObject(BRAND_VARIABLE_MAP);
  });

  it("repaints Mantine's grey scales, which inputs and dividers read directly", () => {
    const out = brandCssVariablesResolver({} as MantineTheme);
    for (let shade = 0; shade <= 9; shade++) {
      expect(out.dark[`--mantine-color-dark-${shade}`], `dark-${shade}`).toMatch(/^var\(--brand-/);
    }
    expect(out.light["--mantine-color-gray-4"]).toBe("var(--brand-border-strong)");
  });

  it("maps every Mantine color the components use onto a brand token", () => {
    for (const color of ["blue", "teal", "yellow", "red"]) {
      for (const part of ["filled", "filled-hover", "light", "light-hover", "light-color", "outline", "text"]) {
        expect(BRAND_VARIABLE_MAP[`--mantine-color-${color}-${part}`], `${color}-${part}`).toMatch(/^var\(--brand-/);
      }
    }
  });
});
