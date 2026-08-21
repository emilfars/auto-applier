import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent, cleanup } from "@testing-library/react";
import { JobFeed } from "./JobFeed";
import type { FeedResponse } from "../api/feed";
import { t, type Locale } from "../i18n";

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

    await waitFor(() =>
      expect(fetchMock.mock.calls.some(([u]) => String(u).includes("offset=24"))).toBe(true),
    );
    expect(screen.getByText("Page 2 of 3")).toBeTruthy();
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

  it("submits and clears every filter in both locale bundles", async () => {
    for (const locale of ["en", "id-ID"] as Locale[]) {
      const fetchMock = vi.fn().mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => page(0, 1),
      });
      vi.stubGlobal("fetch", fetchMock);
      render(<JobFeed locale={locale} />);
      await waitFor(() => expect(fetchMock).toHaveBeenCalled());

      fireEvent.change(screen.getByRole("searchbox"), { target: { value: "backend" } });
      fireEvent.change(screen.getByLabelText(t(locale, "feed.filter.location")), {
        target: { value: "Jakarta" },
      });
      fireEvent.click(screen.getByLabelText(t(locale, "feed.filter.remote")));
      fireEvent.change(screen.getByLabelText(t(locale, "feed.filter.payMin")), {
        target: { value: "8000000" },
      });
      fireEvent.change(screen.getByLabelText(t(locale, "feed.filter.payMax")), {
        target: { value: "12000000" },
      });
      fireEvent.change(screen.getByLabelText(t(locale, "feed.filter.skills")), {
        target: { value: "Go, SQL" },
      });
      fireEvent.change(screen.getByLabelText(t(locale, "feed.filter.maxYoe")), {
        target: { value: "5" },
      });
      fireEvent.change(screen.getByLabelText(t(locale, "feed.filter.postedAfter")), {
        target: { value: "2026-08-01" },
      });
      fireEvent.change(screen.getByLabelText(t(locale, "feed.filter.source")), {
        target: { value: "kalibrr" },
      });
      fireEvent.change(screen.getByLabelText(t(locale, "feed.filter.employmentType")), {
        target: { value: "full_time" },
      });
      fireEvent.click(screen.getByRole("button", { name: t(locale, "feed.filter.apply") }));

      await waitFor(() => {
        const path = String(fetchMock.mock.calls.at(-1)?.[0]);
        const params = new URLSearchParams(path.split("?")[1]);
        expect(params.get("q")).toBe("backend");
        expect(params.get("location")).toBe("Jakarta");
        expect(params.get("remote")).toBe("true");
        expect(params.get("pay_min")).toBe("8000000");
        expect(params.get("pay_max")).toBe("12000000");
        expect(params.get("skills")).toBe("Go,SQL");
        expect(params.get("max_yoe")).toBe("5");
        expect(params.get("posted_after")).toBe("2026-08-01T00:00:00Z");
        expect(params.get("source")).toBe("kalibrr");
        expect(params.get("employment_type")).toBe("full_time");
      });

      fireEvent.click(screen.getByRole("button", { name: t(locale, "feed.filter.clear") }));
      await waitFor(() => expect(String(fetchMock.mock.calls.at(-1)?.[0])).toBe("/feed?limit=24&offset=0"));
      cleanup();
      vi.unstubAllGlobals();
    }
  });
});
