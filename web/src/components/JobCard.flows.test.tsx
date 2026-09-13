// @vitest-environment jsdom

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MantineProvider } from "@mantine/core";
import { afterEach, describe, expect, it, vi } from "vitest";
import { JobCard } from "./JobCard";
import type { JobCard as Job } from "../api/feed";
import * as m5 from "../api/m5";

const wrap = (ui: React.ReactElement) => <MantineProvider>{ui}</MantineProvider>;

vi.mock("../api/m5", async (importOriginal) => {
  const original = await importOriginal<typeof import("../api/m5")>();
  return { ...original, setJobState: vi.fn(), recordApplication: vi.fn() };
});

function makeJob(overrides: Partial<Job> = {}): Job {
  return {
    dedup_key: "kalibrr:1",
    source: "kalibrr",
    source_url: "https://example.com/jobs/1",
    title: "Backend Engineer",
    company: "Nusantara Digital",
    location: "Jakarta",
    remote: false,
    salary: { stated_min: null, stated_max: null, currency: "IDR", label: "", stated: false },
    seniority: "",
    employment_type: "Full Time",
    years_experience: null,
    requirements: [],
    posted_at: null,
    ...overrides,
  };
}

afterEach(() => vi.clearAllMocks());

describe("JobCard signed-in actions", () => {
  it("shows an already-applied job as applied and disabled", () => {
    render(wrap(<JobCard job={makeJob({ already_applied: true })} locale="en" signedIn />));
    const applied = screen.getByRole("button", { name: "Applied" }) as HTMLButtonElement;
    expect(applied.disabled).toBe(true);
  });

  it("hides a job through the job-state endpoint and reports it to the parent", async () => {
    vi.mocked(m5.setJobState).mockResolvedValue({ job_key: "kalibrr:1", dismissed: true, already_applied: false });
    const onDismiss = vi.fn();
    render(wrap(<JobCard job={makeJob()} locale="en" signedIn onDismiss={onDismiss} />));

    fireEvent.click(screen.getByRole("button", { name: /hide/i }));
    await waitFor(() => expect(m5.setJobState).toHaveBeenCalledWith("kalibrr:1", "dismiss"));
    expect(onDismiss).toHaveBeenCalledOnce();
  });

  it("marks a job applied and flips the button to the applied state", async () => {
    vi.mocked(m5.setJobState).mockResolvedValue({ job_key: "kalibrr:1", dismissed: false, already_applied: true });
    render(wrap(<JobCard job={makeJob()} locale="en" signedIn />));

    fireEvent.click(screen.getByRole("button", { name: /mark applied/i }));
    await waitFor(() => expect(m5.setJobState).toHaveBeenCalledWith("kalibrr:1", "applied"));
    expect(await screen.findByRole("button", { name: "Applied" })).toBeTruthy();
  });

  it("records an application once a signed-in Open & Fill is armed", async () => {
    vi.mocked(m5.recordApplication).mockResolvedValue({
      id: "a1",
      company: "Acme",
      status: "form_filled",
    });
    const runOpenFill = vi.fn().mockResolvedValue({ status: "armed" });
    render(wrap(<JobCard job={makeJob()} locale="en" canFill signedIn runOpenFill={runOpenFill} />));

    fireEvent.click(screen.getByRole("button", { name: /open & fill/i }));
    await waitFor(() => expect(m5.recordApplication).toHaveBeenCalledWith("kalibrr:1"));
  });
});

describe("JobCard Open & Fill notices", () => {
  it("prompts to install the extension when none is reachable", async () => {
    const runOpenFill = vi.fn().mockResolvedValue({ status: "noExtension" });
    render(wrap(<JobCard job={makeJob()} locale="en" canFill runOpenFill={runOpenFill} />));
    fireEvent.click(screen.getByRole("button", { name: /open & fill/i }));
    expect(await screen.findByText(/Install the Auto Applier browser extension/i)).toBeTruthy();
  });

  it("shows the profile gate reason when the profile is not confirmed", async () => {
    const runOpenFill = vi.fn();
    render(
      wrap(<JobCard job={makeJob()} locale="en" canFill={false} fillReason="needProfile" runOpenFill={runOpenFill} />),
    );
    fireEvent.click(screen.getByRole("button", { name: /open & fill/i }));
    expect(await screen.findByText(/Confirm your profile before filling/i)).toBeTruthy();
    expect(runOpenFill).not.toHaveBeenCalled();
  });

  it("surfaces a generic error when the arm flow throws", async () => {
    const runOpenFill = vi.fn().mockRejectedValue(new Error("boom"));
    render(wrap(<JobCard job={makeJob()} locale="en" canFill runOpenFill={runOpenFill} />));
    fireEvent.click(screen.getByRole("button", { name: /open & fill/i }));
    await waitFor(() => expect(runOpenFill).toHaveBeenCalledOnce());
    expect(await screen.findByText(/Could not load jobs/i)).toBeTruthy();
  });
});
