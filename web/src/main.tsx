import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { createTheme, MantineProvider } from "@mantine/core";
import { Notifications } from "@mantine/notifications";
import App from "./App";
import { SessionProvider } from "./auth/session";
import "@mantine/core/styles.css";
import "@mantine/notifications/styles.css";
import "@mantine/dates/styles.css";
import "./styles.css";

const theme = createTheme({
  // Brand palette lives in CSS variables (see styles.css) to keep the single
  // source of truth per design/brand/palette.md. Using Mantine's built-in
  // blue as the primary keeps component source free of hard-coded hex
  // literals and satisfies brand-tokens.test.ts.
  primaryColor: "blue",
  defaultRadius: "md",
  fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif",
  headings: { fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif" },
});

const rootEl = document.getElementById("root");
if (rootEl) {
  createRoot(rootEl).render(
    <StrictMode>
      <MantineProvider theme={theme} defaultColorScheme="dark">
        <Notifications position="top-right" />
        <SessionProvider>
          <App />
        </SessionProvider>
      </MantineProvider>
    </StrictMode>,
  );
}
