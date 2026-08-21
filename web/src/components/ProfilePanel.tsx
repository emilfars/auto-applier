import { useCallback, useEffect, useState } from "react";
import { t, type Locale } from "../i18n";
import { useSession } from "../auth/session";
import {
  confirmProfile,
  getProfile,
  patchProfile,
  EMPLOYMENT_TYPES,
  type Profile,
  type ProfilePatch,
} from "../api/profile";
import { ApiError } from "../api/http";

interface FormState {
  full_name: string;
  email: string;
  phone: string;
  linkedin_url: string;
  github_url: string;
  portfolio_url: string;
  address: string;
  city: string;
  summary: string;
  current_employer: string;
  current_title: string;
  highest_education: string;
  education: string;
  work_history: string;
  expected_salary: string;
  notice_period_days: string;
  work_authorization: string;
  employment_type: string;
  open_to_relocation: boolean;
  skills: string;
  preferred_locations: string;
}

function toForm(p: Profile): FormState {
  return {
    full_name: p.full_name,
    email: p.email,
    phone: p.phone,
    linkedin_url: p.linkedin_url,
    github_url: p.github_url,
    portfolio_url: p.portfolio_url,
    address: p.address,
    city: p.city,
    summary: p.summary,
    current_employer: p.current_employer,
    current_title: p.current_title,
    highest_education: p.highest_education,
    education: JSON.stringify(p.education ?? [], null, 2),
    work_history: JSON.stringify(p.work_history ?? [], null, 2),
    expected_salary: p.expected_salary != null ? String(p.expected_salary) : "",
    notice_period_days: p.notice_period_days != null ? String(p.notice_period_days) : "",
    work_authorization: p.work_authorization,
    employment_type: p.employment_type,
    open_to_relocation: p.open_to_relocation,
    skills: (p.skills ?? []).join(", "),
    preferred_locations: (p.preferred_locations ?? []).join(", "),
  };
}

function splitList(s: string): string[] {
  return s
    .split(",")
    .map((x) => x.trim())
    .filter(Boolean);
}

function toPatch(f: FormState): ProfilePatch {
  const patch: ProfilePatch = {
    full_name: f.full_name,
    email: f.email,
    phone: f.phone,
    linkedin_url: f.linkedin_url,
    github_url: f.github_url,
    portfolio_url: f.portfolio_url,
    address: f.address,
    city: f.city,
    summary: f.summary,
    current_employer: f.current_employer,
    current_title: f.current_title,
    highest_education: f.highest_education,
    education: JSON.parse(f.education) as Profile["education"],
    work_history: JSON.parse(f.work_history) as Profile["work_history"],
    work_authorization: f.work_authorization,
    employment_type: f.employment_type,
    open_to_relocation: f.open_to_relocation,
    skills: splitList(f.skills),
    preferred_locations: splitList(f.preferred_locations),
    expected_salary: f.expected_salary.trim() ? Number(f.expected_salary) : null,
    notice_period_days: f.notice_period_days.trim() ? Number(f.notice_period_days) : null,
  };
  return patch;
}

/**
 * ProfilePanel lets a signed-in user review and edit the profile that backs
 * application autofill, then confirm it. Editing resets confirmation, so the
 * user always re-reviews before the fill flow can be armed (CV-3/4/5).
 * `onConfirmedChange` lets the feed enable/disable Open & Fill.
 */
export function ProfilePanel({
  locale,
  onConfirmedChange,
}: {
  locale: Locale;
  onConfirmedChange?: (confirmed: boolean) => void;
}) {
  const { user } = useSession();
  const [form, setForm] = useState<FormState | null>(null);
  const [confirmed, setConfirmed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const applyProfile = useCallback(
    (p: Profile) => {
      setForm(toForm(p));
      setConfirmed(p.confirmed);
      onConfirmedChange?.(p.confirmed);
    },
    [onConfirmedChange],
  );

  useEffect(() => {
    if (!user) {
      setForm(null);
      return;
    }
    const controller = new AbortController();
    getProfile(controller.signal)
      .then(applyProfile)
      .catch((err: unknown) => {
        if (err instanceof DOMException && err.name === "AbortError") return;
        setError(t(locale, "auth.error"));
      });
    return () => controller.abort();
  }, [user, applyProfile, locale]);

  if (!user) {
    return <p className="panel__status text-sm text-brand-muted">{t(locale, "profile.loginRequired")}</p>;
  }
  if (!form) {
    return (
      <div className="panel__status grid animate-pulse gap-3" aria-live="polite">
        <span className="h-10 rounded-xl bg-brand-surface-2" />
        <span className="h-10 rounded-xl bg-brand-surface-2" />
        <span className="h-24 rounded-xl bg-brand-surface-2" />
        <span className="sr-only">{t(locale, "common.loading")}</span>
      </div>
    );
  }

  const set = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((prev) => (prev ? { ...prev, [key]: value } : prev));

  async function run(fn: () => Promise<void>) {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await fn();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t(locale, "auth.error"));
    } finally {
      setBusy(false);
    }
  }

  const onSave = (e: React.FormEvent) => {
    e.preventDefault();
    if (!form) return;
    void run(async () => {
      const saved = await patchProfile(toPatch(form));
      applyProfile(saved);
      setNotice(t(locale, "profile.saved"));
    });
  };

  const onConfirm = () => {
    if (!form) return;
    void run(async () => {
      await patchProfile(toPatch(form));
      const saved = await confirmProfile();
      applyProfile(saved);
      setNotice(t(locale, "profile.confirmed"));
    });
  };

  return (
    <div className="profile space-y-6">
      <form
        className="profile__form grid grid-cols-1 gap-4 md:grid-cols-2 [&>label]:grid [&>label]:gap-2 [&>label]:text-sm [&>label]:font-medium [&>label]:text-brand-muted"
        onSubmit={onSave}
      >
        <label className="full md:col-span-2">
          {t(locale, "profile.fullName")}
          <input required value={form.full_name} onChange={(e) => set("full_name", e.target.value)} />
        </label>
        <label>
          {t(locale, "profile.email")}
          <input required type="email" value={form.email} onChange={(e) => set("email", e.target.value)} />
        </label>
        <label>
          {t(locale, "profile.phone")}
          <input required value={form.phone} onChange={(e) => set("phone", e.target.value)} />
        </label>
        <label>
          {t(locale, "profile.linkedin")}
          <input type="url" value={form.linkedin_url} onChange={(e) => set("linkedin_url", e.target.value)} />
        </label>
        <label>
          {t(locale, "profile.github")}
          <input type="url" value={form.github_url} onChange={(e) => set("github_url", e.target.value)} />
        </label>
        <label>
          {t(locale, "profile.portfolio")}
          <input type="url" value={form.portfolio_url} onChange={(e) => set("portfolio_url", e.target.value)} />
        </label>
        <label>
          {t(locale, "profile.city")}
          <input value={form.city} onChange={(e) => set("city", e.target.value)} />
        </label>
        <label className="full md:col-span-2">
          {t(locale, "profile.address")}
          <input value={form.address} onChange={(e) => set("address", e.target.value)} />
        </label>
        <label className="full md:col-span-2">
          {t(locale, "profile.summary")}
          <textarea rows={4} value={form.summary} onChange={(e) => set("summary", e.target.value)} />
        </label>
        <label>
          {t(locale, "profile.currentEmployer")}
          <input value={form.current_employer} onChange={(e) => set("current_employer", e.target.value)} />
        </label>
        <label>
          {t(locale, "profile.currentTitle")}
          <input value={form.current_title} onChange={(e) => set("current_title", e.target.value)} />
        </label>
        <label>
          {t(locale, "profile.highestEducation")}
          <input value={form.highest_education} onChange={(e) => set("highest_education", e.target.value)} />
        </label>
        <label>
          {t(locale, "profile.workAuth")}
          <input
            required
            value={form.work_authorization}
            onChange={(e) => set("work_authorization", e.target.value)}
          />
        </label>
        <label>
          {t(locale, "profile.expectedSalary")}
          <input
            type="number"
            min={0}
            value={form.expected_salary}
            onChange={(e) => set("expected_salary", e.target.value)}
          />
        </label>
        <label>
          {t(locale, "profile.noticePeriod")}
          <input
            type="number"
            min={0}
            max={365}
            value={form.notice_period_days}
            onChange={(e) => set("notice_period_days", e.target.value)}
          />
        </label>
        <label>
          {t(locale, "profile.employmentType")}
          <select
            required
            value={form.employment_type}
            onChange={(e) => set("employment_type", e.target.value)}
          >
            <option value="">—</option>
            {EMPLOYMENT_TYPES.map((et) => (
              <option key={et} value={et}>
                {et.replace("_", " ")}
              </option>
            ))}
          </select>
        </label>
        <label className="profile__checkbox !flex !grid-cols-none items-center gap-2">
          <input
            type="checkbox"
            checked={form.open_to_relocation}
            onChange={(e) => set("open_to_relocation", e.target.checked)}
          />
          {t(locale, "profile.relocation")}
        </label>
        <label className="full md:col-span-2">
          {t(locale, "profile.education")}
          <textarea
            required
            rows={8}
            value={form.education}
            onChange={(e) => set("education", e.target.value)}
          />
        </label>
        <label className="full md:col-span-2">
          {t(locale, "profile.workHistory")}
          <textarea
            rows={8}
            value={form.work_history}
            onChange={(e) => set("work_history", e.target.value)}
          />
        </label>
        <label className="full md:col-span-2">
          {t(locale, "profile.skills")}
          <input required value={form.skills} onChange={(e) => set("skills", e.target.value)} />
        </label>
        <label className="full md:col-span-2">
          {t(locale, "profile.preferredLocations")}
          <input
            required
            value={form.preferred_locations}
            onChange={(e) => set("preferred_locations", e.target.value)}
          />
        </label>
        <div className="full md:col-span-2">
          <button
            type="submit"
            className="btn btn--primary rounded-xl border border-brand-primary-strong bg-brand-primary-strong px-4 py-2.5 font-semibold text-brand-on-primary transition hover:bg-brand-primary disabled:cursor-not-allowed disabled:opacity-60"
            disabled={busy}
          >
            {t(locale, "profile.save")}
          </button>
        </div>
      </form>

      <div className="profile__confirm border-t border-brand-border pt-5">
        {confirmed ? (
          <p className="panel__status flex items-center gap-2 text-sm font-semibold text-brand-success">
            <span className="badge badge--confirmed inline-flex h-6 w-6 items-center justify-center rounded-full bg-brand-success text-sm text-brand-bg">✓</span>
            {t(locale, "profile.confirmed")}
          </p>
        ) : (
          <>
            <p className="profile__confirm-note m-0 mb-4 max-w-3xl text-sm leading-6 text-brand-muted">
              {t(locale, "profile.confirmNote")}
            </p>
            <button
              type="button"
              className="btn rounded-xl border border-brand-border bg-brand-surface-2 px-4 py-2.5 font-semibold text-brand-text transition hover:border-brand-primary disabled:cursor-not-allowed disabled:opacity-60"
              disabled={busy}
              onClick={onConfirm}
            >
              {t(locale, "profile.confirm")}
            </button>
          </>
        )}
      </div>

      {error && (
          <p className="panel__status panel__status--error rounded-xl border border-brand-danger bg-brand-danger-soft px-4 py-3 text-sm text-brand-danger" role="alert">
          {error}
        </p>
      )}
      {notice && <p className="panel__status rounded-xl border border-brand-success bg-brand-success-soft px-4 py-3 text-sm text-brand-success">{notice}</p>}
    </div>
  );
}
