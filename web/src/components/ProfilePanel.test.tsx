// @vitest-environment jsdom

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MantineProvider } from "@mantine/core";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ProfilePanel } from "./ProfilePanel";
import * as profileApi from "../api/profile";

const wrap = (ui: React.ReactElement) => <MantineProvider>{ui}</MantineProvider>;

vi.mock("../auth/session", () => ({
  useSession: () => ({ user: true }),
}));

vi.mock("../api/profile", async (importOriginal) => {
  const original = await importOriginal<typeof import("../api/profile")>();
  return {
    ...original,
    getProfile: vi.fn(),
    patchProfile: vi.fn(),
    confirmProfile: vi.fn(),
  };
});

const profile: profileApi.Profile = {
  full_name: "Dina",
  email: "dina@example.com",
  phone: "+62812",
  linkedin_url: "https://linkedin.com/in/dina",
  github_url: "https://github.com/dina",
  portfolio_url: "https://dina.example.com",
  address: "Jl. Sudirman 1",
  city: "Jakarta",
  summary: "Engineer",
  current_employer: "Acme",
  current_title: "Engineer",
  highest_education: "S.Kom",
  education: [{ institution: "UI", degree: "S.Kom", field: "CS", start_year: "2020", end_year: "2024" }],
  work_history: [{ company: "Acme", title: "Engineer", start_date: "2024", end_date: "" }],
  skills: ["Go"],
  expected_salary: null,
  notice_period_days: null,
  work_authorization: "WNI",
  open_to_relocation: false,
  preferred_locations: ["Jakarta"],
  employment_type: "full_time",
  confirmed: false,
  confirmed_at: null,
};

describe("ProfilePanel", () => {
  beforeEach(() => vi.clearAllMocks());

  it("edits and saves every parsed field", async () => {
    vi.mocked(profileApi.getProfile).mockResolvedValue(profile);
    vi.mocked(profileApi.patchProfile).mockResolvedValue(profile);

    render(wrap(<ProfilePanel locale="en" />));

    const education = await screen.findByLabelText(/Education \(JSON\)/);
    const workHistory = screen.getByLabelText(/Work history \(JSON\)/);
    expect((education as HTMLTextAreaElement).value).toContain('"institution": "UI"');
    expect((workHistory as HTMLTextAreaElement).value).toContain('"company": "Acme"');

    fireEvent.change(screen.getByLabelText(/Email/), { target: { value: "new@example.com" } });
    fireEvent.change(screen.getByLabelText(/LinkedIn URL/), {
      target: { value: "https://linkedin.com/in/new" },
    });
    fireEvent.change(screen.getByLabelText(/Professional summary/), {
      target: { value: "New summary" },
    });
    fireEvent.change(education, {
      target: { value: '[{"institution":"ITB","degree":"S.T.","field":"","start_year":"","end_year":""}]' },
    });
    fireEvent.change(workHistory, {
      target: { value: '[{"company":"New Co","title":"Lead","start_date":"","end_date":""}]' },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));

    await waitFor(() =>
      expect(profileApi.patchProfile).toHaveBeenCalledWith(
        expect.objectContaining({
          email: "new@example.com",
          linkedin_url: "https://linkedin.com/in/new",
          summary: "New summary",
          education: [expect.objectContaining({ institution: "ITB" })],
          work_history: [expect.objectContaining({ company: "New Co" })],
        }),
      ),
    );
  });

  it("saves reviewed fields before confirming", async () => {
    vi.mocked(profileApi.getProfile).mockResolvedValue(profile);
    vi.mocked(profileApi.patchProfile).mockResolvedValue(profile);
    vi.mocked(profileApi.confirmProfile).mockResolvedValue({ ...profile, confirmed: true });

    render(wrap(<ProfilePanel locale="en" />));
    fireEvent.click(await screen.findByRole("button", { name: "Confirm profile" }));

    await waitFor(() => expect(profileApi.confirmProfile).toHaveBeenCalledOnce());
    expect(profileApi.patchProfile).toHaveBeenCalledOnce();
    expect(vi.mocked(profileApi.patchProfile).mock.invocationCallOrder[0]).toBeLessThan(
      vi.mocked(profileApi.confirmProfile).mock.invocationCallOrder[0]!,
    );
  });
});
