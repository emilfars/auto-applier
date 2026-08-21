import { useEffect, useRef, useState } from "react";
import { fetchFeed, type FeedQuery, type FeedResponse } from "../api/feed";
import { EMPLOYMENT_TYPES } from "../api/profile";
import { t, type Locale } from "../i18n";
import { JobCard } from "./JobCard";

type Status = "loading" | "ready" | "error";

const PAGE_SIZE = 24;
const SOURCE_IDS = ["kalibrr", "jooble", "glints", "jobstreet", "kemnaker", "company-career"];

function parseNumberFilter(value: string): number | undefined {
  if (!value.trim()) return undefined;
  const parsed = Number(value);
  return Number.isSafeInteger(parsed) && parsed >= 0 ? parsed : undefined;
}

function validNumberFilters(payMin: string, payMax: string, maxYoE: string): boolean {
  const min = parseNumberFilter(payMin);
  const max = parseNumberFilter(payMax);
  return (
    (!payMin.trim() || min != null) &&
    (!payMax.trim() || max != null) &&
    (!maxYoE.trim() || parseNumberFilter(maxYoE) != null) &&
    (min == null || max == null || min <= max)
  );
}

function dateFilterToRFC3339(value: string): string | undefined {
  return /^\d{4}-\d{2}-\d{2}$/.test(value) ? `${value}T00:00:00Z` : undefined;
}

/**
 * JobFeed loads the public job feed from the backend and renders it with basic
 * search and filters. Queries are debounced so typing doesn't spam the API, and
 * in-flight requests are aborted when inputs change.
 */
export function JobFeed({
  locale,
  canFill = false,
  fillReason,
}: {
  locale: Locale;
  canFill?: boolean;
  fillReason?: "needLogin" | "needProfile";
}) {
  const [search, setSearch] = useState("");
  const [location, setLocation] = useState("");
  const [remoteOnly, setRemoteOnly] = useState(false);
  const [payMin, setPayMin] = useState("");
  const [payMax, setPayMax] = useState("");
  const [skills, setSkills] = useState("");
  const [maxYoE, setMaxYoE] = useState("");
  const [postedAfter, setPostedAfter] = useState("");
  const [source, setSource] = useState("");
  const [employmentType, setEmploymentType] = useState("");
  const [offset, setOffset] = useState(0);
  const [data, setData] = useState<FeedResponse | null>(null);
  const [status, setStatus] = useState<Status>("loading");
  const debounce = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const payMinValue = parseNumberFilter(payMin);
  const payMaxValue = parseNumberFilter(payMax);

  // Changing any filter resets pagination to the first page.
  const onFilter = (fn: () => void) => {
    fn();
    setOffset(0);
  };

  useEffect(() => {
    const controller = new AbortController();
    if (!validNumberFilters(payMin, payMax, maxYoE)) return;

    const maxYoEValue = parseNumberFilter(maxYoE);
    const postedAfterValue = dateFilterToRFC3339(postedAfter);
    if (postedAfter && !postedAfterValue) return;
    const query: FeedQuery = { limit: PAGE_SIZE, offset };
    if (search.trim()) query.q = search;
    if (location.trim()) query.location = location;
    if (remoteOnly) query.remote = true;
    if (payMinValue != null) query.pay_min = payMinValue;
    if (payMaxValue != null) query.pay_max = payMaxValue;
    if (skills.trim()) query.skills = skills;
    if (maxYoEValue != null) query.max_yoe = maxYoEValue;
    if (postedAfterValue) query.posted_after = postedAfterValue;
    if (source) query.source = source;
    if (employmentType) query.employment_type = employmentType;

    setStatus("loading");
    clearTimeout(debounce.current);
    debounce.current = setTimeout(() => {
      fetchFeed(query, controller.signal)
        .then((res) => {
          setData(res);
          setStatus("ready");
        })
        .catch((err: unknown) => {
          if (err instanceof DOMException && err.name === "AbortError") return;
          setStatus("error");
        });
    }, 250);

    return () => {
      controller.abort();
      clearTimeout(debounce.current);
    };
  }, [
    search,
    location,
    remoteOnly,
    payMin,
    payMax,
    skills,
    maxYoE,
    postedAfter,
    source,
    employmentType,
    offset,
  ]);

  const clearFilters = () => {
    setSearch("");
    setLocation("");
    setRemoteOnly(false);
    setPayMin("");
    setPayMax("");
    setSkills("");
    setMaxYoE("");
    setPostedAfter("");
    setSource("");
    setEmploymentType("");
    setOffset(0);
  };

  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const page = Math.floor(offset / PAGE_SIZE) + 1;
  const hasPrev = offset > 0;
  const hasNext = offset + PAGE_SIZE < total;

  return (
    <section id="feed" className="feed mx-auto max-w-app scroll-mt-28">
      <div className="mb-5 flex flex-wrap items-end justify-between gap-3">
        <div>
          <p className="mb-2 text-xs font-bold uppercase tracking-[0.2em] text-brand-accent-light">Auto Applier</p>
          <h2 className="feed__heading m-0 text-2xl font-black tracking-tight text-brand-text sm:text-3xl">
            {t(locale, "feed.heading")}
          </h2>
        </div>
      </div>

      <div className="feed__layout grid gap-6 lg:grid-cols-[minmax(15rem,18rem)_minmax(0,1fr)] lg:items-start">
        <aside className="rounded-2xl border border-brand-border bg-brand-surface p-4 shadow-lg lg:sticky lg:top-28">
          <form
            className="feed__filters grid gap-3"
            onSubmit={(e) => {
              e.preventDefault();
              if (e.currentTarget.checkValidity() && validNumberFilters(payMin, payMax, maxYoE)) {
                setOffset(0);
              }
            }}
          >
        <input
          className="feed__search rounded-xl border-brand-primary bg-brand-bg"
          type="search"
          name="q"
          value={search}
          onChange={(e) => onFilter(() => setSearch(e.target.value))}
          placeholder={t(locale, "feed.search.placeholder")}
          aria-label={t(locale, "feed.search.placeholder")}
        />
        <input
          className="feed__location"
          type="text"
          name="location"
          value={location}
          onChange={(e) => onFilter(() => setLocation(e.target.value))}
          placeholder={t(locale, "feed.filter.location")}
          aria-label={t(locale, "feed.filter.location")}
        />
        <label className="feed__field grid gap-1.5 text-xs font-semibold text-brand-muted">
          <span>{t(locale, "feed.filter.payMin")}</span>
          <input
             className="feed__location"
            type="number"
            name="pay_min"
            min="0"
            max={payMaxValue ?? undefined}
            step="1"
            value={payMin}
            onChange={(e) => onFilter(() => setPayMin(e.target.value))}
            aria-label={t(locale, "feed.filter.payMin")}
          />
        </label>
        <label className="feed__field grid gap-1.5 text-xs font-semibold text-brand-muted">
          <span>{t(locale, "feed.filter.payMax")}</span>
          <input
             className="feed__location"
            type="number"
            name="pay_max"
            min={payMinValue ?? 0}
            step="1"
            value={payMax}
            onChange={(e) => onFilter(() => setPayMax(e.target.value))}
            aria-label={t(locale, "feed.filter.payMax")}
          />
        </label>
        <label className="feed__field grid gap-1.5 text-xs font-semibold text-brand-muted">
          <span>{t(locale, "feed.filter.skills")}</span>
          <input
            className="feed__location"
            type="text"
            name="skills"
            value={skills}
            onChange={(e) => onFilter(() => setSkills(e.target.value))}
            aria-label={t(locale, "feed.filter.skills")}
          />
        </label>
        <label className="feed__field grid gap-1.5 text-xs font-semibold text-brand-muted">
          <span>{t(locale, "feed.filter.maxYoe")}</span>
          <input
            className="feed__location"
            type="number"
            name="max_yoe"
            min="0"
            step="1"
            value={maxYoE}
            onChange={(e) => onFilter(() => setMaxYoE(e.target.value))}
            aria-label={t(locale, "feed.filter.maxYoe")}
          />
        </label>
        <label className="feed__field grid gap-1.5 text-xs font-semibold text-brand-muted">
          <span>{t(locale, "feed.filter.postedAfter")}</span>
          <input
            className="feed__location"
            type="date"
            name="posted_after"
            value={postedAfter}
            onChange={(e) => onFilter(() => setPostedAfter(e.target.value))}
            aria-label={t(locale, "feed.filter.postedAfter")}
          />
        </label>
        <label className="feed__field grid gap-1.5 text-xs font-semibold text-brand-muted">
          <span>{t(locale, "feed.filter.source")}</span>
          <select
            className="feed__location"
            name="source"
            value={source}
            onChange={(e) => onFilter(() => setSource(e.target.value))}
            aria-label={t(locale, "feed.filter.source")}
          >
            <option value="">{t(locale, "feed.filter.all")}</option>
            {SOURCE_IDS.map((sourceID) => (
              <option key={sourceID} value={sourceID}>
                {sourceID}
              </option>
            ))}
          </select>
        </label>
        <label className="feed__field grid gap-1.5 text-xs font-semibold text-brand-muted">
          <span>{t(locale, "feed.filter.employmentType")}</span>
          <select
            className="feed__location"
            name="employment_type"
            value={employmentType}
            onChange={(e) => onFilter(() => setEmploymentType(e.target.value))}
            aria-label={t(locale, "feed.filter.employmentType")}
          >
            <option value="">{t(locale, "feed.filter.all")}</option>
            {EMPLOYMENT_TYPES.map((type) => (
              <option key={type} value={type}>
                {type.replace("_", " ")}
              </option>
            ))}
          </select>
        </label>
        <label className="feed__remote flex items-center gap-2 py-1 text-sm text-brand-muted">
          <input
            type="checkbox"
            name="remote"
            checked={remoteOnly}
            onChange={(e) => onFilter(() => setRemoteOnly(e.target.checked))}
          />
          {t(locale, "feed.filter.remote")}
        </label>
        <button
          type="submit"
          className="btn btn--primary rounded-xl border border-brand-primary-strong bg-brand-primary-strong px-4 py-2.5 font-semibold text-brand-on-primary transition hover:bg-brand-primary"
        >
          {t(locale, "feed.filter.apply")}
        </button>
        <button
          type="button"
          className="btn btn--ghost rounded-xl border border-brand-border bg-transparent px-4 py-2.5 font-semibold text-brand-text transition hover:border-brand-primary"
          onClick={clearFilters}
        >
          {t(locale, "feed.filter.clear")}
        </button>
          </form>
        </aside>

        <div className="min-w-0">
      {status === "loading" && (
        <div className="feed__loading" aria-busy="true" aria-live="polite">
          <p className="feed__status m-0 pb-4 text-sm text-brand-muted">{t(locale, "feed.loading")}</p>
          <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
            {Array.from({ length: 6 }, (_, index) => (
              <div key={index} className="animate-pulse rounded-2xl border border-brand-border bg-brand-surface p-5">
                <div className="mb-4 h-5 w-3/4 rounded bg-brand-surface-2" />
                <div className="mb-3 h-4 w-1/2 rounded bg-brand-surface-2" />
                <div className="mb-6 h-4 w-2/3 rounded bg-brand-surface-2" />
                <div className="h-9 rounded-xl bg-brand-surface-2" />
              </div>
            ))}
          </div>
        </div>
      )}
      {status === "error" && (
        <p className="feed__status feed__status--error rounded-xl border border-brand-danger bg-brand-danger-soft px-4 py-3 text-sm text-brand-danger" role="alert">
          {t(locale, "feed.error")}
        </p>
      )}
      {status === "ready" && data && (
        <>
          <p className="feed__count mb-4 text-sm text-brand-muted">
            {data.total} {t(locale, "feed.results.count")}
          </p>
          {data.jobs.length === 0 ? (
            <p className="feed__status rounded-2xl border border-dashed border-brand-border bg-brand-surface px-5 py-8 text-center text-sm text-brand-muted">
              {t(locale, "feed.empty")}
            </p>
          ) : (
            <div className="feed__list grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
              {data.jobs.map((job) => (
                <JobCard
                  key={`${job.source}:${job.source_url}`}
                  job={job}
                  locale={locale}
                  canFill={canFill}
                  fillReason={fillReason}
                />
              ))}
            </div>
          )}
          {pages > 1 && (
            <nav className="feed__pager mt-8 flex items-center justify-center gap-3" aria-label={t(locale, "feed.page.status", { page, pages })}>
              <button
                type="button"
                className="feed__pager-btn rounded-xl border border-brand-border bg-brand-surface-2 px-3 py-2 text-sm font-semibold text-brand-text transition hover:border-brand-primary disabled:cursor-not-allowed disabled:opacity-40"
                onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))}
                disabled={!hasPrev}
              >
                {t(locale, "feed.page.prev")}
              </button>
              <span className="feed__pager-status text-sm font-semibold text-brand-muted">
                {t(locale, "feed.page.status", { page, pages })}
              </span>
              <button
                type="button"
                className="feed__pager-btn rounded-xl border border-brand-border bg-brand-surface-2 px-3 py-2 text-sm font-semibold text-brand-text transition hover:border-brand-primary disabled:cursor-not-allowed disabled:opacity-40"
                onClick={() => setOffset((o) => o + PAGE_SIZE)}
                disabled={!hasNext}
              >
                {t(locale, "feed.page.next")}
              </button>
            </nav>
          )}
        </>
      )}
        </div>
      </div>
    </section>
  );
}
