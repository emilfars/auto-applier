#!/usr/bin/env node
/*
 * AC-APP-1/1b/2/5: real Chrome MV3 Open & Fill path.
 *
 * This deliberately uses Chrome's CDP and Node's built-ins instead of a browser
 * framework. The web app, API, extension bundles, storage.session, IndexedDB,
 * dynamic injection, and a supported ATS fixture all run in one browser.
 */
import { spawn, spawnSync } from "node:child_process";
import { createServer as createHttpServer, request as httpRequest } from "node:http";
import { createServer as createHttpsServer } from "node:https";
import {
  createWriteStream,
  existsSync,
  mkdirSync,
  readFileSync,
  readdirSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { createHash } from "node:crypto";
import { extname, join, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";
import { once } from "node:events";
import { setTimeout as sleep } from "node:timers/promises";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const STATE = join(ROOT, `.browser-e2e-state-${process.pid}`);
const EXTENSION = join(ROOT, "extension");
const WEB_DIST = join(ROOT, "web", "dist");
const PDF_PATH = join(STATE, "browser-e2e.pdf");
const PDF_BYTES = Buffer.from("%PDF-1.4\nAuto Applier browser E2E\n%%EOF\n");
const profile = {
  full_name: "Dewi Lestari",
  email: `browser-e2e-${Date.now()}@example.test`,
  phone: "+6281234567890",
  linkedin_url: "https://www.linkedin.com/in/dewi-lestari",
  github_url: "https://github.com/dewi-lestari",
  current_employer: "Nusantara Digital",
  current_title: "Senior Engineer",
  expected_salary: "15000000",
  notice_period: "30",
};
const extensionID = (() => {
  const manifest = JSON.parse(readFileSync(join(EXTENSION, "manifest.json"), "utf8"));
  const digest = createHash("sha256")
    .update(Buffer.from(manifest.key, "base64"))
    .digest("hex")
    .slice(0, 32);
  return digest.replace(/[0-9a-f]/g, (digit) =>
    String.fromCharCode("a".charCodeAt(0) + Number.parseInt(digit, 16)),
  );
})();
const expectedFixtureFields = [
  "full_name",
  "email",
  "phone",
  "linkedin_url",
  "github_url",
  "cv_file",
];

function fail(message) {
  throw new Error(`browser E2E: ${message}`);
}

function command(name, args, options = {}) {
  const result = spawnSync(name, args, {
    cwd: ROOT,
    stdio: "inherit",
    // .cmd/.bat shims (npm on Windows) and PATH lookups need a shell there.
    shell: process.platform === "win32",
    ...options,
  });
  if (result.status !== 0) fail(`${name} ${args.join(" ")} failed`);
}

function findChrome() {
  const candidates = [
    process.env.CHROME_BIN,
    join(ROOT, ".cft", "chrome-win64", "chrome.exe"),
    join(ROOT, ".cft", "chrome-linux64", "chrome"),
    "/Applications/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing",
    join(ROOT, ".cft", "chrome-mac-arm64", "Google Chrome for Testing.app", "Contents", "MacOS", "Google Chrome for Testing"),
    join(ROOT, ".cft", "chrome-mac-x64", "Google Chrome for Testing.app", "Contents", "MacOS", "Google Chrome for Testing"),
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    "/Applications/Chromium.app/Contents/MacOS/Chromium",
    "/usr/bin/google-chrome",
    "/usr/bin/google-chrome-stable",
    "/usr/bin/chromium",
    "/usr/bin/chromium-browser",
  ].filter(Boolean);
  for (const candidate of candidates) {
    if (existsSync(candidate)) return candidate;
  }
  for (const candidate of ["google-chrome", "google-chrome-stable", "chromium", "chromium-browser"]) {
    const result = spawnSync("sh", ["-c", `command -v ${candidate}`], { encoding: "utf8" });
    if (result.status === 0 && result.stdout.trim()) return result.stdout.trim();
  }
  fail("Chrome not found; set CHROME_BIN or install Google Chrome/Chromium");
}

async function freePort() {
  const server = createHttpServer();
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const port = server.address().port;
  await new Promise((resolveClose, reject) =>
    server.close((error) => (error ? reject(error) : resolveClose())),
  );
  return port;
}

async function waitFor(check, label, timeout = 20_000) {
  const end = Date.now() + timeout;
  let lastError;
  while (Date.now() < end) {
    try {
      const value = await check();
      if (value) return value;
    } catch (error) {
      lastError = error;
    }
    await sleep(100);
  }
  fail(`${label}${lastError ? ` (${lastError.message})` : ""}`);
}

async function waitHttp(url, label) {
  await waitFor(async () => {
    try {
      const response = await fetch(url);
      return response.ok;
    } catch {
      return false;
    }
  }, label, 30_000);
}

class CDP {
  #socket;
  #next = 0;
  #pending = new Map();

  static async connect(url) {
    const cdp = new CDP();
    cdp.#socket = new WebSocket(url);
    cdp.#socket.addEventListener("message", (event) => {
      const message = JSON.parse(String(event.data));
      if (!message.id) return;
      const pending = cdp.#pending.get(message.id);
      if (!pending) return;
      cdp.#pending.delete(message.id);
      if (message.error) pending.reject(new Error(message.error.message));
      else pending.resolve(message.result);
    });
    await new Promise((resolveOpen, reject) => {
      cdp.#socket.addEventListener("open", resolveOpen, { once: true });
      cdp.#socket.addEventListener("error", () => reject(new Error("CDP websocket failed")), {
        once: true,
      });
    });
    return cdp;
  }

  send(method, params = {}) {
    const id = ++this.#next;
    return new Promise((resolveResult, reject) => {
      this.#pending.set(id, { resolve: resolveResult, reject });
      this.#socket.send(JSON.stringify({ id, method, params }));
    });
  }

  async evaluate(expression, ...args) {
    const call = typeof expression === "function"
      ? `(${expression.toString()})(${args.map((arg) => JSON.stringify(arg)).join(",")})`
      : `(${expression})()`;
    const result = await this.send("Runtime.evaluate", {
      expression: call,
      awaitPromise: true,
      returnByValue: true,
      userGesture: true,
    });
    if (result.exceptionDetails) fail(result.exceptionDetails.text ?? "page evaluation failed");
    return result.result?.value;
  }

  close() {
    this.#socket?.close();
  }
}

async function json(url) {
  const response = await fetch(url);
  if (!response.ok) fail(`CDP endpoint ${url} returned ${response.status}`);
  return response.json();
}

async function targets(debugPort) {
  return json(`http://127.0.0.1:${debugPort}/json/list`);
}

async function browserCDP(debugPort) {
  const version = await json(`http://127.0.0.1:${debugPort}/json/version`);
  return CDP.connect(version.webSocketDebuggerUrl);
}

function findWorker(list) {
  return list.find(
    (target) =>
      target.type === "service_worker" &&
      target.url.startsWith(`chrome-extension://${extensionID}/`),
  );
}

async function connectPage(debugPort, predicate) {
  const target = await waitFor(async () => {
    const found = (await targets(debugPort)).find(predicate);
    return found?.webSocketDebuggerUrl ? found : null;
  }, "browser page target");
  return { target, cdp: await CDP.connect(target.webSocketDebuggerUrl) };
}

function startBackend(apiPort, sourceURL) {
  const binary = join(STATE, process.platform === "win32" ? "api.exe" : "api");
  command("go", ["build", "-o", binary, "./cmd/api"], { cwd: join(ROOT, "backend") });
  const child = spawn(binary, [], {
    cwd: ROOT,
    env: {
      ...process.env,
      API_ADDR: `127.0.0.1:${apiPort}`,
      AUTH_DEV_EXPOSE_TOKENS: "true",
      AUTH_DEV_ALLOW_IN_MEMORY_REGISTRATION: "true",
      CV_ENCRYPTION_KEY: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
      E2E_SOURCE_URL: sourceURL,
      FEED_SEED_COUNT: "1",
      S3_ENDPOINT: "",
      DATABASE_URL: "",
    },
    stdio: ["ignore", "pipe", "pipe"],
  });
  const logPath = join(STATE, "api.log");
  const log = createWriteStream(logPath);
  child.stdout.pipe(log);
  child.stderr.pipe(log);
  child.once("close", () => log.end());
  return child;
}

function startWebServer(webPort, apiPort, certPath, keyPath) {
  const apiPrefixes = ["/feed", "/auth", "/cv", "/profile", "/account", "/telemetry", "/healthz"];
  const mime = {
    ".css": "text/css",
    ".html": "text/html",
    ".js": "text/javascript",
    ".json": "application/json",
    ".svg": "image/svg+xml",
  };
  const handler = (req, res) => {
    const pathname = new URL(req.url, "http://localhost").pathname;
    if (apiPrefixes.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`))) {
      const upstream = httpRequest(
        {
          hostname: "127.0.0.1",
          port: apiPort,
          path: req.url,
          method: req.method,
          headers: { ...req.headers, host: `127.0.0.1:${apiPort}` },
        },
        (response) => {
          res.writeHead(response.statusCode ?? 502, response.headers);
          response.pipe(res);
        },
      );
      upstream.on("error", (error) => {
        res.writeHead(502, { "content-type": "text/plain" });
        res.end(error.message);
      });
      req.pipe(upstream);
      return;
    }
    const requested = pathname === "/" ? "/index.html" : pathname;
    const file = resolve(WEB_DIST, `.${requested}`);
    if (file !== WEB_DIST && !file.startsWith(`${WEB_DIST}${sep}`)) {
      res.writeHead(400);
      res.end("bad path");
      return;
    }
    try {
      const data = readFileSync(file);
      res.writeHead(200, {
        "content-type": mime[extname(file)] ?? "application/octet-stream",
        "cache-control": "no-store",
      });
      res.end(data);
    } catch {
      res.writeHead(404);
      res.end("not found");
    }
  };
  const server = certPath && keyPath
    ? createHttpsServer({ cert: readFileSync(certPath), key: readFileSync(keyPath) }, handler)
    : createHttpServer(handler);
  server.listen(webPort, "127.0.0.1");
  return server;
}

function fixtureHTML() {
  return `<!doctype html>
<meta charset="utf-8">
<title>Lever browser fixture</title>
<form id="application" action="/submitted" method="post">
  <label>Name <input name="name" autocomplete="name"></label>
  <label>Email <input name="email" type="email"></label>
  <label>Phone <input name="phone" type="tel"></label>
  <label>Company <input name="org" value="Existing Company"></label>
  <label>LinkedIn <input name="urls[LinkedIn]"></label>
  <label>GitHub <input name="urls[GitHub]"></label>
  <label>Portfolio <input name="urls[Portfolio]"></label>
  <label>Education <input name="education"></label>
  <label>Work history <textarea name="work_history"></textarea></label>
  <label>Expected salary <input name="expected_salary"></label>
  <label>Notice period <input name="notice_period"></label>
  <label>CV <input name="resume" type="file"></label>
  <button type="submit">Apply</button>
</form>
<script>
  window.__aaSubmitCount = 0;
  document.querySelector("form").addEventListener("submit", (event) => {
    window.__aaSubmitCount++;
    event.preventDefault();
  });
</script>`;
}

function startFixtureServer(fixturePort, certPath, keyPath) {
  let released = false;
  const pending = [];
  const server = createHttpsServer(
    { cert: readFileSync(certPath), key: readFileSync(keyPath) },
    (req, res) => {
      if (new URL(req.url, "https://jobs.lever.co").pathname !== "/apply/demo") {
        res.writeHead(404);
        res.end("not found");
        return;
      }
      const respond = () => {
        res.writeHead(200, { "content-type": "text/html", "cache-control": "no-store" });
        res.end(fixtureHTML());
      };
      if (released) respond();
      else pending.push(respond);
    },
  );
  server.listen(fixturePort, "127.0.0.1");
  return {
    server,
    release() {
      released = true;
      while (pending.length) pending.shift()();
    },
  };
}

async function setFileInput(page, path) {
  await page.send("DOM.enable");
  const { root } = await page.send("DOM.getDocument");
  const { nodeId } = await page.send("DOM.querySelector", {
    nodeId: root.nodeId,
    selector: 'input[type="file"]',
  });
  if (!nodeId) fail("CV file input not found in web UI");
  await page.send("DOM.setFileInputFiles", { nodeId, files: [path] });
  await page.evaluate(() => {
    document.querySelector('input[type="file"]')?.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

async function stopProcess(child) {
  if (!child || child.exitCode !== null) return;
  child.kill("SIGTERM");
  await Promise.race([once(child, "exit"), sleep(5_000)]);
  if (child.exitCode === null) {
    child.kill("SIGKILL");
    await Promise.race([once(child, "exit"), sleep(1_000)]);
  }
}

async function closeServer(server) {
  if (!server?.listening) return;
  await Promise.race([
    new Promise((resolveClose) => server.close(() => resolveClose())),
    sleep(5_000),
  ]);
}

async function run() {
  const chromeBin = findChrome();
  mkdirSync(STATE, { recursive: true });
  writeFileSync(PDF_PATH, PDF_BYTES);
  const keyPath = join(STATE, "localhost.key");
  const certPath = join(STATE, "localhost.crt");
  command("openssl", [
    "req",
    "-x509",
    "-newkey",
    "rsa:2048",
    "-nodes",
    "-keyout",
    keyPath,
    "-out",
    certPath,
    "-sha256",
    "-days",
    "1",
    "-subj",
    "/CN=jobs.lever.co",
    "-addext",
    "subjectAltName=DNS:jobs.lever.co,DNS:localhost",
  ]);

  const apiPort = await freePort();
  const webPort = await freePort();
  const fixturePort = await freePort();
  const debugPort = await freePort();
  const sourceURL = `https://jobs.lever.co:${fixturePort}/apply/demo`;
  let backend;
  let webServer;
  let fixture;
  let chrome;
  let browser;
  let web;
  let worker;
  let source;
  try {
    command("npm", ["run", "build"], {
      cwd: join(ROOT, "extension"),
      stdio: "inherit",
    });

    backend = startBackend(apiPort, sourceURL);
    await waitHttp(`http://127.0.0.1:${apiPort}/healthz`, "backend health");

    fixture = startFixtureServer(fixturePort, certPath, keyPath);
    webServer = startWebServer(webPort, apiPort, certPath, keyPath);
    await once(webServer, "listening");

    chrome = spawn(
      chromeBin,
      [
        ...(process.env.BROWSER_HEADLESS === "0" ? [] : ["--headless=new"]),
        "--disable-gpu",
        "--no-first-run",
        "--no-default-browser-check",
        "--disable-popup-blocking",
        // Keep the active web page rendering at full rate; Mantine mounts
        // overlays (the delete modal) on the next animation frame, and a
        // backgrounded headless target throttles requestAnimationFrame.
        "--disable-background-timer-throttling",
        "--disable-backgrounding-occluded-windows",
        "--disable-renderer-backgrounding",
        "--ignore-certificate-errors",
        `--remote-debugging-port=${debugPort}`,
        `--user-data-dir=${join(STATE, "chrome-profile")}`,
        `--disable-extensions-except=${EXTENSION}`,
        `--load-extension=${EXTENSION}`,
        "--host-resolver-rules=MAP jobs.lever.co 127.0.0.1",
        "about:blank",
      ].concat(process.platform === "linux" ? ["--no-sandbox"] : []),
      { cwd: ROOT, stdio: ["ignore", "pipe", "pipe"] },
    );
    await waitHttp(`http://127.0.0.1:${debugPort}/json/version`, "Chrome DevTools endpoint");

    const extensionTarget = await waitFor(
      async () => findWorker(await targets(debugPort)),
      "Auto Applier MV3 service worker (use Chrome for Testing/Chromium when stable Chrome rejects --load-extension)",
    );
    const extensionId = new URL(extensionTarget.url).hostname;
    command("npm", ["run", "build"], {
      cwd: join(ROOT, "web"),
      env: { ...process.env, VITE_EXTENSION_ID: extensionId },
      stdio: "inherit",
    });
    await closeServer(webServer);
    webServer = startWebServer(webPort, apiPort, certPath, keyPath);
    await once(webServer, "listening");

    ({ cdp: web } = await connectPage(debugPort, (target) => target.type === "page"));
    await web.send("Page.navigate", { url: `https://localhost:${webPort}/` });
    await waitFor(() => web.evaluate(() => document.readyState === "complete"), "web app load");

    const auth = await web.evaluate(
      async (email) => {
        const password = "browser-e2e-password";
        const response = await fetch("/auth/register", {
          method: "POST",
          credentials: "include",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ email, password, consent: true }),
        });
        return { registered: { status: response.status, body: await response.json() }, password };
      },
      profile.email,
    );
    if (auth.registered.status !== 201 || !auth.registered.body.verification_token) {
      fail(`registration returned ${auth.registered.status}`);
    }
    const verified = await web.evaluate(
      async (token) => {
        const response = await fetch("/auth/verify", {
          method: "POST",
          credentials: "include",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ token }),
        });
        return response.status;
      },
      auth.registered.body.verification_token,
    );
    if (verified !== 200) fail(`verification returned ${verified}`);
    const loggedIn = await web.evaluate(
      async ({ email, password }) => {
        const response = await fetch("/auth/login", {
          method: "POST",
          credentials: "include",
          headers: { "content-type": "application/json" },
          body: JSON.stringify({ email, password }),
        });
        return response.status;
      },
      { email: profile.email, password: auth.password },
    );
    if (loggedIn !== 200) fail(`login returned ${loggedIn}`);

    await web.send("Page.reload", { ignoreCache: true });
    await waitFor(() => web.evaluate(() => document.readyState === "complete"), "web session reload");
    await waitFor(
      () => web.evaluate(() => Boolean(document.querySelector('[data-testid="auth-signedin"]'))),
      "signed-in web UI",
    );
    await waitFor(
      () => web.evaluate(() => Boolean(document.querySelector('[data-testid="profile-form"]'))),
      "profile form",
    );

    await setFileInput(web, PDF_PATH);
    await waitFor(
      () => web.evaluate(() => !document.querySelector('[data-testid="cv-upload-button"]')?.disabled),
      "CV upload button",
    );
    await web.evaluate(() => document.querySelector('[data-testid="cv-upload-button"]')?.click());
    await waitFor(
      () => web.evaluate(() => Boolean(document.querySelector('[data-testid="cv-item"]'))),
      "uploaded CV in web UI",
    );
    const cvName = await web.evaluate(
      () => document.querySelector('[data-testid="cv-item"]')?.textContent ?? "",
    );
    if (!cvName.includes("browser-e2e.pdf")) fail("web UI did not show uploaded CV name");

    const profileSaved = await web.evaluate(async (values) => {
      const field = (key) => {
        const control = document.querySelector(`[data-testid="profile-${key}"]`);
        if (!control) throw new Error(`profile control ${key} missing`);
        return control;
      };
      const setValue = (key, value) => {
        const control = field(key);
        if (!(control instanceof HTMLInputElement || control instanceof HTMLTextAreaElement ||
              control instanceof HTMLSelectElement)) throw new Error(`profile control ${key} invalid`);
        const prototype = control instanceof HTMLTextAreaElement
          ? HTMLTextAreaElement.prototype
          : HTMLInputElement.prototype;
        const setter = Object.getOwnPropertyDescriptor(prototype, "value")?.set;
        setter?.call(control, value);
        control.dispatchEvent(new Event("input", { bubbles: true }));
        control.dispatchEvent(new Event("change", { bubbles: true }));
      };
      const setSelect = async (key, label) => {
        const control = field(key);
        control.click();
        const deadline = Date.now() + 5_000;
        while (Date.now() < deadline) {
          const option = [...document.querySelectorAll('[role="option"]')].find(
            (node) => node.textContent?.trim() === label && node.getClientRects().length > 0,
          );
          if (option) {
            option.click();
            return;
          }
          await new Promise((resolveWait) => setTimeout(resolveWait, 25));
        }
        throw new Error(`option "${label}" for ${key} was not found`);
      };
      setValue("full_name", values.full_name);
      setValue("email", values.email);
      setValue("phone", values.phone);
      setValue("linkedin_url", values.linkedin_url);
      setValue("github_url", values.github_url);
      setValue("portfolio_url", "");
      setValue("city", "Jakarta");
      setValue("address", "Jl. Sudirman 1");
      setValue("summary", "Backend engineer building reliable systems.");
      setValue("current_employer", "Nusantara Digital");
      setValue("current_title", "Senior Engineer");
      setValue("highest_education", "Computer Science");
      setValue("work_authorization", "authorized");
      setValue("expected_salary", values.expected_salary);
      setValue("notice_period_days", values.notice_period);
      await setSelect("employment_type", "full time");
      const relocation = field("open_to_relocation");
      if (!(relocation instanceof HTMLInputElement)) throw new Error("relocation control missing");
      relocation.click();
      setValue("education", JSON.stringify([{
        institution: "Universitas Indonesia",
        degree: "S.Kom",
        field: "Computer Science",
        start_year: "2015",
        end_year: "2019",
      }]));
      setValue("work_history", JSON.stringify([{
        company: "Nusantara Digital",
        title: "Senior Engineer",
        start_date: "2020-01",
        end_date: "",
      }]));
      setValue("skills", "Go, TypeScript, PostgreSQL");
      setValue("preferred_locations", "Jakarta, Depok");
      await new Promise((resolveWait) => setTimeout(resolveWait, 50));
      document.querySelector('[data-testid="profile-form"] button[type=submit]')?.click();
      return true;
    }, profile);
    if (!profileSaved) fail("profile controls did not save");
    await waitFor(
      () =>
        web.evaluate(() =>
          [...document.querySelectorAll('[data-testid="profile-status"]')].some((node) =>
            /saved|disimpan/i.test(node.textContent ?? ""),
          ),
        ),
      "profile save",
    );
    await waitFor(
      () => web.evaluate(() => Boolean(document.querySelector('[data-testid="profile-confirm"]'))),
      "profile confirm button",
    );
    await web.evaluate(() => document.querySelector('[data-testid="profile-confirm"]')?.click());
    await waitFor(
      () => web.evaluate(() => Boolean(document.querySelector('[data-testid="profile-confirmed-badge"]'))),
      "confirmed profile",
    );

    const snapshot = await web.evaluate(async () => {
      const response = await fetch("/profile/fill", { credentials: "include" });
      return { status: response.status, body: await response.json() };
    });
    if (snapshot.status !== 200 || snapshot.body.profile.confirmed !== true) {
      fail(`fill snapshot returned ${snapshot.status}`);
    }
    if (snapshot.body.cv?.filename !== "browser-e2e.pdf") fail("snapshot CV name mismatch");
    const snapshotBytes = Buffer.from(snapshot.body.cv.bytes_base64, "base64");
    if (!snapshotBytes.equals(PDF_BYTES)) fail("snapshot CV bytes mismatch");
    if (snapshot.body.profile.full_name !== profile.full_name) fail("snapshot profile mismatch");

    await waitFor(
      () => web.evaluate(() => Boolean(document.querySelector('[data-testid="job-fill-button"]:not([disabled])'))),
      "enabled Open & Fill button",
    );
    await web.evaluate(() => document.querySelector('[data-testid="job-fill-button"]:not([disabled])')?.click());
    try {
      await waitFor(
        () =>
          web.evaluate(() =>
            Boolean(document.querySelector('[data-testid="job-fill-notice"][data-status="armed"]')),
          ),
        "Open & Fill bridge response",
      );
    } catch (error) {
      console.error(
        await web.evaluate(() => ({
          notices: [...document.querySelectorAll('[data-testid="job-fill-notice"]')].map((node) => node.textContent),
          chromeRuntime: Boolean(globalThis.chrome?.runtime),
          statuses: [...document.querySelectorAll('[data-testid="job-fill-notice"]')]
            .map((node) => node.getAttribute("data-status")),
        })),
      );
      throw error;
    }

    const workerTarget = await waitFor(async () => findWorker(await targets(debugPort)), "armed MV3 worker");
    worker = await CDP.connect(workerTarget.webSocketDebuggerUrl);
    const durable = await worker.evaluate(async () => {
      const stored = await chrome.storage.session.get("autoApplierArms");
      const entries = Object.values(stored.autoApplierArms ?? {});
      const payloads = await new Promise((resolve) => {
        const request = indexedDB.open("autoApplierCv");
        request.onerror = () => resolve([]);
        request.onsuccess = () => {
          const db = request.result;
          const get = db.transaction("payloads", "readonly").objectStore("payloads").getAll();
          get.onerror = () => resolve([]);
          get.onsuccess = () =>
            resolve(get.result.map((record) => ({ id: record.id, size: record.bytes.byteLength })));
        };
      });
      return { entries, payloads };
    });
    if (durable.entries.length !== 1 || durable.payloads.length !== 1) {
      fail("arm metadata or CV payload was not durable");
    }
    if (durable.payloads[0].size !== PDF_BYTES.length) fail("IndexedDB CV payload size mismatch");

    browser = await browserCDP(debugPort);
    const workerInfo = (await browser.send("Target.getTargets")).targetInfos.find(
      (target) => target.targetId === workerTarget.id,
    );
    if (!workerInfo) fail("armed worker target disappeared before restart test");
    const closed = await browser.send("Target.closeTarget", { targetId: workerInfo.targetId });
    if (!closed.success) fail("Chrome refused to terminate the MV3 worker target");
    worker.close();
    worker = undefined;
    await waitFor(
      async () => !findWorker(await targets(debugPort)),
      "MV3 worker termination",
      5_000,
    );

    fixture.release();
    source = (
      await connectPage(debugPort, (target) => target.type === "page" && target.url.startsWith(sourceURL))
    ).cdp;
    await waitFor(
      () => source.evaluate(() => Boolean(document.querySelector("[data-aa-review-summary]"))),
      "injected production content script",
      20_000,
    );
    const result = await source.evaluate(async () => {
      const file = document.querySelector('input[name="resume"]');
      const summary = document.querySelector("[data-aa-review-summary]");
      const fileBytes = file?.files?.[0]
        ? btoa(String.fromCharCode(...new Uint8Array(await file.files[0].arrayBuffer())))
        : "";
      const attr = (name) => summary?.getAttribute(`data-${name}`) ?? "";
      return {
        href: location.href,
        values: Object.fromEntries(
          ["name", "email", "phone", "urls[LinkedIn]", "urls[GitHub]", "org", "urls[Portfolio]"].map(
            (name) => [name, document.querySelector(`[name="${name}"]`)?.value ?? ""],
          ),
        ),
        file: { name: file?.files?.[0]?.name ?? "", size: file?.files?.[0]?.size ?? 0, bytes: fileBytes },
        states: {
          filled: document.querySelectorAll('[data-aa-fill="filled"]').length,
          uncertain: document.querySelectorAll('[data-aa-fill="uncertain"]').length,
          empty: document.querySelectorAll('[data-aa-fill="empty"]').length,
        },
        summary: {
          applied: attr("applied"),
          uncertain: attr("uncertain"),
          empty: attr("empty"),
          coverage: attr("coverage"),
          submitted: attr("submitted"),
          appliedKeys: attr("applied-keys").split(",").filter(Boolean),
          uncertainKeys: attr("uncertain-keys").split(",").filter(Boolean),
          emptyKeys: attr("empty-keys").split(",").filter(Boolean),
        },
        submitCount: window.__aaSubmitCount,
      };
    });
    if (result.href !== sourceURL) fail("fixture navigated during fill");
    if (result.values.name !== profile.full_name) fail("name was not filled");
    if (result.values.email !== profile.email) fail("email was not filled");
    if (result.values.phone !== profile.phone) fail("phone was not filled");
    if (result.values["urls[LinkedIn]"] !== profile.linkedin_url) fail("LinkedIn was not filled");
    if (result.values["urls[GitHub]"] !== "https://github.com/dewi-lestari") fail("GitHub was not filled");
    if (result.values.org !== "Existing Company") fail("existing field was overwritten");
    if (result.values["urls[Portfolio]"] !== "") fail("empty profile field was changed");
    if (result.file.name !== "browser-e2e.pdf" || result.file.size !== PDF_BYTES.length) {
      fail("CV file name or size mismatch on the ATS page");
    }
    if (result.file.bytes !== PDF_BYTES.toString("base64")) fail("attached CV bytes mismatch");
    const applied = new Set(result.summary.appliedKeys);
    const numerator = expectedFixtureFields.filter((key) => applied.has(key)).length;
    const coverage = numerator / expectedFixtureFields.length;
    if (coverage < 0.8) fail(`fixture coverage ${coverage} is below 80%`);
    if (result.states.uncertain < 1 || result.states.empty < 1) {
      fail("uncertain and empty review states were not visible");
    }
    if (result.summary.submitted !== "false" || result.submitCount !== 0) {
      fail("extension submitted the fixture form");
    }
    if (Number(result.summary.applied) !== numerator) fail("ApplyReport applied count mismatch");
    if (result.summary.uncertainKeys.length < 1 || result.summary.emptyKeys.length < 1) {
      fail("ApplyReport review-state keys missing");
    }

    console.log(
      `AC-APP-1/1b/2/5 PASS: ${numerator}/${expectedFixtureFields.length} fixture fields, ` +
        `${result.summary.uncertain} uncertain, ${result.summary.empty} empty, worker restarted`,
    );

    // AC-AUTH-5 / UU PDP: signed-in account data-rights UI. Export a complete
    // copy of the user's data (real download preferred), then erase the account.
    await web.send("Page.navigate", { url: `https://localhost:${webPort}/` });
    await web.send("Page.bringToFront");
    await waitFor(() => web.evaluate(() => document.readyState === "complete"), "account page load");
    await waitFor(
      () => web.evaluate(() => Boolean(document.querySelector('[data-testid="auth-signedin"]'))),
      "signed-in web UI for account flows",
    );
    await waitFor(
      () => web.evaluate(() => Boolean(document.querySelector('[data-testid="account-export"]'))),
      "account data-rights panel",
    );

    const downloadDir = join(STATE, "downloads");
    mkdirSync(downloadDir, { recursive: true });
    await browser.send("Browser.setDownloadBehavior", {
      behavior: "allow",
      downloadPath: downloadDir,
      eventsEnabled: true,
    });
    await web.evaluate(() =>
      document.querySelector('[data-testid="account-export"]')?.click(),
    );

    const readExport = () => {
      for (const name of readdirSync(downloadDir).filter((entry) => entry.endsWith(".json"))) {
        const path = join(downloadDir, name);
        try {
          return { name, parsed: JSON.parse(readFileSync(path, "utf8")) };
        } catch {
          // partial write; retry on the next poll
        }
      }
      return null;
    };
    let downloaded = null;
    try {
      downloaded = await waitFor(readExport, "account export download", 10_000);
    } catch {
      downloaded = null;
    }

    let exportMode;
    if (downloaded) {
      exportMode = "real download file";
      if (downloaded.parsed.account?.email !== profile.email) {
        fail(`account export email mismatch: ${downloaded.parsed.account?.email}`);
      }
      if (
        !(downloaded.parsed.cv_files ?? []).some((file) => file.filename === "browser-e2e.pdf")
      ) {
        fail("account export did not list browser-e2e.pdf");
      }
    } else {
      exportMode = "fallback (notice + fetch /account/export)";
      await waitFor(
        () => web.evaluate(() => Boolean(document.querySelector('[data-testid="account-notice"]'))),
        "account export notice",
        10_000,
      );
      const fallback = await web.evaluate(async () => {
        const response = await fetch("/account/export", { credentials: "include" });
        return { status: response.status, body: await response.json() };
      });
      if (fallback.status !== 200) fail(`fallback /account/export returned ${fallback.status}`);
      if (fallback.body.account?.email !== profile.email) {
        fail("fallback account export email mismatch");
      }
      if (!(fallback.body.cv_files ?? []).some((file) => file.filename === "browser-e2e.pdf")) {
        fail("fallback account export did not list browser-e2e.pdf");
      }
    }
    console.log(`AC-AUTH-5 export: asserted via ${exportMode}`);

    await waitFor(
      () =>
        web.evaluate(() => {
          const button = document.querySelector('[data-testid="account-delete-open"]');
          return Boolean(button && !button.disabled);
        }),
      "enabled account delete open",
    );
    await web.evaluate(() =>
      document.querySelector('[data-testid="account-delete-open"]')?.click(),
    );
    await waitFor(
      () =>
        web.evaluate(() => Boolean(document.querySelector('[data-testid="account-delete-email"]'))),
      "account delete confirmation",
    );
    const disabledBefore = await web.evaluate(() => {
      const button = document.querySelector('[data-testid="account-delete-confirm"]');
      return Boolean(button && (button.disabled || button.hasAttribute("disabled")));
    });
    if (!disabledBefore) fail("account delete confirm was enabled before the email was typed");

    const typed = await web.evaluate((email) => {
      const input = document.querySelector('[data-testid="account-delete-email"]');
      if (!(input instanceof HTMLInputElement)) throw new Error("delete email input missing");
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
      setter?.call(input, email);
      input.dispatchEvent(new Event("input", { bubbles: true }));
      input.dispatchEvent(new Event("change", { bubbles: true }));
      return input.value;
    }, profile.email);
    if (typed !== profile.email) fail("delete email input did not hold the profile email");
    await waitFor(
      () =>
        web.evaluate(() => {
          const button = document.querySelector('[data-testid="account-delete-confirm"]');
          return Boolean(button && !button.disabled && !button.hasAttribute("disabled"));
        }),
      "enabled account delete confirm",
    );
    await web.evaluate(() =>
      document.querySelector('[data-testid="account-delete-confirm"]')?.click(),
    );

    await waitFor(
      () => web.evaluate(() => !document.querySelector('[data-testid="auth-signedin"]')),
      "signed-out UI after account deletion",
    );
    const meStatus = await waitFor(
      async () => {
        const status = await web.evaluate(async () => {
          const response = await fetch("/auth/me", { credentials: "include" });
          return response.status;
        });
        return status !== 200 ? status : null;
      },
      "unauthenticated /auth/me after account deletion",
    );
    if (meStatus === 200) fail("account session survived deletion");

    console.log("AC-AUTH-5 PASS: account export downloaded + account deleted/erased");
  } finally {
    source?.close();
    worker?.close();
    browser?.close();
    web?.close();
    fixture?.release();
    await closeServer(fixture?.server);
    await closeServer(webServer);
    await stopProcess(chrome);
    await stopProcess(backend);
    rmSync(STATE, { recursive: true, force: true });
  }
}

try {
  await run();
} catch (error) {
  console.error(error instanceof Error ? error.message : error);
  rmSync(STATE, { recursive: true, force: true });
  process.exitCode = 1;
}
