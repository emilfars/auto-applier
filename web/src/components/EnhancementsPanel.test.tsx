// @vitest-environment jsdom

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MantineProvider } from "@mantine/core";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { EnhancementsPanel } from "./EnhancementsPanel";
import * as m5 from "../api/m5";
import { ApiError } from "../api/http";

const wrap = (ui: React.ReactElement) => <MantineProvider>{ui}</MantineProvider>;

const testUser = vi.hoisted(() => ({ id: "u1", email: "u@example.com", verified: true }));

vi.mock("../auth/session", () => ({
  useSession: () => ({ user: testUser }),
}));

vi.mock("../api/m5", async (importOriginal) => {
  const original = await importOriginal<typeof import("../api/m5")>();
  return {
    ...original,
    getCompleteness: vi.fn(),
    listSavedFilters: vi.fn(),
    listApplications: vi.fn(),
    listSnippets: vi.fn(),
    saveFilter: vi.fn(),
    deleteSavedFilter: vi.fn(),
    createSnippet: vi.fn(),
    deleteSnippet: vi.fn(),
    updateApplication: vi.fn(),
    confirmSubmitted: vi.fn(),
  };
});

function prime() {
  vi.mocked(m5.getCompleteness).mockResolvedValue({ score: 80, complete: false, missing: ["phone"] });
  vi.mocked(m5.listSavedFilters).mockResolvedValue([]);
  vi.mocked(m5.listApplications).mockResolvedValue([]);
  vi.mocked(m5.listSnippets).mockResolvedValue([]);
}

describe("EnhancementsPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    prime();
  });

  it("renders profile completeness and the empty tracker", async () => {
    render(wrap(<EnhancementsPanel locale="en" />));

    expect(await screen.findByText("80%")).toBeTruthy();
    expect(screen.getByText("Still needed: phone")).toBeTruthy();
    expect(screen.getByText("Applications you start will appear here.")).toBeTruthy();
  });

  it("saves a filter and shows it in the list", async () => {
    vi.mocked(m5.saveFilter).mockResolvedValue({
      id: "f2",
      name: "Remote Go",
      query: { q: "backend" },
      created_at: "",
      last_alerted_at: null,
    });
    render(wrap(<EnhancementsPanel locale="en" />));
    await screen.findByText("80%");

    fireEvent.change(screen.getByLabelText(/Filter name/), { target: { value: "Remote Go" } });
    fireEvent.change(screen.getByLabelText("Title or company search"), {
      target: { value: "backend" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save filter" }));

    await waitFor(() =>
      expect(m5.saveFilter).toHaveBeenCalledWith("Remote Go", { q: "backend" }),
    );
    expect(await screen.findByText("Remote Go")).toBeTruthy();
  });

  it("removes a saved filter", async () => {
    vi.mocked(m5.listSavedFilters).mockResolvedValue([
      { id: "f1", name: "Old filter", query: {}, created_at: "", last_alerted_at: null },
    ]);
    vi.mocked(m5.deleteSavedFilter).mockResolvedValue(undefined);
    render(wrap(<EnhancementsPanel locale="en" />));

    expect(await screen.findByText("Old filter")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Remove Old filter" }));

    await waitFor(() => expect(m5.deleteSavedFilter).toHaveBeenCalledWith("f1"));
    await waitFor(() => expect(screen.queryByText("Old filter")).toBeNull());
  });

  it("creates and deletes answer snippets", async () => {
    vi.mocked(m5.createSnippet).mockResolvedValue({
      id: "s2",
      name: "Greeting",
      body: "Hello there",
      created_at: "",
      updated_at: "",
    });
    render(wrap(<EnhancementsPanel locale="en" />));
    await screen.findByText("80%");

    fireEvent.change(screen.getByLabelText("Snippet name"), { target: { value: "Greeting" } });
    fireEvent.change(screen.getByLabelText(/Answer text/), { target: { value: "Hello there" } });
    fireEvent.click(screen.getByRole("button", { name: "Save snippet" }));

    await waitFor(() =>
      expect(m5.createSnippet).toHaveBeenCalledWith("Greeting", "Hello there"),
    );
    expect(await screen.findByText("Greeting")).toBeTruthy();
  });

  it("confirms a submitted application through the dedicated endpoint", async () => {
    const application: m5.Application = {
      id: "a1",
      company: "Acme",
      job_title: "Engineer",
      status: "form_filled",
    };
    vi.mocked(m5.listApplications).mockResolvedValue([application]);
    vi.mocked(m5.confirmSubmitted).mockResolvedValue({ ...application, status: "submitted" });
    render(wrap(<EnhancementsPanel locale="en" />));

    const select = await screen.findByRole("combobox", { name: "Status of application Engineer" });
    fireEvent.change(select, { target: { value: "submitted" } });

    await waitFor(() => expect(m5.confirmSubmitted).toHaveBeenCalledWith("a1"));
    expect(m5.updateApplication).not.toHaveBeenCalled();
  });

  it("updates a non-submitted application status via PATCH", async () => {
    const application: m5.Application = {
      id: "a1",
      company: "Acme",
      job_title: "Engineer",
      status: "form_filled",
    };
    vi.mocked(m5.listApplications).mockResolvedValue([application]);
    vi.mocked(m5.updateApplication).mockResolvedValue({ ...application, status: "interview" });
    render(wrap(<EnhancementsPanel locale="en" />));

    const select = await screen.findByRole("combobox", { name: "Status of application Engineer" });
    fireEvent.change(select, { target: { value: "interview" } });

    await waitFor(() => expect(m5.updateApplication).toHaveBeenCalledWith("a1", "interview"));
  });

  it("says it is loading, then explains a failed load and retries", async () => {
    vi.mocked(m5.listSnippets).mockRejectedValueOnce(new TypeError("Failed to fetch"));
    render(wrap(<EnhancementsPanel locale="id-ID" />));
    expect(screen.getByText("Memuat lamaran dan alat Anda…")).toBeTruthy();

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Data lamaran Anda tidak bisa dimuat.");
    expect(alert.textContent).toContain("Periksa koneksi Anda");

    fireEvent.click(screen.getByRole("button", { name: "Coba lagi" }));
    expect(await screen.findByText("Lamaran yang Anda mulai akan muncul di sini.")).toBeTruthy();
  });

  it("names tracker statuses and missing profile fields in the user's language", async () => {
    vi.mocked(m5.listApplications).mockResolvedValue([
      { id: "a1", company: "Acme", job_title: "Engineer", status: "form_filled" },
    ]);
    render(wrap(<EnhancementsPanel locale="id-ID" />));

    const select = (await screen.findByRole("combobox", { name: "Status lamaran Engineer" })) as HTMLSelectElement;
    expect([...select.options].map((o) => o.text)).toEqual([
      "Formulir terisi",
      "Terkirim (Anda konfirmasi)",
      "Dilihat",
      "Ditolak",
      "Wawancara",
    ]);
    expect(screen.getByText("Masih diperlukan: telepon")).toBeTruthy();
  });

  it("explains the empty lists and reports a rejected save without losing the input", async () => {
    vi.mocked(m5.saveFilter).mockRejectedValue(new ApiError(429, "too many requests"));
    render(wrap(<EnhancementsPanel locale="en" />));

    expect(await screen.findByText(/No saved filters yet\./)).toBeTruthy();
    expect(screen.getByText("No saved answers yet.")).toBeTruthy();

    const name = screen.getByLabelText(/Filter name/) as HTMLInputElement;
    fireEvent.change(name, { target: { value: "Remote Go" } });
    fireEvent.click(screen.getByRole("button", { name: "Save filter" }));

    expect(await screen.findByText("Too many attempts. Wait a few minutes and try again.")).toBeTruthy();
    expect(name.value).toBe("Remote Go");
  });

  it("saves a filter once when the button is pressed twice", async () => {
    let resolve: (f: m5.SavedFilter) => void = () => undefined;
    vi.mocked(m5.saveFilter).mockReturnValue(new Promise((r) => (resolve = r)));
    render(wrap(<EnhancementsPanel locale="en" />));

    fireEvent.change(await screen.findByLabelText(/Filter name/), { target: { value: "Remote Go" } });
    const form = screen.getByRole("button", { name: "Save filter" }).closest("form") as HTMLFormElement;
    fireEvent.submit(form);
    fireEvent.submit(form);
    expect(m5.saveFilter).toHaveBeenCalledOnce();

    resolve({ id: "f1", name: "Remote Go", query: {}, created_at: "", last_alerted_at: null });
    expect(await screen.findByText("Remote Go")).toBeTruthy();
  });
});
