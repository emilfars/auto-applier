// @vitest-environment jsdom

import { fireEvent, render, screen } from "@testing-library/react";
import { MantineProvider } from "@mantine/core";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import { t } from "./i18n";

const session = vi.hoisted(() => ({
  user: null as { id: string; email: string; verified: boolean } | null,
}));

vi.mock("./auth/session", () => ({
  useSession: () => ({ user: session.user, loading: false, refresh: vi.fn(), signOut: vi.fn() }),
}));

vi.mock("./components/AuthPanel", () => ({ AuthPanel: () => <div data-testid="auth-panel" /> }));
vi.mock("./components/CvPanel", () => ({ CvPanel: () => <div data-testid="cv-panel" /> }));
vi.mock("./components/EnhancementsPanel", () => ({
  EnhancementsPanel: () => <div data-testid="enhancements" />,
}));
vi.mock("./components/ProfilePanel", () => ({
  ProfilePanel: ({ onConfirmedChange }: { onConfirmedChange?: (v: boolean) => void }) => (
    <button onClick={() => onConfirmedChange?.(true)}>probe-confirm</button>
  ),
}));
vi.mock("./components/JobFeed", () => ({
  JobFeed: ({ canFill, fillReason }: { canFill?: boolean; fillReason?: string }) => (
    <div data-testid="job-feed" data-canfill={String(canFill)} data-reason={fillReason ?? ""} />
  ),
}));

function renderApp() {
  return render(
    <MantineProvider>
      <App />
    </MantineProvider>,
  );
}

function feed() {
  return screen.getByTestId("job-feed");
}

describe("App confirm-before-apply gate", () => {
  beforeEach(() => {
    session.user = null;
  });

  it("gates Open & Fill behind login", () => {
    renderApp();
    expect(feed().getAttribute("data-canfill")).toBe("false");
    expect(feed().getAttribute("data-reason")).toBe("needLogin");
    expect(screen.queryByTestId("profile-probe")).toBeNull();
  });

  it("gates Open & Fill behind profile confirmation when signed in", () => {
    session.user = { id: "u1", email: "a@b.co", verified: true };
    renderApp();
    expect(feed().getAttribute("data-canfill")).toBe("false");
    expect(feed().getAttribute("data-reason")).toBe("needProfile");
    expect(screen.getByTestId("cv-panel")).toBeTruthy();
    expect(screen.getByTestId("enhancements")).toBeTruthy();
  });

  it("unlocks Open & Fill only after a change reports the profile confirmed", () => {
    session.user = { id: "u1", email: "a@b.co", verified: true };
    renderApp();
    fireEvent.click(screen.getByRole("button", { name: "probe-confirm" }));
    expect(feed().getAttribute("data-canfill")).toBe("true");
    expect(feed().getAttribute("data-reason")).toBe("");
  });
});

describe("App language switch", () => {
  beforeEach(() => {
    session.user = null;
  });

  it("re-renders the UI in the selected locale", () => {
    renderApp();
    expect(screen.getByText(t("id-ID", "app.tagline"))).toBeTruthy();

    fireEvent.click(screen.getByRole("textbox", { name: t("id-ID", "app.language") }));
    fireEvent.click(screen.getByRole("option", { name: "en" }));

    expect(screen.getByText(t("en", "app.tagline"))).toBeTruthy();
    expect(screen.queryByText(t("id-ID", "app.tagline"))).toBeNull();
  });
});
