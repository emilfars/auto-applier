/**
 * Content-script orchestration for Open & Fill (APP-5). On a supported source
 * page it asks the background whether this page was armed via Open & Fill; only
 * then does it load the confirmed profile and run the fill. It never submits.
 */
import type { ProfileData } from "@auto-applier/fill-mappings";
import { runFill, type RunFillResult } from "./fill.js";
import type { IsArmedMessage, IsArmedResult } from "./messaging.js";
import type { ApplyOptions } from "./apply.js";
import type { FillSnapshot } from "./arming.js";

/** Sends a message to the background and resolves its reply. */
export type SendMessage = (msg: IsArmedMessage) => Promise<IsArmedResult>;

/** Loads the user's confirmed profile (+ optional CV file) on demand. */
export type LoadProfile = () => Promise<{
  profile: ProfileData;
  file?: File | null;
}>;

export type ContentFillResult =
  | { ran: false; reason: "not-armed" | "profile-unconfirmed" }
  | { ran: true; report: RunFillResult };

/**
 * Run the Open & Fill flow for the current page. Returns without filling if the
 * page was not armed, or if the profile is not confirmed (defense in depth — the
 * engine also enforces the confirm gate, AC-CV-5).
 */
export async function runContentFill(
  send: SendMessage,
  doc: Document,
  loc: Pick<Location, "hostname" | "href">,
  loadProfile: LoadProfile,
  options: Pick<ApplyOptions, "onCorrection"> = {},
): Promise<ContentFillResult> {
  const armed = await send({ type: "isArmed", url: loc.href });
  if (!armed.armed) return { ran: false, reason: "not-armed" };

  const { profile, file } = armed.snapshot
    ? snapshotProfile(armed.snapshot)
    : await loadProfile();
  if (profile.confirmed !== true) {
    return { ran: false, reason: "profile-unconfirmed" };
  }

  const opts: ApplyOptions = file ? { file } : {};
  if (options.onCorrection) opts.onCorrection = options.onCorrection;
  const report = runFill(doc, loc, profile, opts);
  return { ran: true, report };
}

function snapshotProfile(snapshot: FillSnapshot): {
  profile: ProfileData;
  file?: File | null;
} {
  if (!snapshot.cv) return { profile: snapshot.profile };
  const binary = atob(snapshot.cv.bytesBase64);
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
  return {
    profile: { ...snapshot.profile, cv_file: snapshot.cv.name },
    file: new File([bytes], snapshot.cv.name, { type: snapshot.cv.type }),
  };
}

/** Render a small in-page review summary without requiring a popup UI. */
export function renderReviewSummary(doc: Document, report: RunFillResult): void {
  const existing = doc.querySelector("[data-aa-review-summary]");
  existing?.remove();
  const summary = doc.createElement("aside");
  summary.setAttribute("data-aa-review-summary", "true");
  summary.dataset.applied = String(report.applied);
  summary.dataset.uncertain = String(report.uncertain);
  summary.dataset.empty = String(report.fields.filter((f) => f.state === "empty").length);
  summary.dataset.coverage = String(report.coverage);
  summary.dataset.submitted = String(report.submitted);
  summary.dataset.appliedKeys = report.fields
    .filter((field) => field.applied)
    .map((field) => field.key)
    .join(",");
  summary.dataset.uncertainKeys = report.fields
    .filter((field) => field.state === "uncertain")
    .map((field) => field.key)
    .join(",");
  summary.dataset.emptyKeys = report.fields
    .filter((field) => field.state === "empty")
    .map((field) => field.key)
    .join(",");
  summary.setAttribute("role", "status");
  summary.textContent =
    `Auto Applier review: ${report.applied} filled, ` +
    `${report.uncertain} uncertain, ${report.fields.filter((f) => f.state === "empty").length} empty.`;
  summary.style.cssText =
    "position:fixed;z-index:2147483647;right:12px;bottom:12px;" +
    "max-width:360px;padding:10px 12px;background:#111827;color:#fff;" +
    "font:13px system-ui,sans-serif;border-radius:6px;box-shadow:0 2px 8px #0005;";
  (doc.body ?? doc.documentElement).append(summary);
}
