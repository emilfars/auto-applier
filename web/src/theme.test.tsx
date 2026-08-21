// @vitest-environment jsdom

import { fireEvent, render, screen, cleanup } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { THEME_STORAGE_KEY, ThemeToggle } from "./theme";

function contrast(foreground: [number, number, number], background: [number, number, number]): number {
  const luminance = (color: [number, number, number]) =>
    color.reduce((sum, channel, index) => {
      const value = channel / 255;
      const linear = value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
      return sum + linear * [0.2126, 0.7152, 0.0722][index];
    }, 0);
  const lighter = Math.max(luminance(foreground), luminance(background));
  const darker = Math.min(luminance(foreground), luminance(background));
  return (lighter + 0.05) / (darker + 0.05);
}

const contrastTokens: Array<{
  text: [number, number, number];
  muted: [number, number, number];
  accent: [number, number, number];
  background: [number, number, number];
  surface: [number, number, number];
}> = [
  {
    text: [255, 255, 255],
    muted: [169, 184, 216],
    accent: [90, 167, 255],
    background: [11, 24, 48],
    surface: [19, 35, 74],
  },
  {
    text: [19, 34, 63],
    muted: [82, 98, 127],
    accent: [30, 95, 208],
    background: [244, 247, 252],
    surface: [255, 255, 255],
  },
];

describe("theme", () => {
  beforeEach(() => {
    const values = new Map<string, string>();
    Object.defineProperty(window, "localStorage", {
      configurable: true,
      value: {
        clear: () => values.clear(),
        getItem: (key: string) => values.get(key) ?? null,
        setItem: (key: string, value: string) => values.set(key, value),
      },
    });
    window.localStorage.clear();
    delete document.documentElement.dataset.theme;
  });

  afterEach(() => cleanup());

  it("defaults to dark and persists the selected theme", () => {
    render(<ThemeToggle />);
    expect(document.documentElement.dataset.theme).toBe("dark");

    fireEvent.click(screen.getByRole("button", { name: "Switch to light theme" }));
    expect(document.documentElement.dataset.theme).toBe("light");
    expect(window.localStorage.getItem(THEME_STORAGE_KEY)).toBe("light");

    cleanup();
    render(<ThemeToggle />);
    expect(document.documentElement.dataset.theme).toBe("light");
  });

  it("keeps body text at WCAG AA contrast in both themes", () => {
    for (const tokens of contrastTokens) {
      const { text, muted, accent, background, surface } = tokens;
      expect(contrast(text, background)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(muted, background)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(accent, background)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(text, surface)).toBeGreaterThanOrEqual(4.5);
      expect(contrast(muted, surface)).toBeGreaterThanOrEqual(4.5);
    }
  });
});
