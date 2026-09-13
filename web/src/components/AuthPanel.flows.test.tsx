// @vitest-environment jsdom

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MantineProvider } from "@mantine/core";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AuthPanel } from "./AuthPanel";
import { ApiError } from "../api/http";
import * as authApi from "../api/auth";

const wrap = (ui: React.ReactElement) => <MantineProvider>{ui}</MantineProvider>;

const session = vi.hoisted(() => ({
  user: null as { id: string; email: string; verified: boolean } | null,
  refresh: vi.fn(),
  signOut: vi.fn(),
}));

vi.mock("../auth/session", () => ({
  useSession: () => ({
    user: session.user,
    loading: false,
    refresh: session.refresh,
    signOut: session.signOut,
  }),
}));

vi.mock("../api/auth", () => ({
  register: vi.fn(),
  verify: vi.fn(),
  login: vi.fn(),
  requestReset: vi.fn(),
  confirmReset: vi.fn(),
}));

function fillCredentials() {
  fireEvent.change(screen.getByLabelText(/Email/), { target: { value: "a@b.co" } });
  fireEvent.change(screen.getByLabelText(/Password/), { target: { value: "password123" } });
}

describe("AuthPanel flows", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    session.user = null;
  });

  it("blocks signup until consent is given", () => {
    render(wrap(<AuthPanel locale="en" />));
    fireEvent.click(screen.getByRole("button", { name: "Sign up" }));
    fillCredentials();
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));

    expect(authApi.register).not.toHaveBeenCalled();
    expect((screen.getByRole("checkbox") as HTMLInputElement).required).toBe(true);
  });

  it("registers with consent, prefills the emailed token, and verifies", async () => {
    vi.mocked(authApi.register).mockResolvedValue({
      id: "u1",
      email: "a@b.co",
      verified: false,
      consent: true,
      verification_token: "tok-123",
    });
    vi.mocked(authApi.verify).mockResolvedValue({ status: "verified" });

    render(wrap(<AuthPanel locale="en" />));
    fireEvent.click(screen.getByRole("button", { name: "Sign up" }));
    fillCredentials();
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));

    await waitFor(() =>
      expect(authApi.register).toHaveBeenCalledWith("a@b.co", "password123", true),
    );
    expect(await screen.findByText(/Check your email to verify/i)).toBeTruthy();
    expect((screen.getByLabelText(/Verification token/) as HTMLInputElement).value).toBe("tok-123");

    fireEvent.click(screen.getByRole("button", { name: "Verify" }));
    await waitFor(() => expect(authApi.verify).toHaveBeenCalledWith("tok-123"));
    expect(await screen.findByText(/Account verified/i)).toBeTruthy();
  });

  it("surfaces a server error from registration", async () => {
    vi.mocked(authApi.register).mockRejectedValue(new ApiError(409, "email already registered"));
    render(wrap(<AuthPanel locale="en" />));
    fireEvent.click(screen.getByRole("button", { name: "Sign up" }));
    fillCredentials();
    fireEvent.click(screen.getByRole("checkbox"));
    fireEvent.click(screen.getByRole("button", { name: "Create account" }));

    expect(await screen.findByText("email already registered")).toBeTruthy();
  });

  it("logs in and refreshes the session", async () => {
    vi.mocked(authApi.login).mockResolvedValue({ status: "authenticated" });
    render(wrap(<AuthPanel locale="en" />));
    fillCredentials();
    const signIn = screen.getAllByRole("button", { name: "Sign in" });
    fireEvent.click(signIn[signIn.length - 1]);

    await waitFor(() => expect(authApi.login).toHaveBeenCalledWith("a@b.co", "password123"));
    await waitFor(() => expect(session.refresh).toHaveBeenCalledOnce());
  });

  it("requests a reset token and confirms the new password", async () => {
    vi.mocked(authApi.requestReset).mockResolvedValue({ status: "ok", reset_token: "rt-1" });
    vi.mocked(authApi.confirmReset).mockResolvedValue({ status: "ok" });

    render(wrap(<AuthPanel locale="en" />));
    fireEvent.change(screen.getByLabelText(/Email/), { target: { value: "a@b.co" } });
    fireEvent.click(screen.getByRole("button", { name: /forgot password/i }));
    fireEvent.click(screen.getByRole("button", { name: "Send reset token" }));

    await waitFor(() => expect(authApi.requestReset).toHaveBeenCalledWith("a@b.co"));
    expect(await screen.findByText(/reset token has been sent/i)).toBeTruthy();
    expect((screen.getByLabelText(/Reset token/) as HTMLInputElement).value).toBe("rt-1");

    fireEvent.change(screen.getByLabelText(/New password/), {
      target: { value: "newpassword1" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Set new password" }));
    await waitFor(() => expect(authApi.confirmReset).toHaveBeenCalledWith("rt-1", "newpassword1"));
    expect(await screen.findByText(/Password updated/i)).toBeTruthy();
  });

  it("signs out an authenticated user", async () => {
    session.user = { id: "u1", email: "a@b.co", verified: true };
    render(wrap(<AuthPanel locale="en" />));
    expect(screen.getByText(/Signed in as/)).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Sign out" }));
    expect(session.signOut).toHaveBeenCalledOnce();
  });
});
