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

  return (
    <article className="job-card">
      <div className="job-card__head">
        <h3 className="job-card__title">{job.title}</h3>
        <span className="job-card__source">{job.source}</span>
      </div>
      <p className="job-card__company">{job.company}</p>
      <p className="job-card__meta">
        <span>{job.location}</span>
        {job.remote && <span className="badge badge--remote">{t(locale, "feed.remote")}</span>}
        {job.employment_type && <span className="badge">{job.employment_type}</span>}
        {job.seniority && <span className="badge">{job.seniority}</span>}
        {job.years_experience != null && (
          <span className="badge">
            {job.years_experience} {t(locale, "feed.experience")}
          </span>
        )}
      </p>
      <p className="job-card__salary" data-stated={stated}>
        {stated ? job.salary.label : t(locale, "feed.salary.undisclosed")}
      </p>
      {job.requirements.length > 0 && (
        <ul className="job-card__reqs" aria-label={t(locale, "feed.requirements")}>
          {job.requirements.slice(0, 6).map((req) => (
            <li key={req} className="chip">
              {req}
            </li>
          ))}
        </ul>
      )}
      <div className="job-card__foot">
        <button
          type="button"
          className="job-card__fill"
          onClick={onOpenFill}
          disabled={busy}
        >
          {t(locale, "feed.openFill")}
        </button>
        <a
          className="job-card__apply"
          href={job.source_url}
          target="_blank"
          rel="noreferrer noopener"
        >
          {t(locale, "feed.viewApply")}
        </a>
      </div>
      <p className="job-card__apply-note">{t(locale, "feed.openFill.note")}</p>
      {notice && (
        <p
          className={`job-card__fill-notice job-card__fill-notice--${notice}`}
          role="status"
        >
          {t(locale, noticeKey[notice])}
        </p>
      )}
    </article>
  );
}
