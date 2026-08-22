// @vitest-environment jsdom

import { fireEvent, render, screen } from "@testing-library/react";
import { MantineProvider } from "@mantine/core";
import { describe, expect, it, vi } from "vitest";
import { AuthPanel } from "./AuthPanel";

vi.mock("../auth/session", () => ({
  useSession: () => ({
    user: null,
    loading: false,
    refresh: vi.fn(),
    signOut: vi.fn(),
  }),
}));

describe("AuthPanel signup", () => {
  it("requires explicit data-processing consent", () => {
    render(<MantineProvider><AuthPanel locale="en" /></MantineProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Sign up" }));
    const checkbox = screen.getByRole("checkbox") as HTMLInputElement;
    expect(checkbox.required).toBe(true);
    expect(checkbox.checked).toBe(false);
  });
});
