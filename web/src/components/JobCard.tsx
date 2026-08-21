import { useState } from "react";
import { t, type Locale, type TranslationKey } from "../i18n";
import { hasStatedSalary, type JobCard as Job } from "../api/feed";
import {
  openFillDeps,
  requestOpenAndFill,
  type OpenFillResult,
  type OpenFillStatus,
} from "../api/openfill";

/**
 * JobCard renders a single feed listing. Salary is shown only when the employer
 * stated it; otherwise a neutral "not disclosed" label is shown — never an
 * estimate (locked decision).
 *
 * "Open & Fill" arms the browser extension to autofill the posting and opens it
 * in a new tab. The user reviews every field and clicks Apply themselves — the
 * system never submits (Prime Directive). The plain apply link is always
 * available as a manual fallback.
 */
export function JobCard({
  job,
  locale,
  canFill = false,
  fillReason,
  runOpenFill = (url) => requestOpenAndFill(url, openFillDeps),
}: {
  job: Job;
  locale: Locale;
  canFill?: boolean;
  fillReason?: "needLogin" | "needProfile";
  runOpenFill?: (url: string) => Promise<OpenFillResult>;
}) {
  const stated = hasStatedSalary(job.salary);
  const [notice, setNotice] = useState<OpenFillStatus | null>(null);
  const [busy, setBusy] = useState(false);

  const noticeKey: Record<OpenFillStatus, TranslationKey> = {
    armed: "feed.openFill.armed",
    noExtension: "feed.openFill.noExtension",
    needLogin: "feed.openFill.needLogin",
    needProfile: "feed.openFill.needProfile",
    error: "feed.error",
  };

  async function onOpenFill() {
    if (!canFill) {
      setNotice(fillReason ?? "needLogin");
      return;
    }
    setBusy(true);
    try {
      const result = await runOpenFill(job.source_url);
      setNotice(result.status);
    } catch {
      setNotice("error");
    } finally {
      setBusy(false);
    }
  }

  const noticeTone = notice === "armed"
    ? "border-brand-success bg-brand-success-soft text-brand-success"
    : notice === "error"
      ? "border-brand-danger bg-brand-danger-soft text-brand-danger"
      : "border-brand-warning bg-brand-warning-soft text-brand-warning";

  return (
    <article className="job-card flex h-full flex-col gap-4 rounded-2xl border border-brand-border bg-brand-surface p-5 shadow-lg transition hover:-translate-y-0.5 hover:border-brand-primary">
      <div className="job-card__head flex items-start justify-between gap-3">
        <h3 className="job-card__title m-0 text-lg font-bold leading-tight text-brand-text">{job.title}</h3>
        <span className="job-card__source shrink-0 rounded-full bg-brand-surface-2 px-2.5 py-1 text-[0.65rem] font-bold uppercase tracking-widest text-brand-muted">
          {job.source}
        </span>
      </div>
      <p className="job-card__company m-0 font-semibold text-brand-accent-light">{job.company}</p>
      <p className="job-card__meta m-0 flex flex-wrap items-center gap-2 text-sm text-brand-muted">
        <span>{job.location}</span>
        {job.remote && <span className="badge badge--remote rounded-full bg-brand-primary-soft px-2.5 py-1 text-xs font-semibold text-brand-accent-light">{t(locale, "feed.remote")}</span>}
        {job.employment_type && <span className="badge rounded-full bg-brand-surface-2 px-2.5 py-1 text-xs font-medium text-brand-text">{job.employment_type}</span>}
        {job.seniority && <span className="badge rounded-full bg-brand-surface-2 px-2.5 py-1 text-xs font-medium text-brand-text">{job.seniority}</span>}
        {job.years_experience != null && (
          <span className="badge rounded-full bg-brand-surface-2 px-2.5 py-1 text-xs font-medium text-brand-text">
            {job.years_experience} {t(locale, "feed.experience")}
          </span>
        )}
      </p>
      <p
        className={`job-card__salary m-0 text-base font-bold ${stated ? "text-brand-success" : "font-medium italic text-brand-muted"}`}
        data-stated={stated}
      >
        {stated ? job.salary.label : t(locale, "feed.salary.undisclosed")}
      </p>
      {job.requirements.length > 0 && (
        <ul className="job-card__reqs m-0 flex list-none flex-wrap gap-2 p-0" aria-label={t(locale, "feed.requirements")}>
          {job.requirements.slice(0, 6).map((req) => (
            <li key={req} className="chip rounded-lg bg-brand-surface-2 px-2.5 py-1 text-xs text-brand-text">
              {req}
            </li>
          ))}
        </ul>
      )}
      <div className="job-card__foot mt-auto flex flex-wrap items-center gap-2 pt-2">
        <button
          type="button"
          className="job-card__fill rounded-xl border border-brand-primary-strong bg-brand-primary-strong px-3.5 py-2 text-sm font-bold text-brand-on-primary transition hover:bg-brand-primary disabled:cursor-progress disabled:opacity-60"
          onClick={onOpenFill}
          disabled={busy}
        >
          {t(locale, "feed.openFill")}
        </button>
        <a
          className="job-card__apply rounded-xl border border-brand-primary px-3.5 py-2 text-sm font-bold text-brand-accent-light no-underline transition hover:bg-brand-primary-soft"
          href={job.source_url}
          target="_blank"
          rel="noreferrer noopener"
        >
          {t(locale, "feed.viewApply")}
        </a>
      </div>
      <p className="job-card__apply-note m-0 text-xs leading-5 text-brand-muted">{t(locale, "feed.openFill.note")}</p>
      {notice && (
        <p
          className={`job-card__fill-notice job-card__fill-notice--${notice} rounded-xl border px-3 py-2 text-sm ${noticeTone}`}
          role="status"
        >
          {t(locale, noticeKey[notice])}
        </p>
      )}
    </article>
  );
}
