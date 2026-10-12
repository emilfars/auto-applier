import { describe, it, expect } from "vitest";
import nginxConf from "../../nginx.conf?raw";
import viteConfig from "../../vite.config.ts?raw";

// Every API path the client calls must be proxied to the backend by nginx
// (compose/production) and by the Vite dev server; an unproxied path falls
// through to the SPA and returns index.html.
const clientSources = import.meta.glob(["./*.ts", "!./*.test.ts"], {
  eager: true,
  import: "default",
  query: "?raw",
}) as Record<string, string>;

function clientPrefixes(): string[] {
  const found = new Set<string>();
  for (const source of Object.values(clientSources)) {
    for (const match of source.matchAll(/["`](\/[a-z][a-z-]*)/g)) found.add(match[1]);
  }
  return [...found].sort();
}

describe("API proxy prefixes", () => {
  const prefixes = clientPrefixes();

  it("finds the client's API prefixes", () => {
    expect(prefixes).toEqual(
      expect.arrayContaining(["/auth", "/cv", "/profile", "/feed", "/saved-filters", "/applications", "/snippets", "/jobs"]),
    );
  });

  it("nginx proxies every prefix the client calls", () => {
    const location = /location ~ \^\/\(([^)]+)\)/.exec(nginxConf);
    const proxied = (location?.[1] ?? "").split("|").map((p) => `/${p}`);
    expect(prefixes.filter((p) => !proxied.includes(p))).toEqual([]);
  });

  it("the Vite dev server proxies every prefix the client calls", () => {
    expect(prefixes.filter((p) => !viteConfig.includes(`"${p}"`))).toEqual([]);
  });
});
