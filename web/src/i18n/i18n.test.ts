import { describe, it, expect } from "vitest";
import {
  bundles,
  locales,
  t,
  DEFAULT_LOCALE,
  DEFAULT_CURRENCY,
  formatCurrency,
  type TranslationKey,
} from "./index";
import en from "./locales/en.json";

// AC-NFR-I18N: UI strings resolve for id-ID and en; currency defaults to IDR.
describe("AC-NFR-I18N i18n locale bundles", () => {
  const keys = Object.keys(en) as TranslationKey[];

  it("has both required locales", () => {
    expect(locales).toContain("id-ID");
    expect(locales).toContain("en");
  });

  it("has no missing keys in any locale bundle", () => {
    for (const locale of locales) {
      for (const key of keys) {
        expect(
          Object.prototype.hasOwnProperty.call(bundles[locale], key),
          `locale ${locale} missing key ${key}`,
        ).toBe(true);
        expect(t(locale, key)).not.toBe("");
      }
    }
  });

  it("has no extra keys beyond the canonical (en) set in any locale", () => {
    const canonical = new Set(keys);
    for (const locale of locales) {
      for (const key of Object.keys(bundles[locale])) {
        expect(canonical.has(key as TranslationKey), `locale ${locale} has stray key ${key}`).toBe(true);
      }
    }
  });

  it("defaults to id-ID locale and IDR currency", () => {
    expect(DEFAULT_LOCALE).toBe("id-ID");
    expect(DEFAULT_CURRENCY).toBe("IDR");
    const formatted = formatCurrency(8_000_000);
    expect(formatted).toMatch(/Rp/);
  });
});
