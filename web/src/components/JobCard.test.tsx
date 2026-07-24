import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { JobCard } from "./JobCard";
import type { JobCard as Job } from "../api/feed";

function makeJob(overrides: Partial<Job> = {}): Job {
  return {
    source: "kalibrr",
    source_url: "https://example.com/jobs/1",
    title: "Backend Engineer",
    company: "Nusantara Digital",
    location: "Jakarta Selatan, DKI Jakarta",
    remote: false,
    salary: {
      stated_min: 8_000_000,
      stated_max: 11_000_000,
      currency: "IDR",
      label: "Rp 8.000.000 - Rp 11.000.000",
      stated: true,
    },
    seniority: "Mid",
    employment_type: "Full Time",
    years_experience: 3,
    requirements: ["Go", "PostgreSQL"],
    posted_at: null,
    ...overrides,
  };
}

describe("JobCard", () => {
  it("shows stated employer salary", () => {
    render(<JobCard job={makeJob()} locale="en" />);
    expect(screen.getByText("Rp 8.000.000 - Rp 11.000.000")).toBeTruthy();
  });

  it("shows a neutral label when pay is not stated — never an estimate", () => {
    const job = makeJob({
      salary: { stated_min: null, stated_max: null, currency: "IDR", label: "", stated: false },
    });
    render(<JobCard job={job} locale="en" />);
    expect(screen.getByText("Salary not disclosed")).toBeTruthy();
  });

  it("links out to the original posting for the user to apply themselves", () => {
    render(<JobCard job={makeJob()} locale="en" />);
    const link = screen.getByRole("link", { name: /view & apply/i }) as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("https://example.com/jobs/1");
    expect(link.getAttribute("target")).toBe("_blank");
    expect(link.getAttribute("rel")).toContain("noopener");
  });
});
