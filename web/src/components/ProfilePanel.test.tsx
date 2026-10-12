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

    const institution = (await screen.findByLabelText("Institution")) as HTMLInputElement;
    const company = screen.getByLabelText("Company") as HTMLInputElement;
    expect(institution.value).toBe("UI");
    expect(company.value).toBe("Acme");

    fireEvent.change(screen.getByLabelText(/Email/), { target: { value: "new@example.com" } });
    fireEvent.change(screen.getByLabelText(/LinkedIn URL/), {
      target: { value: "https://linkedin.com/in/new" },
    });
    fireEvent.change(screen.getByLabelText(/Professional summary/), {
      target: { value: "New summary" },
    });
    fireEvent.change(institution, { target: { value: "ITB" } });
    fireEvent.change(company, { target: { value: "New Co" } });
    fireEvent.click(screen.getByRole("button", { name: "Add experience" }));
    expect(screen.getAllByLabelText("Company")).toHaveLength(2);
    fireEvent.click(screen.getByRole("button", { name: "Save changes" }));

    await waitFor(() =>
      expect(profileApi.patchProfile).toHaveBeenCalledWith(
        expect.objectContaining({
          email: "new@example.com",
          linkedin_url: "https://linkedin.com/in/new",
          summary: "New summary",
          education: [expect.objectContaining({ institution: "ITB", degree: "S.Kom" })],
          work_history: [expect.objectContaining({ company: "New Co", title: "Engineer" })],
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

  it("adds and removes education rows without any JSON", async () => {
    vi.mocked(profileApi.getProfile).mockResolvedValue({ ...profile, education: [] });
    vi.mocked(profileApi.patchProfile).mockResolvedValue(profile);
    render(wrap(<ProfilePanel locale="id-ID" />));

    expect(await screen.findByText(/Belum ada pendidikan\./)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Tambah pendidikan" }));
    fireEvent.click(screen.getByRole("button", { name: "Tambah pendidikan" }));
    fireEvent.change(screen.getAllByLabelText("Institusi")[0], { target: { value: "UGM" } });
    fireEvent.click(screen.getByRole("button", { name: "Hapus Pendidikan 2" }));
    expect(screen.getAllByLabelText("Institusi")).toHaveLength(1);

    fireEvent.click(screen.getByRole("button", { name: "Simpan perubahan" }));
    await waitFor(() =>
      expect(profileApi.patchProfile).toHaveBeenCalledWith(
        expect.objectContaining({ education: [expect.objectContaining({ institution: "UGM" })] }),
      ),
    );
  });
});
