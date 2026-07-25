// Typed client for the CV API (CV-1/2). CV bytes are stored encrypted at rest;
// parsing writes structured data into the profile as an assist, leaving it
// unconfirmed so the user reviews it (parsing is never authoritative).

import { apiFetch } from "./http";

export interface CVFile {
  id: string;
  filename: string;
  content_type: string;
  size_bytes: number;
  created_at: string;
}

export async function listCVs(signal?: AbortSignal): Promise<CVFile[]> {
  const res = await apiFetch<{ files: CVFile[] }>("/cv", { signal });
  return res.files ?? [];
}

/** Upload a CV file (PDF/DOCX). Returns the stored file metadata. */
export function uploadCV(file: File): Promise<CVFile> {
  const form = new FormData();
  form.append("file", file);
  return apiFetch<CVFile>("/cv", { method: "POST", rawBody: form });
}

/** Parse a previously-uploaded CV into the user's profile. */
export function parseCV(id: string): Promise<unknown> {
  return apiFetch<unknown>(`/cv/${encodeURIComponent(id)}/parse`, { method: "POST" });
}
