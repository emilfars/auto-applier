// @vitest-environment jsdom

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MantineProvider } from "@mantine/core";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccountPanel } from "./AccountPanel";
import * as accountApi from "../api/account";
import * as download from "../api/download";
import { ApiError } from "../api/http";

const session = vi.hoisted(() => ({
  user: { id: "u1", email: "user@example.com", verified: true },
  refresh: vi.fn(),
}));

vi.mock("../auth/session", () => ({
  useSession: () => ({
    user: session.user,
    loading: false,
    refresh: session.refresh,
    signOut: vi.fn(),
  }),
}));

vi.mock("../api/account", async (importOriginal) => {
  const original = await importOriginal<typeof import("../api/account")>();
  return { ...original, exportAccount: vi.fn(), deleteAccount: vi.fn() };
});

vi.mock("../api/download", () => ({ downloadJSON: vi.fn() }));

const wrap = (ui: React.ReactElement) => <MantineProvider>{ui}</MantineProvider>;

const sampleExport: accountApi.AccountExport = {
  account: {
    email: "user@example.com",
    verified: true,
    created_at: "2026-01-01T00:00:00Z",
    consent_at: "2026-01-01T00:00:00Z",
  },
  profile: null,
  cv_files: [],
  exported_at: "2026-01-02T00:00:00Z",
};

describe("AccountPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    session.refresh.mockResolvedValue(undefined);
  });

  it("exports data and downloads the returned payload as JSON", async () => {
    vi.mocked(accountApi.exportAccount).mockResolvedValue(sampleExport);

    render(wrap(<AccountPanel locale="en" />));
    fireEvent.click(screen.getByTestId("account-export"));

    await waitFor(() => expect(download.downloadJSON).toHaveBeenCalledOnce());
    const [filename, data] = vi.mocked(download.downloadJSON).mock.calls[0]!;
    expect(filename).toMatch(/^auto-applier-account-export-.*\.json$/);
    expect(data).toEqual(sampleExport);
    expect(await screen.findByTestId("account-notice")).toBeTruthy();
  });

  it("keeps the confirm button disabled until the typed email matches", async () => {
    render(wrap(<AccountPanel locale="en" />));
    fireEvent.click(screen.getByTestId("account-delete-open"));

    const input = await screen.findByTestId("account-delete-email");
    const confirm = screen.getByTestId("account-delete-confirm") as HTMLButtonElement;
    expect(confirm.disabled).toBe(true);

    fireEvent.change(input, { target: { value: "wrong@example.com" } });
    expect(confirm.disabled).toBe(true);

    fireEvent.change(input, { target: { value: "user@example.com" } });
    expect(confirm.disabled).toBe(false);
  });

  it("deletes the account and refreshes the session", async () => {
    vi.mocked(accountApi.deleteAccount).mockResolvedValue(undefined);

    render(wrap(<AccountPanel locale="en" />));
    fireEvent.click(screen.getByTestId("account-delete-open"));
    fireEvent.change(await screen.findByTestId("account-delete-email"), {
      target: { value: "user@example.com" },
    });
    fireEvent.click(screen.getByTestId("account-delete-confirm"));

    await waitFor(() => expect(accountApi.deleteAccount).toHaveBeenCalledOnce());
    await waitFor(() => expect(session.refresh).toHaveBeenCalledOnce());
    expect(vi.mocked(accountApi.deleteAccount).mock.invocationCallOrder[0]).toBeLessThan(
      session.refresh.mock.invocationCallOrder[0]!,
    );
  });

  it("shows an error status when the export fails", async () => {
    vi.mocked(accountApi.exportAccount).mockRejectedValue(new ApiError(500, "export failed"));

    render(wrap(<AccountPanel locale="en" />));
    fireEvent.click(screen.getByTestId("account-export"));

    const status = await screen.findByTestId("account-status");
    expect(status.textContent).toContain("export failed");
  });
});
