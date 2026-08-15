// Typed client for the public job feed (FEED-1..3). Pay is employer-stated only
// and clearly labeled — the client never derives or displays an estimate
// (locked decision). Job cards link out to the original posting; the user
// applies there themselves. The system never submits an application.

export interface Salary {
  stated_min: number | null;
  stated_max: number | null;
  currency: string;
  /** Preformatted stated-pay label (e.g. "Rp 8.000.000 - Rp 11.000.000"). */
  label: string;
  /** True only when the employer stated pay. Never set from an estimate. */
  stated: boolean;
}

export interface JobCard {
  source: string;
  source_url: string;
  title: string;
  company: string;
  location: string;
  remote: boolean;
  salary: Salary;
  seniority: string;
  employment_type: string;
  years_experience: number | null;
  requirements: string[];
  posted_at: string | null;
}

export interface FeedResponse {
  total: number;
  limit: number;
  offset: number;
  jobs: JobCard[];
}

export interface FeedQuery {
  q?: string;
  location?: string;
  employment_type?: string;
  remote?: boolean;
  pay_min?: number;
  pay_max?: number;
  skills?: string | string[];
  max_yoe?: number;
  posted_after?: string;
  source?: string;
  limit?: number;
  offset?: number;
}

/** Build the `/feed` request path with query params, omitting empty filters. */
export function buildFeedPath(query: FeedQuery = {}): string {
  const params = new URLSearchParams();
  if (query.q?.trim()) params.set("q", query.q.trim());
  if (query.location?.trim()) params.set("location", query.location.trim());
  if (query.employment_type?.trim())
    params.set("employment_type", query.employment_type.trim());
  if (query.remote != null) params.set("remote", String(query.remote));
  if (query.pay_min != null) params.set("pay_min", String(query.pay_min));
  if (query.pay_max != null) params.set("pay_max", String(query.pay_max));
  const skills = Array.isArray(query.skills) ? query.skills : query.skills?.split(",");
  const normalizedSkills = skills?.map((skill) => skill.trim()).filter(Boolean).join(",");
  if (normalizedSkills) params.set("skills", normalizedSkills);
  if (query.max_yoe != null) params.set("max_yoe", String(query.max_yoe));
  if (query.posted_after?.trim()) params.set("posted_after", query.posted_after.trim());
  if (query.source?.trim()) params.set("source", query.source.trim());
  if (query.limit != null) params.set("limit", String(query.limit));
  if (query.offset != null) params.set("offset", String(query.offset));
  const qs = params.toString();
  return qs ? `/feed?${qs}` : "/feed";
}

/** Fetch a page of the job feed from the backend. */
export async function fetchFeed(
  query: FeedQuery = {},
  signal?: AbortSignal,
): Promise<FeedResponse> {
  const res = await fetch(buildFeedPath(query), {
    signal,
    headers: { Accept: "application/json" },
  });
  if (!res.ok) {
    throw new Error(`feed request failed: ${res.status}`);
  }
  return (await res.json()) as FeedResponse;
}

/** True when the employer stated pay and a label is present to show. */
export function hasStatedSalary(salary: Salary): boolean {
  return salary.stated && salary.label.trim() !== "";
}
