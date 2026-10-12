/**
 * Content-script orchestration for Open & Fill (APP-5). On a supported source
 * page it asks the background whether this page was armed via Open & Fill; only
 * then does it load the confirmed profile and run the fill. It never submits.
 */
import type { ProfileData } from "@auto-applier/fill-mappings";
import { runFill, type RunFillResult } from "./fill.js";
import type { IsArmedMessage, IsArmedResult } from "./messaging.js";
import { REVIEW_COLORS, type ApplyOptions } from "./apply.js";
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

const REVIEW_COPY = {
  id: {
    title: "Tinjauan Auto Applier",
    filled: "terisi",
    uncertain: "perlu dicek",
    empty: "kosong",
    note: "Warna garis tiap kolom mengikuti daftar ini. Tinjau, lalu klik Lamar sendiri. Auto Applier tidak pernah mengirim lamaran.",
    close: "Tutup tinjauan",
  },
  en: {
    title: "Auto Applier review",
    filled: "filled",
    uncertain: "to check",
    empty: "empty",
    note: "Field outlines match this list. Review, then click Apply yourself. Auto Applier never sends an application.",
    close: "Close review",
  },
} as const;

/**
 * Render the in-page review panel: a legend for the field outlines with counts.
 * Indonesian unless the browser language is English, like the web app's default.
 */
export function renderReviewSummary(
  doc: Document,
  report: RunFillResult,
  language: string = globalThis.navigator?.language ?? "id",
): void {
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
  const copy = REVIEW_COPY[language.toLowerCase().startsWith("en") ? "en" : "id"];
  const empty = report.fields.filter((f) => f.state === "empty").length;
  summary.style.cssText =
    "position:fixed;z-index:2147483647;right:12px;bottom:12px;box-sizing:border-box;" +
    "width:min(340px,calc(100vw - 24px));padding:12px 14px;background:#13234a;color:#fff;" +
    "border:1px solid #6d7c9f;border-radius:8px;" +
    "font:14px/1.45 system-ui,sans-serif;";

  const header = doc.createElement("div");
  header.style.cssText = "display:flex;justify-content:space-between;align-items:center;gap:8px;";
  const title = doc.createElement("strong");
  title.textContent = copy.title;
  const close = doc.createElement("button");
  close.type = "button";
  close.textContent = "\u00d7";
  close.setAttribute("aria-label", copy.close);
  close.style.cssText =
    "min-width:44px;min-height:44px;margin:-8px -10px -8px 0;border:0;background:transparent;" +
    "color:#fff;font-size:22px;line-height:1;cursor:pointer;";
  close.addEventListener("click", () => summary.remove());
  header.append(title, close);

  const list = doc.createElement("ul");
  list.style.cssText = "list-style:none;margin:6px 0 8px;padding:0;";
  const rows: Array<[keyof typeof REVIEW_COLORS, number]> = [
    ["filled", report.applied],
    ["uncertain", report.uncertain],
    ["empty", empty],
  ];
  for (const [state, count] of rows) {
    const item = doc.createElement("li");
    item.style.cssText = "display:flex;align-items:baseline;gap:8px;";
    const marker = doc.createElement("span");
    marker.setAttribute("aria-hidden", "true");
    marker.style.cssText = `flex:none;width:10px;height:10px;border-radius:2px;outline:1px solid #fff;background:${REVIEW_COLORS[state]};`;
    const label = doc.createElement("span");
    label.textContent = `${count} ${copy[state]}`;
    item.append(marker, label);
    list.append(item);
  }

  const note = doc.createElement("p");
  note.textContent = copy.note;
  note.style.cssText = "margin:0;color:#a9b8d8;";
  summary.append(header, list, note);
  (doc.body ?? doc.documentElement).append(summary);
}
