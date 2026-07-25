import { useState } from "react";
import { t, locales, DEFAULT_LOCALE, type Locale } from "./i18n";
import { JobFeed } from "./components/JobFeed";
import { AuthPanel } from "./components/AuthPanel";

export default function App() {
  const [locale, setLocale] = useState<Locale>(DEFAULT_LOCALE);

  return (
    <main className="app">
      <header className="app__header">
        <div className="app__brand">
          <h1>{t(locale, "app.title")}</h1>
          <label className="app__lang">
            Language
            <select
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
        </div>
        <p className="app__tagline">{t(locale, "app.tagline")}</p>
        <p className="app__safety" role="note">
          {t(locale, "app.safety")}
        </p>
        <nav className="app__nav">
          <a href="#feed">{t(locale, "nav.feed")}</a>
          <a href="#account">{t(locale, "nav.account")}</a>
        </nav>
      </header>

      <section id="account" className="panel">
        <h2 className="panel__heading">{t(locale, "auth.heading")}</h2>
        <AuthPanel locale={locale} />
      </section>

      <JobFeed locale={locale} />
    </main>
  );
}
