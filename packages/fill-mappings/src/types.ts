/**
 * Canonical, framework-agnostic types for the shared fill layer.
 *
 * PRIME DIRECTIVE: nothing in this package may submit an application. The types
 * describe *what* to fill and *how confident* we are; the actual DOM writes live
 * in the consuming surface (extension / WebView) and must never trigger submit.
 */

/** Per-field outcome state. `uncertain` must never be reported as `filled`. */
export type FieldState = "filled" | "uncertain" | "empty";

/** Canonical profile keys a portal map can target. */
export type ProfileKey =
  | "full_name"
  | "first_name"
  | "last_name"
  | "email"
  | "phone"
  | "city"
  | "address"
  | "linkedin_url"
  | "portfolio_url"
  | "github_url"
  | "expected_salary"
  | "notice_period"
  | "years_experience"
  | "current_company"
  | "current_title"
  | "highest_education"
  | "university"
  | "major"
  | "graduation_year"
  | "summary"
  | "cv_file";

/**
 * User profile snapshot passed to the engine. Only confirmed profiles should be
 * used to arm a fill (AC-CV-5); the engine treats a missing key as no data.
 */
export type ProfileData = Partial<Record<ProfileKey, string>> & {
  /** Present only when the user has confirmed their parsed CV (AC-CV-5). */
  confirmed?: boolean;
};

/** How a mapping locates and scores a form field. */
export interface FieldMapping {
  /** Canonical profile key this field is filled from. */
  key: ProfileKey;
  /**
   * CSS selectors tried in order. Earlier selectors are higher-confidence
   * (portal-specific ids/names); later ones are looser fallbacks.
   */
  selectors: string[];
  /**
   * Confidence when matched by the FIRST selector. Fallback selectors decay
   * this (see engine). Below `uncertainBelow` the fill is flagged `uncertain`
   * and must be user-reviewed, never silently committed.
   */
  confidence: number;
  /** Optional value transform (e.g. salary formatting) applied before fill. */
  transform?: (value: string) => string;
  /** Marks a file-attach target (CV upload) rather than a text field. */
  file?: boolean;
}

/** A versioned per-portal field map. A DOM/selector change bumps `version`. */
export interface PortalMap {
  /** Stable portal id, e.g. "glints". */
  id: string;
  /** Semver-ish version; bump on any selector/field change. */
  version: string;
  /** Hostnames this map applies to (suffix match, e.g. "glints.com"). */
  hosts: string[];
  /** Field mappings for this portal. */
  fields: FieldMapping[];
}

/** Resolved intent for a single field: what to write and how sure we are. */
export interface FieldOutcome {
  key: ProfileKey;
  state: FieldState;
  /** Confidence in [0,1]; 0 when empty. */
  confidence: number;
  /** The value to write, when state is `filled` or `uncertain`. */
  value?: string;
  /** CSS selector that matched, for the extension to locate the element. */
  selector?: string;
  /** True for file-attach targets (CV upload). */
  file?: boolean;
  /** Human-readable reason when empty/uncertain (for the review UI). */
  note?: string;
}

/** The full plan for a page: per-field outcomes plus coverage metrics. */
export interface FillPlan {
  portalId: string;
  version: string;
  outcomes: FieldOutcome[];
  /** Count of fields resolved to `filled`. */
  filled: number;
  /** Count flagged `uncertain` (needs review). */
  uncertain: number;
  /** Count with no data or no matching element. */
  empty: number;
  /** filled / (mappable fields). */
  coverage: number;
}
