import { useEffect, useRef, useState } from "react";
import { fetchFeed, type FeedQuery, type FeedResponse } from "../api/feed";
import { t, type Locale } from "../i18n";
import { JobCard } from "./JobCard";

type Status = "loading" | "ready" | "error";

const PAGE_SIZE = 24;

/**
 * JobFeed loads the public job feed from the backend and renders it with basic
 * search and filters. Queries are debounced so typing doesn't spam the API, and
 * in-flight requests are aborted when inputs change.
 */
export function JobFeed({ locale }: { locale: Locale }) {
  const [search, setSearch] = useState("");
  const [location, setLocation] = useState("");
  const [remoteOnly, setRemoteOnly] = useState(false);
  const [data, setData] = useState<FeedResponse | null>(null);
  const [status, setStatus] = useState<Status>("loading");
  const debounce = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  useEffect(() => {
    const controller = new AbortController();
    const query: FeedQuery = { limit: PAGE_SIZE };
    if (search.trim()) query.q = search;
    if (location.trim()) query.location = location;
    if (remoteOnly) query.remote = true;

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
  }, [search, location, remoteOnly]);

  return (
    <section id="feed" className="feed">
      <h2 className="feed__heading">{t(locale, "feed.heading")}</h2>

      <form className="feed__filters" onSubmit={(e) => e.preventDefault()}>
        <input
          className="feed__search"
          type="search"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t(locale, "feed.search.placeholder")}
          aria-label={t(locale, "feed.search.placeholder")}
        />
        <input
          className="feed__location"
          type="text"
          value={location}
          onChange={(e) => setLocation(e.target.value)}
          placeholder={t(locale, "feed.filter.location")}
          aria-label={t(locale, "feed.filter.location")}
        />
        <label className="feed__remote">
          <input
            type="checkbox"
            checked={remoteOnly}
            onChange={(e) => setRemoteOnly(e.target.checked)}
          />
          {t(locale, "feed.filter.remote")}
        </label>
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
                <JobCard key={`${job.source}:${job.source_url}`} job={job} locale={locale} />
              ))}
            </div>
          )}
        </>
      )}
    </section>
  );
}
