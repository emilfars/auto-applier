import { useEffect, useState } from "react";
import { useSession } from "../auth/session";
import {
  confirmSubmitted,
  createSnippet,
  deleteSavedFilter,
  deleteSnippet,
  getCompleteness,
  listApplications,
  listSavedFilters,
  listSnippets,
  saveFilter,
  updateApplication,
  type Application,
  type Completeness,
  type SavedFilter,
  type Snippet,
} from "../api/m5";
import { ApiError } from "../api/http";
import { t, type Locale } from "../i18n";

export function EnhancementsPanel({ locale }: { locale: Locale }) {
  const { user } = useSession();
  const [completeness, setCompleteness] = useState<Completeness | null>(null);
  const [filters, setFilters] = useState<SavedFilter[]>([]);
  const [applications, setApplications] = useState<Application[]>([]);
  const [snippets, setSnippets] = useState<Snippet[]>([]);
  const [filterName, setFilterName] = useState("");
  const [filterSearch, setFilterSearch] = useState("");
  const [snippetName, setSnippetName] = useState("");
  const [snippetBody, setSnippetBody] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!user) return;
    Promise.all([getCompleteness(), listSavedFilters(), listApplications(), listSnippets()])
      .then(([score, saved, tracked, reusable]) => {
        setCompleteness(score);
        setFilters(saved);
        setApplications(tracked);
        setSnippets(reusable);
      })
      .catch(() => setError(t(locale, "auth.error")));
  }, [user, locale]);

  if (!user) return null;

  async function createFilter(event: React.FormEvent) {
    event.preventDefault();
    if (!filterName.trim()) return;
    try {
      const filter = await saveFilter(filterName, { q: filterSearch });
      setFilters((current) => [...current, filter]);
      setFilterName("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t(locale, "auth.error"));
    }
  }

  async function createAnswer(event: React.FormEvent) {
    event.preventDefault();
    if (!snippetName.trim() || !snippetBody.trim()) return;
    try {
      const snippet = await createSnippet(snippetName, snippetBody);
      setSnippets((current) => [...current, snippet]);
      setSnippetName("");
      setSnippetBody("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t(locale, "auth.error"));
    }
  }

  async function changeStatus(application: Application, status: Application["status"]) {
    try {
      const updated = status === "submitted"
        ? await confirmSubmitted(application.id)
        : await updateApplication(application.id, status);
      setApplications((current) => current.map((item) => item.id === updated.id ? updated : item));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t(locale, "auth.error"));
    }
  }

  return (
    <div className="m5 space-y-6">
      {completeness && (
        <div className="rounded-xl border border-brand-border bg-brand-surface-2 p-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 className="m-0 text-base font-bold text-brand-text">{t(locale, "m5.completeness")}</h3>
            <strong className="text-brand-success">{completeness.score}%</strong>
          </div>
          {completeness.missing.length > 0 && (
            <p className="m-0 mt-2 text-sm text-brand-muted">{t(locale, "m5.missing", { fields: completeness.missing.join(", ") })}</p>
          )}
        </div>
      )}

      <section className="grid gap-4 lg:grid-cols-2">
        <div className="rounded-xl border border-brand-border bg-brand-surface-2 p-4">
          <h3 className="m-0 mb-3 text-base font-bold text-brand-text">{t(locale, "m5.savedFilters")}</h3>
          <form className="grid gap-2" onSubmit={createFilter}>
            <input value={filterName} onChange={(event) => setFilterName(event.target.value)} placeholder={t(locale, "m5.filterName")} aria-label={t(locale, "m5.filterName")} />
            <input value={filterSearch} onChange={(event) => setFilterSearch(event.target.value)} placeholder={t(locale, "m5.filterSearch")} aria-label={t(locale, "m5.filterSearch")} />
            <button className="btn rounded-xl border border-brand-primary-strong bg-brand-primary-strong px-3 py-2 text-sm font-semibold text-brand-on-primary" type="submit">{t(locale, "m5.saveFilter")}</button>
          </form>
          <ul className="mt-3 grid gap-2 p-0 text-sm text-brand-muted">
            {filters.map((filter) => (
              <li className="flex items-center justify-between gap-2" key={filter.id}>
                <span>{filter.name}</span>
                <button className="text-brand-danger" type="button" onClick={() => void deleteSavedFilter(filter.id).then(() => setFilters((current) => current.filter((item) => item.id !== filter.id))).catch((err: unknown) => setError(err instanceof ApiError ? err.message : t(locale, "auth.error")))}>{t(locale, "m5.remove")}</button>
              </li>
            ))}
          </ul>
        </div>

        <div className="rounded-xl border border-brand-border bg-brand-surface-2 p-4">
          <h3 className="m-0 mb-3 text-base font-bold text-brand-text">{t(locale, "m5.tracker")}</h3>
          {applications.length === 0 ? <p className="m-0 text-sm text-brand-muted">{t(locale, "m5.noApplications")}</p> : (
            <ul className="grid gap-3 p-0">
              {applications.map((application) => (
                <li className="flex flex-wrap items-center justify-between gap-2 text-sm" key={application.id}>
                  <span className="font-medium text-brand-text">{application.job_title || application.job_key}</span>
                  <select value={application.status} onChange={(event) => void changeStatus(application, event.target.value as Application["status"])} aria-label={application.job_title || application.job_key}>
                    {(["form_filled", "submitted", "viewed", "rejected", "interview"] as const).map((status) => <option key={status} value={status}>{status.replace("_", " ")}</option>)}
                  </select>
                </li>
              ))}
            </ul>
          )}
        </div>
      </section>

      <section className="rounded-xl border border-brand-border bg-brand-surface-2 p-4">
        <h3 className="m-0 mb-3 text-base font-bold text-brand-text">{t(locale, "m5.snippets")}</h3>
        <form className="grid gap-2" onSubmit={createAnswer}>
          <input value={snippetName} onChange={(event) => setSnippetName(event.target.value)} placeholder={t(locale, "m5.snippetName")} aria-label={t(locale, "m5.snippetName")} />
          <textarea rows={3} value={snippetBody} onChange={(event) => setSnippetBody(event.target.value)} placeholder={t(locale, "m5.snippetBody")} aria-label={t(locale, "m5.snippetBody")} />
          <button className="btn w-fit rounded-xl border border-brand-primary-strong bg-brand-primary-strong px-3 py-2 text-sm font-semibold text-brand-on-primary" type="submit">{t(locale, "m5.saveSnippet")}</button>
        </form>
        <ul className="mt-3 grid gap-2 p-0 text-sm text-brand-muted">
          {snippets.map((snippet) => (
            <li className="flex flex-wrap items-center justify-between gap-2" key={snippet.id}>
              <span><strong className="text-brand-text">{snippet.name}</strong>: {snippet.body}</span>
              <button className="text-brand-danger" type="button" onClick={() => void deleteSnippet(snippet.id).then(() => setSnippets((current) => current.filter((item) => item.id !== snippet.id))).catch((err: unknown) => setError(err instanceof ApiError ? err.message : t(locale, "auth.error")))}>{t(locale, "m5.remove")}</button>
            </li>
          ))}
        </ul>
      </section>
      {error && <p className="m-0 rounded-xl border border-brand-danger bg-brand-danger-soft px-4 py-3 text-sm text-brand-danger" role="alert">{error}</p>}
    </div>
  );
}
