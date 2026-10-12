import { describe, it, expect } from "vitest";
import { Window } from "happy-dom";
import type { ProfileData } from "@auto-applier/fill-mappings";
import { SessionArmingStore, type BinaryPayloadStore, type SessionStorageLike } from "./arming.js";
import {
  asIsArmed,
  asOpenAndFill,
  type IsArmedMessage,
  type IsArmedResult,
} from "./messaging.js";
import { renderReviewSummary, runContentFill } from "./content.js";

const profile: ProfileData = {
  confirmed: true,
  full_name: "Sri Wahyuni",
  email: "sri.wahyuni@example.com",
  phone: "081234567890",
};

const leverForm = `
  <form>
    <input name="name" type="text" />
    <input name="email" type="email" />
    <input name="phone" type="tel" />
    <input name="resume" type="file" />
  </form>`;

const TAB_ID = 1;

/**
 * Stand-in for the chrome message bus around the production SessionArmingStore:
 * arm on the web app's request, consume once from the content script, the same
 * calls background.ts makes with chrome.storage.session behind them.
 */
function makeBackground() {
  const state: Record<string, unknown> = {};
  const storage: SessionStorageLike = {
    get: async (key) => (key in state ? { [key]: state[key] } : {}),
    set: async (items) => {
      Object.assign(state, items);
    },
    remove: async (key) => {
      delete state[key];
    },
  };
  const payloads = new Map<string, Uint8Array>();
  const binary: BinaryPayloadStore = {
    put: async (bytes) => {
      const id = `payload-${payloads.size}`;
      payloads.set(id, bytes.slice());
      return id;
    },
    get: async (id) => payloads.get(id) ?? null,
    delete: async (id) => {
      payloads.delete(id);
    },
    sweep: async () => undefined,
  };
  const store = new SessionArmingStore(storage, undefined, binary);
  const openedTabs: string[] = [];

  const fromWebApp = async (message: unknown) => {
    const req = asOpenAndFill(message);
    if (!req) throw new Error("bad openAndFill");
    await store.arm(req.url, Date.now(), TAB_ID, req.snapshot);
    openedTabs.push(req.url);
    return { armed: true };
  };

  const fromContent = async (msg: IsArmedMessage): Promise<IsArmedResult> => {
    const req = asIsArmed(msg);
    if (!req) throw new Error("bad isArmed");
    const entry = await store.consume(req.url, Date.now(), TAB_ID);
    return entry?.snapshot ? { armed: true, snapshot: entry.snapshot } : { armed: entry !== null };
  };

  return { store, openedTabs, fromWebApp, fromContent };
}

describe("Open & Fill end-to-end (AC-APP-5)", () => {
  it("arms from the feed, opens the source page, and fills once loaded", async () => {
    const bg = makeBackground();
    const sourceUrl = "https://jobs.lever.co/acme/1/apply";

    // 1) Feed "Open & Fill" → background arms + opens the source page.
    const armed = await bg.fromWebApp({ type: "openAndFill", url: sourceUrl });
    expect(armed).toEqual({ armed: true });
    expect(bg.openedTabs).toEqual([sourceUrl]);

    // 2) Source page loads; its content script runs the fill.
    const win = new Window({ url: sourceUrl });
    win.document.body.innerHTML = leverForm;
    const doc = win.document as unknown as Document;

    const result = await runContentFill(
      bg.fromContent,
      doc,
      { hostname: "jobs.lever.co", href: sourceUrl },
      async () => ({ profile }),
    );

    expect(result.ran).toBe(true);
    if (result.ran) {
      expect(result.report.submitted).toBe(false);
      expect(result.report.applied).toBeGreaterThan(0);
      renderReviewSummary(doc, result.report);
      expect(doc.querySelector("[data-aa-review-summary]")?.textContent).toContain(
        "filled",
      );
    }
    expect((doc.querySelector('input[name="email"]') as HTMLInputElement).value).toBe(
      "sri.wahyuni@example.com",
    );
  });

  it("does not fill a page that was never armed", async () => {
    const bg = makeBackground();
    const win = new Window({ url: "https://jobs.lever.co/acme/1/apply" });
    win.document.body.innerHTML = leverForm;
    const doc = win.document as unknown as Document;

    const result = await runContentFill(
      bg.fromContent,
      doc,
      { hostname: "jobs.lever.co", href: "https://jobs.lever.co/acme/1/apply" },
      async () => ({ profile }),
    );

    expect(result).toEqual({ ran: false, reason: "not-armed" });
    expect((doc.querySelector('input[name="email"]') as HTMLInputElement).value).toBe("");
  });

  it("arm is single-use: a reload does not re-fill", async () => {
    const bg = makeBackground();
    const url = "https://jobs.lever.co/acme/1/apply";
    await bg.fromWebApp({ type: "openAndFill", url });

    const load = async () => {
      const win = new Window({ url });
      win.document.body.innerHTML = leverForm;
      const doc = win.document as unknown as Document;
      const res = await runContentFill(
        bg.fromContent,
        doc,
        { hostname: "jobs.lever.co", href: url },
        async () => ({ profile }),
      );
      return { doc, res };
    };

    const first = await load();
    expect(first.res.ran).toBe(true);

    const second = await load();
    expect(second.res).toEqual({ ran: false, reason: "not-armed" });
  });

  it("does not fill when the profile is unconfirmed (AC-CV-5 defense in depth)", async () => {
    const bg = makeBackground();
    const url = "https://jobs.lever.co/acme/1/apply";
    await bg.fromWebApp({ type: "openAndFill", url });

    const win = new Window({ url });
    win.document.body.innerHTML = leverForm;
    const doc = win.document as unknown as Document;

    const result = await runContentFill(
      bg.fromContent,
      doc,
      { hostname: "jobs.lever.co", href: url },
      async () => ({ profile: { ...profile, confirmed: false } }),
    );

    expect(result).toEqual({ ran: false, reason: "profile-unconfirmed" });
    expect((doc.querySelector('input[name="email"]') as HTMLInputElement).value).toBe("");
  });

  it("transfers the confirmed profile and uploaded CV bytes through the one-shot arm", async () => {
    const bg = makeBackground();
    const url = "https://jobs.lever.co/acme/1/apply";
    await bg.fromWebApp({
      type: "openAndFill",
      url,
      snapshot: {
        profile,
        cv: { name: "resume.pdf", type: "application/pdf", bytesBase64: "Y3YtYnl0ZXM=" },
      },
    });
    const win = new Window({ url });
    win.document.body.innerHTML = leverForm;
    const doc = win.document as unknown as Document;
    const result = await runContentFill(
      bg.fromContent,
      doc,
      { hostname: "jobs.lever.co", href: url },
      async () => {
        throw new Error("snapshot should be used");
      },
    );
    expect(result.ran).toBe(true);
    const files = (doc.querySelector('input[name="resume"]') as HTMLInputElement).files;
    expect(files?.[0]?.name).toBe("resume.pdf");
    expect(await files?.[0]?.text()).toBe("cv-bytes");
  });

  it("wires privacy-safe correction telemetry through runContentFill", async () => {
    const bg = makeBackground();
    const url = "https://jobs.lever.co/acme/1/apply";
    await bg.fromWebApp({ type: "openAndFill", url });
    const win = new Window({ url });
    win.document.body.innerHTML = leverForm;
    const doc = win.document as unknown as Document;
    const events: unknown[] = [];
    await runContentFill(
      bg.fromContent,
      doc,
      { hostname: "jobs.lever.co", href: url },
      async () => ({ profile }),
      { onCorrection: (event) => events.push(event) },
    );
    const email = doc.querySelector('input[name="email"]') as HTMLInputElement;
    email.value = "changed@example.com";
    email.dispatchEvent(new Event("input", { bubbles: true }));
    expect(events).toHaveLength(1);
    expect(events[0]).not.toHaveProperty("value");
  });
});
