#!/usr/bin/env node
// E-STATE (plan-revamp.md section 5): screenshot every screen in every
// theme x width x state x locale against the in-memory API and the production
// web build, and fail on a missing cell, horizontal page scroll, clipped text,
// or a page background that is not the brand token.
//
// Usage: PW_CORE=/dir/with/node_modules node scripts/ui-matrix.mjs <out-dir>
// MATRIX_SCREENS=cv,profile limits the run to those screens.
// MATRIX_DIST=/path serves a copy of web/dist (implies no rebuild of that copy).
// `node scripts/ui-matrix.mjs --serve` only starts the stack and prints its URL.
// playwright-core is not a repo dependency; install it anywhere
// (`npm i playwright-core` in a scratch dir) and point PW_CORE at that dir.
// Chrome comes from CHROME_BIN or .cft/, same as browser-e2e.mjs.
import { spawn, spawnSync } from "node:child_process";
import { createRequire } from "node:module";
import { createServer, request as httpRequest } from "node:http";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { once } from "node:events";
import { tmpdir } from "node:os";
import { extname, join, normalize } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const OUT = process.argv[2];
if (!OUT) {
  console.error("usage: node scripts/ui-matrix.mjs <out-dir>");
  process.exit(2);
}
const { chromium } = OUT === "--serve" ? {} : createRequire(join(process.env.PW_CORE ?? ROOT, "noop.js"))("playwright-core");

const THEMES = ["dark", "light"];
const WIDTHS = [360, 768, 1280];
const LOCALES = ["id-ID", "en"];
const STATES = ["loading", "empty", "error", "populated"];
const WAIT_MS = Number(process.env.MATRIX_WAIT_MS ?? 5_000);
const EMPTY_FEED = { jobs: [], total: 0, limit: 24, offset: 0 };
// api: GET paths whose response defines the screen's state. act: interaction
// that produces the error state on screens whose error only follows an action.
const SCREENS = [
  { id: "auth", selector: "#account", signedIn: false, api: ["/auth/me"], skip: { empty: "form has no empty state" }, errorAct: "badLogin" },
  { id: "feed", selector: "#feed", signedIn: false, api: ["/feed"], emptyBody: EMPTY_FEED },
  { id: "cv", selector: "#cv", signedIn: true, api: ["/cv"] },
  { id: "profile", selector: "#profile", signedIn: true, api: ["/profile"] },
  { id: "enhancements", selector: "#enhancements", signedIn: true, api: ["/saved-filters", "/applications", "/snippets", "/profile/completeness"] },
  { id: "account", selector: "#account", signedIn: true, api: [], skip: { loading: "no fetch on mount", empty: "no list" }, errorAct: "badExport" },
];

function chromePath() {
  const candidates = [
    process.env.CHROME_BIN,
    join(ROOT, ".cft/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing"),
    join(ROOT, ".cft/chrome-mac-x64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing"),
    join(ROOT, ".cft/chrome-linux64/chrome"),
    join(ROOT, ".cft/chrome-win64/chrome.exe"),
  ].filter(Boolean);
  const found = candidates.find((c) => existsSync(c));
  if (!found) throw new Error("ui-matrix: Chrome not found; set CHROME_BIN");
  return found;
}

async function freePort() {
  const server = createServer();
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const { port } = server.address();
  server.close();
  await once(server, "close");
  return port;
}

const MIME = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".json": "application/json", ".svg": "image/svg+xml" };

// Serves web/dist and proxies everything that is not a built file to the API,
// which is what nginx does in the compose stack.
function serveWeb(apiPort) {
  const dist = process.env.MATRIX_DIST ?? join(ROOT, "web", "dist");
  return createServer((req, res) => {
    const { pathname } = new URL(req.url, "http://localhost");
    const file = normalize(join(dist, pathname === "/" ? "index.html" : pathname));
    if (file.startsWith(dist) && existsSync(file) && statSync(file).isFile()) {
      res.writeHead(200, { "Content-Type": MIME[extname(file)] ?? "application/octet-stream" });
      res.end(readFileSync(file));
      return;
    }
    const upstream = httpRequest({ host: "127.0.0.1", port: apiPort, path: req.url, method: req.method, headers: req.headers }, (up) => {
      // The session cookie is Secure; this harness is plain http, so drop the flag here only.
      const cookies = up.headers["set-cookie"]?.map((c) => c.replace(/; Secure/i, ""));
      res.writeHead(up.statusCode ?? 502, cookies ? { ...up.headers, "set-cookie": cookies } : up.headers);
      up.pipe(res);
    });
    upstream.on("error", () => res.writeHead(502).end());
    req.pipe(upstream);
  });
}

async function waitForHTTP(url) {
  for (let i = 0; i < 100; i++) {
    try {
      if ((await fetch(url)).ok) return;
    } catch {
      // not up yet
    }
    await new Promise((r) => setTimeout(r, 200));
  }
  throw new Error(`ui-matrix: ${url} did not come up`);
}

async function signUp(request, base, seed) {
  const email = `ui-matrix-${seed}-${Date.now()}@example.test`;
  const password = "matrix-password-1";
  const reg = await (await request.post(`${base}/auth/register`, { data: { email, password, consent: true } })).json();
  await request.post(`${base}/auth/verify`, { data: { token: reg.verification_token } });
  await request.post(`${base}/auth/login`, { data: { email, password } });
  const me = await request.get(`${base}/auth/me`);
  if (!me.ok()) throw new Error(`ui-matrix: sign-in for the ${seed} user failed (${me.status()})`);
  if (seed !== "populated") return;
  await request.post(`${base}/cv`, {
    multipart: { file: { name: "cv-contoh.pdf", mimeType: "application/pdf", buffer: Buffer.from("%PDF-1.4\nui matrix\n%%EOF\n") } },
  });
  await request.patch(`${base}/profile`, {
    data: {
      full_name: "Dewi Lestari Kusumawardhani",
      email,
      phone: "+6281234567890",
      city: "Jakarta Selatan",
      summary: "Backend engineer yang membangun sistem pembayaran andal untuk jutaan pengguna.",
      current_employer: "Nusantara Digital",
      current_title: "Senior Software Engineer",
      highest_education: "S1 Ilmu Komputer",
      education: [{ institution: "Universitas Indonesia", degree: "S.Kom", field: "Ilmu Komputer", start_year: "2015", end_year: "2019" }],
      work_history: [{ company: "Nusantara Digital", title: "Senior Software Engineer", start_date: "2020-01", end_date: "" }],
      skills: ["Go", "TypeScript", "PostgreSQL"],
      expected_salary: 15000000,
      notice_period_days: 30,
      work_authorization: "authorized",
      preferred_locations: ["Jakarta", "Depok"],
      employment_type: "full_time",
    },
  });
  await request.post(`${base}/saved-filters`, { data: { name: "Backend Jakarta di atas 12 juta", query: { q: "backend", location: "Jakarta", pay_min: 12000000 } } });
  await request.post(`${base}/snippets`, { data: { name: "Salam pembuka", body: "Halo tim {company}, saya {name} dan tertarik dengan posisi {role}." } });
}

const isAPI = (request, paths) => {
  if (!["fetch", "xhr"].includes(request.resourceType()) || request.method() !== "GET") return false;
  const { pathname } = new URL(request.url());
  return paths.some((p) => pathname === p || pathname.startsWith(`${p}/`));
};

async function capture(browser, base, storage, screen, theme, width, locale, state) {
  const context = await browser.newContext({
    viewport: { width, height: 900 },
    storageState: screen.signedIn ? storage[state === "empty" ? "empty" : "populated"] : undefined,
    reducedMotion: "reduce",
  });
  let layout;
  let file;
  const consoleErrors = [];
  try {
    await context.addInitScript((t) => window.localStorage.setItem("auto-applier-theme", t), theme);
    const page = await context.newPage();
    page.setDefaultTimeout(Math.max(WAIT_MS, 5_000));
    page.on("console", (m) => {
      if (m.type() === "error" || m.type() === "warning") consoleErrors.push(m.text());
    });
    page.on("pageerror", (e) => consoleErrors.push(String(e)));

    if (state === "loading") await page.route(() => true, (route) => (isAPI(route.request(), screen.api) ? undefined : route.continue()));
    if (state === "error" && !screen.errorAct) {
      await page.route(() => true, (route) =>
        isAPI(route.request(), screen.api) ? route.fulfill({ status: 500, contentType: "application/json", body: '{"error":"internal error"}' }) : route.continue(),
      );
    }
    if (state === "empty" && screen.emptyBody) {
      await page.route(() => true, (route) =>
        isAPI(route.request(), screen.api) ? route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(screen.emptyBody) }) : route.continue(),
      );
    }

    await page.goto(base, { waitUntil: "domcontentloaded" });
    if (locale !== "id-ID") {
      await page.getByLabel(/^(language|bahasa)$/i).first().click();
      await page.getByRole("option", { name: locale, exact: true }).click();
    }
    const target = page.locator(screen.selector).first();
    await target.waitFor({ state: "visible", timeout: WAIT_MS }).catch((error) => {
      throw new Error(consoleErrors.length ? `screen did not render: ${consoleErrors[0].split("\n")[0]}` : String(error));
    });
    if (state === "error" && screen.errorAct === "badLogin") {
      await target.locator('input[type="email"]').fill("nobody@example.test");
      await target.locator('input[type="password"]').fill("wrong-password-1");
      await target.locator('button[type="submit"]').first().click();
      await target.getByRole("alert").first().waitFor({ timeout: 10_000 });
    }
    if (state === "error" && screen.errorAct === "badExport") {
      await page.route("**/account/export", (route) => route.fulfill({ status: 500, contentType: "application/json", body: '{"error":"internal error"}' }));
      await target.locator('[data-testid="account-export"]').click();
      await target.getByRole("alert").first().waitFor({ timeout: 10_000 });
    }
    await page.waitForTimeout(state === "loading" ? 400 : 900);
    await target.scrollIntoViewIfNeeded();

    layout = await page.evaluate((selector) => {
      const root = document.querySelector(selector);
      const clipped = [];
      for (const el of root.querySelectorAll("*")) {
        if (!el.childNodes.length || ![...el.childNodes].some((n) => n.nodeType === 3 && n.textContent.trim())) continue;
        const style = getComputedStyle(el);
        const intended = style.textOverflow === "ellipsis" || style.webkitLineClamp !== "none";
        if (!intended && style.overflowX !== "visible" && el.scrollWidth > el.clientWidth + 1) clipped.push(el.textContent.trim().slice(0, 40));
        const box = el.getBoundingClientRect();
        if (box.width > 0 && box.right > window.innerWidth + 1) clipped.push(`offscreen: ${el.textContent.trim().slice(0, 40)}`);
      }
      const probe = document.createElement("div");
      probe.style.background = "var(--brand-bg)";
      document.body.append(probe);
      const brandBg = getComputedStyle(probe).backgroundColor;
      probe.remove();
      return {
        pageOverflow: document.documentElement.scrollWidth > window.innerWidth,
        clipped,
        offBrand: getComputedStyle(document.body).backgroundColor !== brandBg,
      };
    }, screen.selector);

    file = `${screen.id}__${theme}__${width}__${state}__${locale}.png`;
    await target.screenshot({ path: join(OUT, file) });
  } finally {
    await context.close();
  }
  const expected = state === "error" || state === "loading" ? /Failed to load resource|status of (401|500)/ : /status of 401/;
  return { file, ...layout, consoleErrors: consoleErrors.filter((e) => !expected.test(e)) };
}

async function main() {
  const tmp = mkdtempSync(join(tmpdir(), "aa-matrix-"));
  const children = [];
  let browser;
  let web;
  try {
    if (!process.env.SKIP_BUILD) {
      if (spawnSync("npm", ["run", "build"], { cwd: join(ROOT, "web"), stdio: "inherit" }).status !== 0) throw new Error("web build failed");
    }
    const api = join(tmp, "api");
    if (spawnSync("go", ["build", "-o", api, "./cmd/api"], { cwd: join(ROOT, "backend"), stdio: "inherit" }).status !== 0) throw new Error("api build failed");
    const apiPort = await freePort();
    const webPort = await freePort();
    children.push(
      spawn(api, [], {
        env: {
          ...process.env,
          API_ADDR: `127.0.0.1:${apiPort}`,
          AUTH_DEV_EXPOSE_TOKENS: "true",
          AUTH_DEV_ALLOW_IN_MEMORY_REGISTRATION: "true",
          CV_ENCRYPTION_KEY: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
          FEED_SEED_COUNT: "60",
          S3_ENDPOINT: "",
          DATABASE_URL: "",
        },
        stdio: "ignore",
      }),
    );
    web = serveWeb(apiPort);
    web.listen(webPort, "localhost");
    await once(web, "listening");
    const base = `http://localhost:${webPort}`;
    await waitForHTTP(`http://127.0.0.1:${apiPort}/healthz`);

    if (OUT === "--serve") {
      console.log(base);
      await new Promise((resolve) => process.once("SIGINT", resolve).once("SIGTERM", resolve));
      return;
    }
    mkdirSync(OUT, { recursive: true });
    browser = await chromium.launch({ executablePath: chromePath(), headless: true });
    const storage = {};
    for (const seed of ["empty", "populated"]) {
      if (process.env.MATRIX_DEBUG) console.error(`sign up ${seed}`);
      const context = await browser.newContext();
      await signUp(context.request, base, seed);
      storage[seed] = await context.storageState();
      await context.close();
    }

    const cells = [];
    const only = process.env.MATRIX_SCREENS?.split(",");
    for (const screen of SCREENS.filter((sc) => !only || only.includes(sc.id)))
      for (const theme of THEMES)
        for (const width of WIDTHS)
          for (const state of STATES)
            for (const locale of LOCALES) {
              const cell = { screen: screen.id, theme, width, state, locale };
              if (screen.skip?.[state]) {
                cells.push({ ...cell, na: screen.skip[state] });
                continue;
              }
              if (process.env.MATRIX_DEBUG) console.error(JSON.stringify(cell));
              try {
                cells.push({ ...cell, ...(await capture(browser, base, storage, screen, theme, width, locale, state)) });
              } catch (error) {
                cells.push({ ...cell, missing: String(error).split("\n")[0] });
                if (process.env.MATRIX_DEBUG) console.error(String(error));
              }
            }

    writeFileSync(join(OUT, "matrix.json"), JSON.stringify(cells, null, 1));
    const count = (pred) => cells.filter(pred).length;
    const summary = {
      cells: cells.length,
      captured: count((c) => c.file),
      notApplicable: count((c) => c.na),
      missing: count((c) => c.missing),
      pageOverflow: count((c) => c.pageOverflow),
      clipped: count((c) => c.clipped?.length),
      offBrand: count((c) => c.offBrand),
      withConsoleErrors: count((c) => c.consoleErrors?.length),
    };
    console.log(JSON.stringify(summary));
    if (summary.missing || summary.pageOverflow || summary.clipped || summary.offBrand) process.exitCode = 1;
  } finally {
    await browser?.close();
    web?.close();
    for (const child of children) child.kill();
    rmSync(tmp, { recursive: true, force: true });
  }
}

await main();
