import { useEffect, useState } from "react";
import { ActionIcon, useMantineColorScheme } from "@mantine/core";
import { IconMoon, IconSun } from "@tabler/icons-react";
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
  // Mantine v7 uses data-mantine-color-scheme for its built-in theming.
  // Keep both attributes in sync so legacy CSS and Mantine styles agree.
  document.documentElement.setAttribute("data-mantine-color-scheme", theme);
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    // Theme still applies when storage is unavailable.
  }
}

export function ThemeToggle({ locale = "en" }: { locale?: Locale }) {
  const [theme, setTheme] = useState<Theme>(readTheme);

  // Try to sync with Mantine's color scheme when inside a MantineProvider.
  // Outside the provider (e.g. in isolated tests) the hook throws — fall back
  // to the local state implementation so tests remain stable.
  let mantineScheme: ReturnType<typeof useMantineColorScheme> | null = null;
  try {
    // Resolving the hook inside a try keeps ThemeToggle usable outside a
    // MantineProvider (isolated tests); the fallback below covers that case.
    mantineScheme = useMantineColorScheme();
  } catch {
    mantineScheme = null;
  }

  useEffect(() => {
    applyTheme(theme);
    // Keep Mantine's internal scheme in sync when provider is present
    if (mantineScheme && mantineScheme.colorScheme !== theme) {
      mantineScheme.setColorScheme(theme);
    }
  }, [theme]);

  // If we're inside MantineProvider, delegate toggle to its setter so
  // notifications, modals, etc. all track the same scheme.
  const toggle = () => {
    const next: Theme = theme === "dark" ? "light" : "dark";
    setTheme(next);
    if (mantineScheme) mantineScheme.setColorScheme(next);
  };

  const current = mantineScheme?.colorScheme ?? theme;

  // Render a native button when no MantineProvider is present so isolated
  // unit tests (which render ThemeToggle without a provider) continue to pass.
  if (!mantineScheme) {
    return (
      <button
        type="button"
        aria-label={t(locale, current === "dark" ? "theme.toLight" : "theme.toDark")}
        onClick={toggle}
        title={t(locale, current === "dark" ? "theme.toLight" : "theme.toDark")}
        style={{
          display: "inline-flex",
          alignItems: "center",
          gap: 8,
          borderRadius: 9999,
          border: "1px solid var(--mantine-color-default-border)",
          padding: "6px 12px",
          fontSize: 14,
          fontWeight: 600,
          background: "var(--mantine-color-default)",
        }}
      >
        <span aria-hidden="true">{t(locale, current === "dark" ? "theme.light" : "theme.dark")}</span>
      </button>
    );
  }

  return (
    <ActionIcon
      variant="default"
      radius="xl"
      size="lg"
      aria-label={t(locale, current === "dark" ? "theme.toLight" : "theme.toDark")}
      onClick={toggle}
      title={t(locale, current === "dark" ? "theme.toLight" : "theme.toDark")}
    >
      {current === "dark" ? <IconSun size={18} /> : <IconMoon size={18} />}
    </ActionIcon>
  );
}
