import { describe, expect, it } from "vitest";
import html from "../index.html?raw";
import id from "./i18n/locales/id.json";

describe("boot shell in index.html", () => {
  it.each(["app.title", "app.tagline", "feed.heading", "feed.loading"] as const)(
    "shows the same %s text as the default locale",
    (key) => {
      expect(html).toContain(`>${id[key]}<`);
    },
  );
});
