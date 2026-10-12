import { createTheme, type CSSVariablesResolver } from "@mantine/core";

// Brand tokens are authored as CSS custom properties in styles.css (from
// design/brand/palette.md). This module wires them into Mantine without color
// literals in TypeScript, so the "no color literals in source" test holds.
//
// Mantine still needs a concrete palette for internal shade math, so its
// built-in blue backs the primary slot; the resolver remaps the variables
// Mantine consumes onto the brand tokens.
const SYSTEM_FONT = 'system-ui, -apple-system, "Segoe UI", Roboto, sans-serif';

export const brandTheme = createTheme({
  primaryColor: "blue",
  defaultRadius: "md",
  fontFamily: SYSTEM_FONT,
  headings: { fontFamily: SYSTEM_FONT },
  respectReducedMotion: true,
  // Cards are the one raised surface (repeated entities, see DESIGN.md);
  // Mantine would otherwise paint them with the page color.
  components: { Card: { defaultProps: { bg: "var(--mantine-color-default)" } } },
});

// Mantine color name -> brand token stem, for the colors components pass as props.
const STATE_COLORS = { teal: "success", yellow: "warning", red: "danger" } as const;

function stateVariables(): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [color, token] of Object.entries(STATE_COLORS)) {
    out[`--mantine-color-${color}-filled`] = `var(--brand-${token})`;
    out[`--mantine-color-${color}-filled-hover`] = `var(--brand-${token})`;
    out[`--mantine-color-${color}-light`] = `var(--brand-${token}-soft)`;
    out[`--mantine-color-${color}-light-hover`] = `var(--brand-${token}-soft)`;
    out[`--mantine-color-${color}-light-color`] = "var(--brand-text)";
    out[`--mantine-color-${color}-outline`] = `var(--brand-${token})`;
    out[`--mantine-color-${color}-text`] = `var(--brand-${token})`;
  }
  return out;
}

// The Mantine variables the brand tokens drive. Kept as data so tests can
// assert every value references a token.
export const BRAND_VARIABLE_MAP: Record<string, string> = {
  "--mantine-primary-color-filled": "var(--brand-primary-strong)",
  "--mantine-primary-color-filled-hover": "var(--brand-primary-hover)",
  "--mantine-primary-color-light": "var(--brand-primary-soft)",
  "--mantine-primary-color-light-hover": "var(--brand-primary-soft)",
  "--mantine-primary-color-light-color": "var(--brand-link)",
  "--mantine-primary-color-contrast": "var(--brand-on-primary)",
  "--mantine-color-blue-filled": "var(--brand-primary-strong)",
  "--mantine-color-blue-filled-hover": "var(--brand-primary-hover)",
  "--mantine-color-blue-light": "var(--brand-primary-soft)",
  "--mantine-color-blue-light-hover": "var(--brand-primary-soft)",
  "--mantine-color-blue-light-color": "var(--brand-link)",
  "--mantine-color-blue-outline": "var(--brand-link)",
  "--mantine-color-blue-text": "var(--brand-link)",
  ...stateVariables(),
  "--mantine-color-anchor": "var(--brand-link)",
  "--mantine-color-body": "var(--brand-bg)",
  "--mantine-color-text": "var(--brand-text)",
  "--mantine-color-bright": "var(--brand-text)",
  "--mantine-color-dimmed": "var(--brand-muted)",
  "--mantine-color-placeholder": "var(--brand-muted)",
  "--mantine-color-default": "var(--brand-surface)",
  "--mantine-color-default-hover": "var(--brand-surface-2)",
  "--mantine-color-default-color": "var(--brand-text)",
  "--mantine-color-default-border": "var(--brand-border-strong)",
  "--mantine-color-error": "var(--brand-danger)",
};

// Mantine paints inputs, skeletons and dividers straight from its grey scales,
// so the dark scale is mapped onto navy too (gray-4 is the light input border).
const DARK_SCALE: Record<string, string> = {
  "--mantine-color-dark-0": "var(--brand-text)",
  "--mantine-color-dark-1": "var(--brand-text)",
  "--mantine-color-dark-2": "var(--brand-muted)",
  "--mantine-color-dark-3": "var(--brand-muted)",
  "--mantine-color-dark-4": "var(--brand-border-strong)",
  "--mantine-color-dark-5": "var(--brand-surface-2)",
  "--mantine-color-dark-6": "var(--brand-surface-2)",
  "--mantine-color-dark-7": "var(--brand-surface)",
  "--mantine-color-dark-8": "var(--brand-bg)",
  "--mantine-color-dark-9": "var(--brand-bg)",
};

// Mantine sets its own values under :root[data-mantine-color-scheme], which
// outranks plain :root, so the map has to be emitted per scheme to win. The
// tokens themselves change per scheme in styles.css.
export const brandCssVariablesResolver: CSSVariablesResolver = () => ({
  variables: {},
  dark: { ...BRAND_VARIABLE_MAP, ...DARK_SCALE },
  light: { ...BRAND_VARIABLE_MAP, "--mantine-color-gray-4": "var(--brand-border-strong)" },
});
