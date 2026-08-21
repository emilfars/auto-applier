import { apiFetch } from "./http";
import type { FeedQuery } from "./feed";

export interface JobState {
  job_key: string;
  dismissed: boolean;
  already_applied: boolean;
}

export interface SavedFilter {
  id: string;
  name: string;
  query: FeedQuery;
  created_at: string;
  last_alerted_at: string | null;
}

export interface Application {
  id: string;
  jobKey?: string;
  job_key?: string;
  jobTitle?: string;
  job_title?: string;
  company: string;
  status: "form_filled" | "submitted" | "viewed" | "rejected" | "interview";
  createdAt?: string;
  created_at?: string;
  updatedAt?: string;
  updated_at?: string;
}

export interface Snippet {
  id: string;
  name: string;
  body: string;
  created_at: string;
  updated_at: string;
}

export interface Completeness {
  score: number;
  complete: boolean;
  missing: string[];
}

export function setJobState(jobKey: string, action: "dismiss" | "undismiss" | "applied") {
  const path = `/jobs/${encodeURIComponent(jobKey)}/${action === "undismiss" ? "dismiss" : action}`;
  return apiFetch<JobState>(path, { method: action === "undismiss" ? "DELETE" : "POST" });
}

export function recordApplication(jobKey: string) {
  return apiFetch<Application>("/applications", { method: "POST", body: { job_id: jobKey } });
}

export async function listApplications() {
  const response = await apiFetch<{ applications: Application[] }>("/applications");
  return response.applications;
}

export function updateApplication(id: string, status: Application["status"]) {
  return apiFetch<Application>(`/applications/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: { status },
  });
}

export function confirmSubmitted(id: string) {
  return apiFetch<Application>(`/applications/${encodeURIComponent(id)}/confirm`, { method: "POST" });
}

export async function listSavedFilters() {
  const response = await apiFetch<{ filters: SavedFilter[] }>("/saved-filters");
  return response.filters;
}

export function saveFilter(name: string, query: FeedQuery) {
  return apiFetch<SavedFilter>("/saved-filters", { method: "POST", body: { name, query } });
}

export function deleteSavedFilter(id: string) {
  return apiFetch<void>(`/saved-filters/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export async function listSnippets() {
  const response = await apiFetch<{ snippets: Snippet[] }>("/snippets");
  return response.snippets;
}

export function createSnippet(name: string, body: string) {
  return apiFetch<Snippet>("/snippets", { method: "POST", body: { name, body } });
}

export function deleteSnippet(id: string) {
  return apiFetch<void>(`/snippets/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export function getCompleteness(signal?: AbortSignal) {
  return apiFetch<Completeness>("/profile/completeness", { signal });
}
