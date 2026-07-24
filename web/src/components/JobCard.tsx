import { t, type Locale } from "../i18n";
import { hasStatedSalary, type JobCard as Job } from "../api/feed";

/**
 * JobCard renders a single feed listing. Salary is shown only when the employer
 * stated it; otherwise a neutral "not disclosed" label is shown — never an
 * estimate (locked decision). The apply link opens the original posting in a
 * new tab; the user reviews and submits it themselves. The system never submits.
 */
export function JobCard({ job, locale }: { job: Job; locale: Locale }) {
  const stated = hasStatedSalary(job.salary);
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
        <a
          className="job-card__apply"
          href={job.source_url}
          target="_blank"
          rel="noreferrer noopener"
        >
          {t(locale, "feed.viewApply")}
        </a>
        <span className="job-card__apply-note">{t(locale, "feed.applyNote")}</span>
      </div>
    </article>
  );
}
