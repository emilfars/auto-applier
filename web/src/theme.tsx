import { useEffect, useState } from "react";
import { t, type Locale } from "./i18n";

export type Theme = "dark" | "light";

export const THEME_STORAGE_KEY = "auto-applier-theme";

export function readTheme(): Theme {
  if (typeof window === "undefined") return "dark";
  try {
    return window.localStorage.getItem(THEME_STORAGE_KEY) === "light" ? "light" : "dark";
  } catch {
    return "dark";
  }
}

export function applyTheme(theme: Theme): void {
  document.documentElement.dataset.theme = theme;
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    // Theme still applies when storage is unavailable.
  }
}

export function ThemeToggle({ locale = "en" }: { locale?: Locale }) {
  const [theme, setTheme] = useState<Theme>(readTheme);

  useEffect(() => applyTheme(theme), [theme]);

  return (
    <button
      type="button"
      className="theme-toggle inline-flex items-center gap-2 rounded-full border border-brand-border bg-brand-surface-2 px-3 py-1.5 text-sm font-semibold text-brand-text transition hover:border-brand-primary focus-visible:ring-2 focus-visible:ring-brand-primary"
      aria-label={t(locale, theme === "dark" ? "theme.toLight" : "theme.toDark")}
      onClick={() => setTheme((current) => (current === "dark" ? "light" : "dark"))}
    >
      <span aria-hidden="true">{theme === "dark" ? t(locale, "theme.light") : t(locale, "theme.dark")}</span>
    </button>
  );
}
