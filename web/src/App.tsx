import { useState } from "react";
import { t, locales, DEFAULT_LOCALE, type Locale } from "./i18n";

export default function App() {
  const [locale, setLocale] = useState<Locale>(DEFAULT_LOCALE);

  return (
    <main>
      <header>
        <h1>{t(locale, "app.title")}</h1>
        <p>{t(locale, "app.tagline")}</p>
        <p role="note">{t(locale, "app.safety")}</p>
      </header>
      <nav>
        <a href="#feed">{t(locale, "nav.feed")}</a>
        <a href="#profile">{t(locale, "nav.profile")}</a>
      </nav>
      <label>
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
    </main>
  );
}
