// Typed client for the account data-rights API (AUTH-5 / UU PDP No. 27/2022).
// Users can export a complete copy of their data or erase their account and all
// PII. Both routes require an authenticated, verified session.

import { apiFetch } from "./http";

export interface AccountInfo {
  email: string;
  verified: boolean;
  created_at: string;
  consent_at: string | null;
}

export interface AccountCVFile {
  filename: string;
  content_type: string;
  size_bytes: number;
  created_at: string;
}

/** Complete export payload returned by GET /account/export. */
export interface AccountExport {
  account: AccountInfo;
  profile: Record<string, unknown> | null;
  cv_files: AccountCVFile[];
  additional?: unknown;
  exported_at: string;
}

/** Download a complete copy of the signed-in user's data. */
export function exportAccount(): Promise<AccountExport> {
  return apiFetch<AccountExport>("/account/export");
}

/** Permanently erase the signed-in user's account and all associated PII. */
export function deleteAccount(): Promise<void> {
  return apiFetch<void>("/account", { method: "DELETE" });
}
