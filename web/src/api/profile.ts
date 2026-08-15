// Typed client for the profile API (CV-3/4/5). The profile backs application
// autofill, so the user must review and confirm it before the fill flow can be
// armed. Editing resets confirmation server-side (confirm-before-apply gate).

import { apiFetch } from "./http";

/** Employment types the backend accepts for employment_type. */
export const EMPLOYMENT_TYPES = [
  "full_time",
  "part_time",
  "contract",
  "internship",
  "freelance",
  "temporary",
] as const;

export interface Profile {
  full_name: string;
  email: string;
  phone: string;
  linkedin_url: string;
  github_url: string;
  portfolio_url: string;
  address: string;
  city: string;
  summary: string;
  current_employer: string;
  current_company?: string;
  current_title: string;
  highest_education: string;
  education: EducationEntry[];
  work_history: WorkEntry[];
  skills: string[];
  expected_salary: number | null;
  notice_period_days: number | null;
  work_authorization: string;
  open_to_relocation: boolean;
  preferred_locations: string[];
  employment_type: string;
  confirmed: boolean;
  confirmed_at: string | null;
}

export interface EducationEntry {
  institution: string;
  degree: string;
  field: string;
  start_year: string;
  end_year: string;
}

export interface WorkEntry {
  company: string;
  title: string;
  start_date: string;
  end_date: string;
}

/** Fields the UI can patch. Arrays are sent as JSON arrays; omit to leave unchanged. */
export interface ProfilePatch {
  full_name?: string;
  email?: string;
  phone?: string;
  linkedin_url?: string;
  github_url?: string;
  portfolio_url?: string;
  address?: string;
  city?: string;
  summary?: string;
  current_employer?: string;
  current_company?: string;
  current_title?: string;
  highest_education?: string;
  education?: EducationEntry[];
  work_history?: WorkEntry[];
  expected_salary?: number | null;
  notice_period_days?: number | null;
  work_authorization?: string;
  open_to_relocation?: boolean;
  employment_type?: string;
  skills?: string[];
  preferred_locations?: string[];
}

export function getProfile(signal?: AbortSignal): Promise<Profile> {
  return apiFetch<Profile>("/profile", { signal });
}

export function patchProfile(patch: ProfilePatch): Promise<Profile> {
  return apiFetch<Profile>("/profile", { method: "PATCH", body: patch });
}

/** Confirm the profile — required before the fill flow may be armed. */
export function confirmProfile(): Promise<Profile> {
  return apiFetch<Profile>("/profile/confirm", { method: "POST" });
}

export interface ArmStatus {
  can_arm: boolean;
  reason?: string;
}

/** Check whether the fill flow may be armed (profile confirmed). */
export function checkArm(signal?: AbortSignal): Promise<ArmStatus> {
  return apiFetch<ArmStatus>("/profile/arm", { signal });
}

export interface FillSnapshot {
  profile: Record<string, unknown> & { confirmed: boolean };
  cv: {
    id: string;
    filename: string;
    content_type: string;
    bytes_base64: string;
  } | null;
}

/** Fetch the confirmed profile and latest owned CV for one Open & Fill arm. */
export function getFillSnapshot(cvID?: string): Promise<FillSnapshot> {
  const query = cvID ? `?cv_id=${encodeURIComponent(cvID)}` : "";
  return apiFetch<FillSnapshot>(`/profile/fill${query}`);
}
