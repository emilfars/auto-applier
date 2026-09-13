import { createTheme, type CSSVariablesResolver } from "@mantine/core";

// Brand tokens are authored as CSS custom properties in styles.css (the
// authoritative list from design/brand/palette.md). This module wires them into
// Mantine without hard-coding color literals in TypeScript, so the test suite's
// "no color literals in component source" rule keeps holding.
//
// Mantine still needs a concrete palette for internal shade math, so its
// built-in blue backs the primary slot; the resolver below remaps the variables
// Mantine actually consumes onto the brand tokens (getMergedVariables deep-
// merges the resolver over Mantine's defaults, so these win). The tokens
// themselves re-resolve per color scheme via the [data-mantine-color-scheme]
// overrides in styles.css.
export const brandTheme = createTheme({
  primaryColor: "blue",
  defaultRadius: "md",
  fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif",
  headings: { fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif" },
});

// The Mantine variables the brand tokens must drive. Kept as data so it can be
// asserted directly in tests (including that every value references a token).
export const BRAND_VARIABLE_MAP: Record<string, string> = {
  "--mantine-primary-color-filled": "var(--brand-primary)",
  "--mantine-primary-color-filled-hover": "var(--brand-primary-strong)",
  "--mantine-primary-color-light": "var(--brand-primary-soft)",
  "--mantine-primary-color-light-hover": "var(--brand-primary-soft)",
  "--mantine-primary-color-light-color": "var(--brand-primary)",
  "--mantine-primary-color-contrast": "var(--brand-on-primary)",
  "--mantine-color-anchor": "var(--brand-primary)",
  "--mantine-color-body": "var(--brand-bg)",
  "--mantine-color-text": "var(--brand-text)",
  "--mantine-color-dimmed": "var(--brand-muted)",
  "--mantine-color-placeholder": "var(--brand-muted)",
  "--mantine-color-default": "var(--brand-surface)",
  "--mantine-color-default-hover": "var(--brand-surface-2)",
  "--mantine-color-default-color": "var(--brand-text)",
  "--mantine-color-default-border": "var(--brand-border)",
  "--mantine-color-error": "var(--brand-danger)",
};

export const brandCssVariablesResolver: CSSVariablesResolver = () => ({
  variables: { ...BRAND_VARIABLE_MAP },
  dark: {},
  light: {},
});
