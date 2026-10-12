// @vitest-environment jsdom

import { render, screen } from "@testing-library/react";
import { MantineProvider } from "@mantine/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ErrorBoundary } from "./ErrorBoundary";

function Broken(): never {
  throw new Error("render failed with someone@example.com in it");
}

describe("ErrorBoundary", () => {
  afterEach(() => vi.restoreAllMocks());

  it("replaces a crashed tree with an explanation and a reload action", () => {
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    render(
      <MantineProvider>
        <ErrorBoundary>
          <Broken />
        </ErrorBoundary>
      </MantineProvider>,
    );

    const alert = screen.getByRole("alert");
    expect(alert.textContent).toContain("Halaman ini bermasalah");
    expect(alert.textContent).not.toContain("someone@example.com");
    expect(screen.getByRole("button", { name: "Muat ulang halaman" })).toBeTruthy();
  });

  it("renders its children when nothing fails", () => {
    render(
      <MantineProvider>
        <ErrorBoundary>
          <p>fine</p>
        </ErrorBoundary>
      </MantineProvider>,
    );
    expect(screen.getByText("fine")).toBeTruthy();
  });
});
