// @vitest-environment jsdom

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SessionProvider, useSession } from "./session";
import * as authApi from "../api/auth";
import * as openfill from "../api/openfill";

vi.mock("../api/auth", () => ({ me: vi.fn(), logout: vi.fn() }));
vi.mock("../api/openfill", () => ({ clearExtensionState: vi.fn() }));

function Probe() {
  const { user, loading, refresh, signOut } = useSession();
  return (
    <div>
      <span data-testid="loading">{String(loading)}</span>
      <span data-testid="user">{user?.email ?? "none"}</span>
      <button onClick={() => void refresh()}>refresh</button>
      <button onClick={() => void signOut()}>signout</button>
    </div>
  );
}

describe("SessionProvider", () => {
  beforeEach(() => vi.clearAllMocks());

  it("loads the current user from /auth/me and clears loading", async () => {
    vi.mocked(authApi.me).mockResolvedValue({ id: "u1", email: "a@b.co", verified: true });
    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );
    await waitFor(() => expect(screen.getByTestId("user").textContent).toBe("a@b.co"));
    expect(screen.getByTestId("loading").textContent).toBe("false");
  });

  it("refresh re-fetches the user", async () => {
    vi.mocked(authApi.me).mockResolvedValue(null);
    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );
    await waitFor(() => expect(screen.getByTestId("loading").textContent).toBe("false"));
    expect(screen.getByTestId("user").textContent).toBe("none");

    vi.mocked(authApi.me).mockResolvedValue({ id: "u2", email: "new@b.co", verified: true });
    fireEvent.click(screen.getByRole("button", { name: "refresh" }));
    await waitFor(() => expect(screen.getByTestId("user").textContent).toBe("new@b.co"));
  });

  it("signOut clears the server session and local user", async () => {
    vi.mocked(authApi.me).mockResolvedValue({ id: "u1", email: "a@b.co", verified: true });
    vi.mocked(authApi.logout).mockResolvedValue({ status: "ok" });
    vi.mocked(openfill.clearExtensionState).mockResolvedValue(true);

    render(
      <SessionProvider>
        <Probe />
      </SessionProvider>,
    );
    await waitFor(() => expect(screen.getByTestId("user").textContent).toBe("a@b.co"));

    fireEvent.click(screen.getByRole("button", { name: "signout" }));
    await waitFor(() => expect(screen.getByTestId("user").textContent).toBe("none"));
    expect(authApi.logout).toHaveBeenCalledOnce();
    expect(openfill.clearExtensionState).toHaveBeenCalledOnce();
  });

  it("throws when useSession is used outside the provider", () => {
    const spy = vi.spyOn(console, "error").mockImplementation(() => {});
    expect(() => render(<Probe />)).toThrow(/useSession must be used within a SessionProvider/);
    spy.mockRestore();
  });
});
