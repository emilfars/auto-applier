import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent, cleanup } from "@testing-library/react";
import { JobFeed } from "./JobFeed";
import type { FeedResponse } from "../api/feed";

function page(offset: number, total: number): FeedResponse {
  return {
    total,
    limit: 24,
    offset,
    jobs: [
      {
        source: "kalibrr",
        source_url: `https://x/${offset}`,
        title: `Job ${offset}`,
        company: "Acme",
        location: "Jakarta",
        remote: false,
        salary: { stated_min: null, stated_max: null, currency: "IDR", label: "", stated: false },
        seniority: "",
        employment_type: "full_time",
        years_experience: null,
        requirements: [],
        posted_at: null,
      },
    ],
  };
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("JobFeed pagination", () => {
  it("shows a pager and advances the offset on Next", async () => {
    const fetchMock = vi.fn().mockImplementation((url: string) => {
      const offset = url.includes("offset=24") ? 24 : 0;
      return Promise.resolve({
        ok: true,
        status: 200,
        json: async () => page(offset, 50),
      });
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<JobFeed locale="en" />);

    await waitFor(() => expect(screen.getByText("Page 1 of 3")).toBeTruthy());
    expect(screen.getByText("Job 0")).toBeTruthy();

    fireEvent.click(screen.getByText("Next"));

    await waitFor(() => expect(screen.getByText("Page 2 of 3")).toBeTruthy());
    expect(fetchMock.mock.calls.some(([u]) => String(u).includes("offset=24"))).toBe(true);
  });

  it("hides the pager when a single page covers all results", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => page(0, 10) }),
    );
    render(<JobFeed locale="en" />);
    await waitFor(() => expect(screen.getByText("Job 0")).toBeTruthy());
    expect(screen.queryByText(/Page 1 of/)).toBeNull();
  });
});
