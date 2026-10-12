import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { MantineProvider } from "@mantine/core";
import App from "./App";
import { SessionProvider } from "./auth/session";
import { brandCssVariablesResolver, brandTheme } from "./mantine-theme";
import "@mantine/core/styles.css";
import "./styles.css";

const rootEl = document.getElementById("root");
if (rootEl) {
  createRoot(rootEl).render(
    <StrictMode>
      <MantineProvider
        theme={brandTheme}
        cssVariablesResolver={brandCssVariablesResolver}
        defaultColorScheme="dark"
      >
        <SessionProvider>
          <App />
        </SessionProvider>
      </MantineProvider>
    </StrictMode>,
  );
}
