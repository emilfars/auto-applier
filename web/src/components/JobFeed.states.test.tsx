import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MantineProvider } from "@mantine/core";
import { JobFeed } from "./JobFeed";
import type { FeedResponse } from "../api/feed";

const wrap = (ui: React.ReactElement) => <MantineProvider>{ui}</MantineProvider>;

const empty: FeedResponse = { total: 0, limit: 24, offset: 0, jobs: [] };
const oneJob: FeedResponse = {
  total: 1,
  limit: 24,
  offset: 0,
  jobs: [
    {
      source: "kalibrr",
      source_url: "https://x/1",
      title: "Backend Engineer",
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

afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("JobFeed states", () => {
  it("shows the empty state when no listings match", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => empty }),
    );
    render(wrap(<JobFeed locale="en" />));
    expect(await screen.findByText("No jobs match your filters.")).toBeTruthy();
    expect(screen.getByText("0 matching jobs")).toBeTruthy();
  });

  it("shows an error state when the feed request fails", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 500, json: async () => ({}) }));
    render(wrap(<JobFeed locale="en" />));
    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("Could not load jobs");
  });

  it("shows a loading state before the first response resolves", () => {
    vi.stubGlobal("fetch", vi.fn().mockReturnValue(new Promise(() => {})));
    render(wrap(<JobFeed locale="en" />));
    expect(screen.getByText(/Loading jobs/)).toBeTruthy();
  });
});

describe("JobFeed debounced search", () => {
  beforeEach(() => vi.useFakeTimers());

  it("waits for the debounce window and sends only the settled query", async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => oneJob });
    vi.stubGlobal("fetch", fetchMock);

    render(wrap(<JobFeed locale="en" />));
    await act(async () => {
      vi.advanceTimersByTime(300);
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    fetchMock.mockClear();
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "go" } });
    await act(async () => {
      vi.advanceTimersByTime(100);
    });
    expect(fetchMock).not.toHaveBeenCalled();

    await act(async () => {
      vi.advanceTimersByTime(200);
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(String(fetchMock.mock.calls[0][0])).toContain("q=go");
  });
});

describe("JobFeed results", () => {
  it("renders an employer-stated salary label", async () => {
    const job: FeedResponse = {
      ...oneJob,
      jobs: [
        {
          ...oneJob.jobs[0],
          salary: {
            stated_min: 8_000_000,
            stated_max: 11_000_000,
            currency: "IDR",
            label: "Rp 8.000.000 - Rp 11.000.000",
            stated: true,
          },
        },
      ],
    };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => job }));
    render(wrap(<JobFeed locale="en" />));
    await waitFor(() => expect(screen.getByText("Backend Engineer")).toBeTruthy());
    expect(screen.getByText("Rp 8.000.000 - Rp 11.000.000")).toBeTruthy();
  });

  it("shows a neutral label when pay is neither stated nor estimated", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => oneJob }));
    render(wrap(<JobFeed locale="en" />));
    await waitFor(() => expect(screen.getByText("Backend Engineer")).toBeTruthy());
    expect(screen.getByText("Salary not disclosed")).toBeTruthy();
  });
});
