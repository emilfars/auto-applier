// @vitest-environment jsdom

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MantineProvider } from "@mantine/core";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CvPanel } from "./CvPanel";
import * as cvApi from "../api/cv";
import { ApiError } from "../api/http";

const wrap = (ui: React.ReactElement) => <MantineProvider>{ui}</MantineProvider>;

const testUser = vi.hoisted(() => ({ id: "u1", email: "u@example.com", verified: true }));

vi.mock("../auth/session", () => ({
  useSession: () => ({ user: testUser }),
}));

vi.mock("../api/cv", async (importOriginal) => {
  const original = await importOriginal<typeof import("../api/cv")>();
  return {
    ...original,
    listCVs: vi.fn(),
    uploadCV: vi.fn(),
    parseCV: vi.fn(),
    updateCVVersion: vi.fn(),
  };
});

const cv = (id: string, over: Partial<cvApi.CVFile> = {}): cvApi.CVFile => ({
  id,
  filename: `${id}.pdf`,
  content_type: "application/pdf",
  size_bytes: 2048,
  created_at: "2026-01-01T00:00:00Z",
  ...over,
});

describe("CvPanel", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(cvApi.listCVs).mockResolvedValue([]);
  });

  afterEach(() => vi.unstubAllGlobals());

  it("shows the empty state when no CVs are uploaded", async () => {
    render(wrap(<CvPanel locale="en" />));
    expect(await screen.findByText("No CVs uploaded yet.")).toBeTruthy();
  });

  it("lists stored CVs with a formatted size and primary badge", async () => {
    vi.mocked(cvApi.listCVs).mockResolvedValue([
      cv("a", { is_primary: true }),
      cv("b", { size_bytes: 1024 * 1024 * 2, label: "Updated CV" }),
    ]);
    render(wrap(<CvPanel locale="en" />));

    expect(await screen.findByText("a.pdf")).toBeTruthy();
    expect(screen.getByText("(2 KB)")).toBeTruthy();
    expect(screen.getByText("Updated CV")).toBeTruthy();
    expect(screen.getByText("(2.0 MB)")).toBeTruthy();
    expect(screen.getByText("Primary")).toBeTruthy();
  });

  it("uploads a selected file and prepends it to the list", async () => {
    vi.mocked(cvApi.uploadCV).mockResolvedValue(cv("new"));
    const { container } = render(wrap(<CvPanel locale="en" />));
    await screen.findByText("No CVs uploaded yet.");

    const input = container.querySelector('input[type="file"]') as HTMLInputElement;
    const file = new File(["cv"], "resume.pdf", { type: "application/pdf" });
    fireEvent.change(input, { target: { files: [file] } });
    fireEvent.click(screen.getByRole("button", { name: /upload cv/i }));

    await waitFor(() => expect(cvApi.uploadCV).toHaveBeenCalledOnce());
    expect((vi.mocked(cvApi.uploadCV).mock.calls[0][0] as File).name).toBe("resume.pdf");
    expect(await screen.findByText("CV uploaded.")).toBeTruthy();
  });

  it("parses a CV and notifies the parent so the profile reloads", async () => {
    vi.mocked(cvApi.listCVs).mockResolvedValue([cv("a")]);
    vi.mocked(cvApi.parseCV).mockResolvedValue({ status: "ok" });
    const onParsed = vi.fn();
    render(wrap(<CvPanel locale="en" onParsed={onParsed} />));

    fireEvent.click(await screen.findByRole("button", { name: /parse into profile/i }));

    await waitFor(() => expect(cvApi.parseCV).toHaveBeenCalledWith("a"));
    expect(onParsed).toHaveBeenCalledOnce();
    expect(await screen.findByText(/Parsed into your profile/i)).toBeTruthy();
  });

  it("explains that parsing is unavailable on a 503 instead of a generic error", async () => {
    vi.mocked(cvApi.listCVs).mockResolvedValue([cv("a")]);
    vi.mocked(cvApi.parseCV).mockRejectedValue(new ApiError(503, "cv parsing is not available"));
    render(wrap(<CvPanel locale="en" />));

    fireEvent.click(await screen.findByRole("button", { name: /parse into profile/i }));

    expect(
      await screen.findByText(/CV parsing is not available right now/i),
    ).toBeTruthy();
  });

  it("promotes a CV to primary and clears the badge from the previous one", async () => {
    vi.mocked(cvApi.listCVs).mockResolvedValue([cv("a", { is_primary: true }), cv("b")]);
    vi.mocked(cvApi.updateCVVersion).mockResolvedValue(cv("b", { is_primary: true }));
    render(wrap(<CvPanel locale="en" />));

    await screen.findByText("a.pdf");
    expect(screen.queryAllByText("Primary")).toHaveLength(1);

    fireEvent.click(screen.getByRole("button", { name: /use as primary/i }));

    await waitFor(() =>
      expect(cvApi.updateCVVersion).toHaveBeenCalledWith("b", { is_primary: true }),
    );
    expect(await screen.findByText("Primary CV updated.")).toBeTruthy();
    expect(screen.queryAllByText("Primary")).toHaveLength(1);
  });
});
