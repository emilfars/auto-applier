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
    <section id="feed" className="feed">
      <h2 className="feed__heading">{t(locale, "feed.heading")}</h2>

      <form
        className="feed__filters"
        onSubmit={(e) => {
          e.preventDefault();
          if (e.currentTarget.checkValidity() && validNumberFilters(payMin, payMax, maxYoE)) {
            setOffset(0);
          }
        }}
      >
        <input
          className="feed__search"
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
        <label className="feed__field">
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
        <label className="feed__field">
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
        <label className="feed__field">
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
        <label className="feed__field">
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
        <label className="feed__field">
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
        <label className="feed__field">
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
        <label className="feed__field">
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
        <label className="feed__remote">
          <input
            type="checkbox"
            name="remote"
            checked={remoteOnly}
            onChange={(e) => onFilter(() => setRemoteOnly(e.target.checked))}
          />
          {t(locale, "feed.filter.remote")}
        </label>
        <button type="submit" className="btn btn--primary">
          {t(locale, "feed.filter.apply")}
        </button>
        <button type="button" className="btn btn--ghost" onClick={clearFilters}>
          {t(locale, "feed.filter.clear")}
        </button>
      </form>

      {status === "loading" && <p className="feed__status">{t(locale, "feed.loading")}</p>}
      {status === "error" && (
        <p className="feed__status feed__status--error" role="alert">
          {t(locale, "feed.error")}
        </p>
      )}
      {status === "ready" && data && (
        <>
          <p className="feed__count">
            {data.total} {t(locale, "feed.results.count")}
          </p>
          {data.jobs.length === 0 ? (
            <p className="feed__status">{t(locale, "feed.empty")}</p>
          ) : (
            <div className="feed__list">
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
            <nav className="feed__pager" aria-label={t(locale, "feed.page.status", { page, pages })}>
              <button
                type="button"
                className="feed__pager-btn"
                onClick={() => setOffset((o) => Math.max(0, o - PAGE_SIZE))}
                disabled={!hasPrev}
              >
                {t(locale, "feed.page.prev")}
              </button>
              <span className="feed__pager-status">
                {t(locale, "feed.page.status", { page, pages })}
              </span>
              <button
                type="button"
                className="feed__pager-btn"
                onClick={() => setOffset((o) => o + PAGE_SIZE)}
                disabled={!hasNext}
              >
                {t(locale, "feed.page.next")}
              </button>
            </nav>
          )}
        </>
      )}
    </section>
  );
}
