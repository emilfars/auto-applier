// @vitest-environment jsdom

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MantineProvider } from "@mantine/core";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { EnhancementsPanel } from "./EnhancementsPanel";
import * as m5 from "../api/m5";

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

    fireEvent.change(screen.getByLabelText("Filter name"), { target: { value: "Remote Go" } });
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
    fireEvent.click(screen.getByRole("button", { name: "Remove" }));

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
    fireEvent.change(screen.getByLabelText(/Use \{name\}/), { target: { value: "Hello there" } });
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

    const select = await screen.findByRole("textbox", { name: "Engineer" });
    fireEvent.click(select);
    fireEvent.click(await screen.findByRole("option", { name: "submitted" }));

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

    const select = await screen.findByRole("textbox", { name: "Engineer" });
    fireEvent.click(select);
    fireEvent.click(await screen.findByRole("option", { name: "interview" }));

    await waitFor(() => expect(m5.updateApplication).toHaveBeenCalledWith("a1", "interview"));
  });
});
