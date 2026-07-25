import en from "./locales/en.json";
import id from "./locales/id.json";

export const DEFAULT_LOCALE = "id-ID" as const;
export const DEFAULT_CURRENCY = "IDR" as const;

export type Locale = "id-ID" | "en";

const bundles: Record<Locale, Record<string, string>> = {
  "id-ID": id,
  en,
};

export type TranslationKey = keyof typeof en;

/** Translate a key for a locale, falling back to the key itself if missing.
 * Optional params interpolate `{name}` placeholders in the translation. */
export function t(
  locale: Locale,
  key: TranslationKey,
  params?: Record<string, string | number>,
): string {
  let s = bundles[locale][key] ?? key;
  if (params) {
    for (const [name, value] of Object.entries(params)) {
      s = s.replace(new RegExp(`\\{${name}\\}`, "g"), String(value));
    }
  }
  return s;
}

/** Format a number as currency, defaulting to IDR. */
export function formatCurrency(
  amount: number,
  locale: Locale = DEFAULT_LOCALE,
  currency: string = DEFAULT_CURRENCY,
): string {
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
    maximumFractionDigits: 0,
  }).format(amount);
}

export const locales = Object.keys(bundles) as Locale[];
export { bundles };
