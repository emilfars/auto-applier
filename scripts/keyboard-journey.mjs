#!/usr/bin/env node
// E-A11Y keyboard run (plan-revamp.md section 5): drive the web journey with the
// keyboard only (Tab, Shift+Tab, Enter, Space, typing). No mouse, no element
// clicks. Asserts that focus is always visible and that each step lands.
//
// Usage: PW_CORE=/dir/with/node_modules node scripts/keyboard-journey.mjs <base-url>
// Start the stack with `node scripts/ui-matrix.mjs --serve` and pass its URL.
// Not covered here: choosing a CV file (an OS file dialog) and the extension
// hand-off; scripts/browser-e2e.mjs covers those with real Chrome.
import { createRequire } from "node:module";
import { existsSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = fileURLToPath(new URL("..", import.meta.url));
const base = process.argv[2];
if (!base) {
  console.error("usage: node scripts/keyboard-journey.mjs <base-url>");
  process.exit(2);
}
const { chromium } = createRequire(join(process.env.PW_CORE ?? ROOT, "noop.js"))("playwright-core");
const chrome = [
  process.env.CHROME_BIN,
  join(ROOT, ".cft/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing"),
  join(ROOT, ".cft/chrome-linux64/chrome"),
].find((c) => c && existsSync(c));

const browser = await chromium.launch({ executablePath: chrome, headless: true });
const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
const steps = [];
const step = (name) => {
  steps.push(name);
  console.log(`ok  ${name}`);
};

/** Tab until the focused element matches, checking every stop shows a focus ring. */
async function tabTo(selector, { max = 80, back = false } = {}) {
  for (let i = 0; i < max; i++) {
    await page.keyboard.press(back ? "Shift+Tab" : "Tab");
    const state = await page.evaluate((sel) => {
      const el = document.activeElement;
      if (!el || el === document.body) return { match: false, visible: true, tag: "body" };
      const hasRing = (node) => {
        const style = getComputedStyle(node);
        return style.outlineStyle !== "none" && parseFloat(style.outlineWidth) > 0;
      };
      // A password field's ring is drawn on the wrapper that holds the input and its toggle.
      const wrapper = el.closest(".mantine-Input-input");
      const ring = hasRing(el) || (!!wrapper && hasRing(wrapper));
      // Radio inputs of a segmented control are visually hidden; their label carries the ring.
      const label = el.matches('input[type="radio"]') ? el.nextElementSibling : null;
      const labelRing = !!label && hasRing(label);
      return { match: el.matches(sel), visible: ring || labelRing, tag: el.outerHTML.slice(0, 90) };
    }, selector);
    if (!state.visible) throw new Error(`focus not visible on ${state.tag}`);
    if (state.match) return;
  }
  throw new Error(`could not reach ${selector} with the keyboard`);
}

try {
  const email = `keyboard-${Date.now()}@example.test`;
  await page.goto(`${base}/#account`);
  await page.waitForSelector('[data-testid="auth-mode-signup"]');

  await tabTo('[data-testid="auth-mode-signup"]');
  await page.keyboard.press("Enter");
  await tabTo('[data-testid="auth-email"]');
  await page.keyboard.type(email);
  await tabTo('[data-testid="auth-password"]');
  await page.keyboard.type("keyboard-password-1");
  await tabTo('[data-testid="auth-consent"]');
  await page.keyboard.press("Space");
  await tabTo('[data-testid="auth-submit"]');
  await page.keyboard.press("Enter");
  await page.waitForFunction(() => document.querySelector('[data-testid="auth-verify-token"]')?.value);
  step("register with keyboard");

  await tabTo('[data-testid="auth-verify-submit"]');
  await page.keyboard.press("Enter");
  await page.waitForSelector('[data-testid="auth-verify-token"]', { state: "detached" });
  step("verify with keyboard");

  await tabTo('[data-testid="auth-email"]', { back: true });
  await page.keyboard.press("ControlOrMeta+A");
  await page.keyboard.type(email);
  await tabTo('[data-testid="auth-password"]');
  await page.keyboard.press("ControlOrMeta+A");
  await page.keyboard.type("keyboard-password-1");
  await page.keyboard.press("Enter");
  await page.waitForSelector('[data-testid="auth-signedin"]');
  step("sign in with keyboard (Enter submits the form)");

  await tabTo('a[href="#profile"]', { back: true });
  await page.keyboard.press("Enter");
  await page.waitForFunction(() => location.hash === "#profile");
  await tabTo('[data-testid="profile-full_name"]');
  await page.keyboard.type("Dewi Lestari");
  await tabTo('[data-testid="profile-education-add"]');
  await page.keyboard.press("Enter");
  await tabTo('[data-testid="profile-education-0-institution"]', { back: true });
  await page.keyboard.type("Universitas Indonesia");
  step("reach the profile view, type into fields and add an education row with keyboard");

  await tabTo('a[href="#feed"]', { back: true, max: 120 });
  await page.keyboard.press("Enter");
  await page.waitForFunction(() => location.hash === "#feed");
  await tabTo('#feed input[type="search"]');
  await page.keyboard.type("zzz-no-such-listing");
  await page.waitForFunction(() => document.querySelectorAll('[data-testid="job-fill-button"]').length === 0 && !document.querySelector('#feed [aria-busy="true"]'));
  await page.keyboard.press("ControlOrMeta+A");
  await page.keyboard.press("Backspace");
  await page.waitForSelector('[data-testid="job-fill-button"]');
  step("filter the feed with keyboard");

  await tabTo('[data-testid="job-fill-button"]', { max: 60 });
  await page.keyboard.press("Enter");
  await page.waitForSelector('[data-testid="job-fill-notice"]');
  const status = await page.getAttribute('[data-testid="job-fill-notice"]', "data-status");
  if (status !== "needProfile") throw new Error(`expected the confirm-profile notice, got ${status}`);
  step("activate Open & Fill with keyboard (unconfirmed profile is refused with a notice)");

  await tabTo('a[href="#account"]', { back: true, max: 120 });
  await page.keyboard.press("Enter");
  await tabTo('[data-testid="account-delete-open"]');
  await page.keyboard.press("Enter");
  await page.waitForSelector('[data-testid="account-delete-email"]');
  await page.keyboard.press("Escape");
  await page.waitForSelector('[data-testid="account-delete-email"]', { state: "detached" });
  step("open the delete dialog and close it with Escape");

  await tabTo('[data-testid="auth-signout"]', { back: true });
  await page.keyboard.press("Enter");
  await page.waitForSelector('[data-testid="auth-mode-signup"]');
  step("sign out with keyboard");

  console.log(`E-A11Y keyboard PASS: ${steps.length} steps, focus visible at every stop`);
} catch (error) {
  console.error(`E-A11Y keyboard FAIL after "${steps.at(-1) ?? "start"}": ${error.message}`);
  process.exitCode = 1;
} finally {
  await browser.close();
}
