import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import { SessionProvider } from "./auth/session";
import "./styles.css";

const rootEl = document.getElementById("root");
if (rootEl) {
  createRoot(rootEl).render(
    <StrictMode>
      <SessionProvider>
        <App />
      </SessionProvider>
    </StrictMode>,
  );
}
