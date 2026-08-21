import { useState } from "react";
import { t, locales, DEFAULT_LOCALE, type Locale } from "./i18n";
import { JobFeed } from "./components/JobFeed";
import { AuthPanel } from "./components/AuthPanel";
import { ProfilePanel } from "./components/ProfilePanel";
import { CvPanel } from "./components/CvPanel";
import { useSession } from "./auth/session";
import { ThemeToggle } from "./theme";

export default function App() {
  const [locale, setLocale] = useState<Locale>(DEFAULT_LOCALE);
  const { user } = useSession();
  const [profileRefresh, setProfileRefresh] = useState(0);
  const [profileConfirmed, setProfileConfirmed] = useState(false);

  const canFill = !!user && profileConfirmed;
  const fillReason: "needLogin" | "needProfile" | undefined = !user
    ? "needLogin"
    : !profileConfirmed
      ? "needProfile"
      : undefined;

  return (
    <main className="app min-h-screen bg-brand-bg text-brand-text">
      <header className="app__header sticky top-0 z-30 border-b border-brand-border bg-brand-bg shadow-sm">
        <div className="mx-auto max-w-app px-4 py-4 sm:px-6 lg:px-8">
          <div className="app__brand flex flex-wrap items-center justify-between gap-4">
            <h1 className="text-2xl font-black tracking-tight text-brand-text sm:text-3xl">
              {t(locale, "app.title")}
            </h1>
            <div className="flex items-center gap-2">
              <label className="app__lang inline-flex items-center gap-2 text-sm text-brand-muted">
            Language
            <select
              className="w-auto rounded-lg py-1 text-sm"
              value={locale}
              onChange={(e) => setLocale(e.target.value as Locale)}
            >
              {locales.map((l) => (
                <option key={l} value={l}>
                  {l}
              </option>
            ))}
              </select>
              </label>
              <ThemeToggle />
            </div>
          </div>
          <p className="app__tagline mt-3 max-w-2xl text-base leading-7 text-brand-muted">
            {t(locale, "app.tagline")}
          </p>
          <p
            className="app__safety mt-4 rounded-xl border border-brand-success bg-brand-success-soft px-4 py-3 text-sm font-medium text-brand-text"
            role="note"
          >
            {t(locale, "app.safety")}
          </p>
          <nav className="app__nav flex flex-wrap gap-x-6 gap-y-2 border-t border-brand-border pt-4 text-sm font-semibold">
            <a className="text-brand-accent-light" href="#feed">
              {t(locale, "nav.feed")}
            </a>
            <a className="text-brand-accent-light" href="#account">
              {t(locale, "nav.account")}
            </a>
            {user && (
              <a className="text-brand-accent-light" href="#profile">
                {t(locale, "nav.profile")}
              </a>
            )}
          </nav>
        </div>
      </header>

      <div className="mx-auto max-w-app px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
        <section
          id="account"
          className="panel mb-6 rounded-2xl border border-brand-border bg-brand-surface p-5 shadow-xl sm:p-7"
        >
          <h2 className="panel__heading mb-5 text-xl font-bold tracking-tight text-brand-text">
            {t(locale, "auth.heading")}
          </h2>
          <AuthPanel locale={locale} />
        </section>

        {user && (
          <>
            <section
              id="cv"
              className="panel mb-6 rounded-2xl border border-brand-border bg-brand-surface p-5 shadow-xl sm:p-7"
            >
              <h2 className="panel__heading mb-5 text-xl font-bold tracking-tight text-brand-text">
                {t(locale, "cv.heading")}
              </h2>
              <CvPanel locale={locale} onParsed={() => setProfileRefresh((n) => n + 1)} />
            </section>

            <section
              id="profile"
              className="panel mb-6 rounded-2xl border border-brand-border bg-brand-surface p-5 shadow-xl sm:p-7"
            >
              <h2 className="panel__heading mb-5 text-xl font-bold tracking-tight text-brand-text">
                {t(locale, "profile.heading")}
              </h2>
              <ProfilePanel
                key={profileRefresh}
                locale={locale}
                onConfirmedChange={setProfileConfirmed}
              />
            </section>
          </>
        )}

        <JobFeed locale={locale} canFill={canFill} fillReason={fillReason} />
      </div>
    </main>
  );
}
