import { describe, it, expect } from "vitest";
import { Window } from "happy-dom";
import type { ProfileData } from "@auto-applier/fill-mappings";
import { buildFillPlan, getMapForHost } from "@auto-applier/fill-mappings";
import { applyPlan } from "./apply.js";
import {
  watchForCorrection,
  type FillCorrectionEvent,
} from "./telemetry.js";

const profile: ProfileData = {
  confirmed: true,
  first_name: "Sri",
  last_name: "Wahyuni",
  email: "sri.wahyuni@example.com",
  phone: "081234567890",
};

const form = `
  <form id="f">
    <input id="first_name" name="first_name" type="text" />
    <input id="last_name" name="last_name" type="text" />
    <input id="email" name="email" type="email" />
    <input id="phone" name="phone" type="tel" />
  </form>`;

function makeDoc(): Document {
  const win = new Window({ url: "https://boards.greenhouse.io/acme/jobs/1" });
  win.document.body.innerHTML = form;
  return win.document as unknown as Document;
}

describe("watchForCorrection (AC-APP-TEL)", () => {
  it("emits one privacy-safe event when the user overrides a filled value", () => {
    const win = new Window({ url: "https://x" });
    win.document.body.innerHTML = `<input id="e" value="filled@x.com" />`;
    const el = win.document.querySelector("#e") as unknown as HTMLInputElement;

    const events: FillCorrectionEvent[] = [];
    watchForCorrection(
      el as unknown as EventTarget & { value?: string },
      "filled@x.com",
      { portalId: "greenhouse", version: "1.0.0", key: "email", selector: "#e" },
      (e) => events.push(e),
      () => 1234,
    );

    el.value = "user@x.com";
    el.dispatchEvent(new Event("input", { bubbles: true }));

    expect(events).toHaveLength(1);
    const ev = events[0]!;
    expect(ev.type).toBe("fill_correction");
    expect(ev.portalId).toBe("greenhouse");
    expect(ev.key).toBe("email");
    expect(ev.selector).toBe("#e");
    expect(ev.at).toBe(1234);
    // Privacy: the event must not carry the field value / PII.
    expect(JSON.stringify(ev)).not.toContain("user@x.com");
    expect(JSON.stringify(ev)).not.toContain("filled@x.com");
  });

  it("does not emit when the value is unchanged", () => {
    const win = new Window({ url: "https://x" });
    win.document.body.innerHTML = `<input id="e" value="same" />`;
    const el = win.document.querySelector("#e") as unknown as HTMLInputElement;

    const events: FillCorrectionEvent[] = [];
    watchForCorrection(
      el as unknown as EventTarget & { value?: string },
      "same",
      { portalId: "p", version: "1", key: "email" },
      (e) => events.push(e),
    );

    el.dispatchEvent(new Event("input", { bubbles: true }));
    expect(events).toHaveLength(0);
  });

  it("emits at most once even across multiple edits", () => {
    const win = new Window({ url: "https://x" });
    win.document.body.innerHTML = `<input id="e" value="v0" />`;
    const el = win.document.querySelector("#e") as unknown as HTMLInputElement;

    const events: FillCorrectionEvent[] = [];
    watchForCorrection(
      el as unknown as EventTarget & { value?: string },
      "v0",
      { portalId: "p", version: "1", key: "phone" },
      (e) => events.push(e),
    );

    el.value = "v1";
    el.dispatchEvent(new Event("input", { bubbles: true }));
    el.value = "v2";
    el.dispatchEvent(new Event("input", { bubbles: true }));

    expect(events).toHaveLength(1);
  });

  it("stops emitting after detach", () => {
    const win = new Window({ url: "https://x" });
    win.document.body.innerHTML = `<input id="e" value="v0" />`;
    const el = win.document.querySelector("#e") as unknown as HTMLInputElement;

    const events: FillCorrectionEvent[] = [];
    const detach = watchForCorrection(
      el as unknown as EventTarget & { value?: string },
      "v0",
      { portalId: "p", version: "1", key: "phone" },
      (e) => events.push(e),
    );
    detach();

    el.value = "v1";
    el.dispatchEvent(new Event("input", { bubbles: true }));
    expect(events).toHaveLength(0);
  });
});

describe("applyPlan wires correction telemetry (AC-APP-TEL)", () => {
  it("emits a correction when a user overrides an auto-filled field", () => {
    const doc = makeDoc();
    const portal = getMapForHost("boards.greenhouse.io");
    const plan = buildFillPlan(portal, profile, doc);

    const events: FillCorrectionEvent[] = [];
    applyPlan(plan, doc, { onCorrection: (e) => events.push(e) });

    // The engine filled #email; the fill's own dispatch must NOT count.
    expect(events).toHaveLength(0);

    const email = doc.querySelector("#email") as HTMLInputElement;
    email.value = "corrected@example.com";
    email.dispatchEvent(new Event("input", { bubbles: true }));

    expect(events).toHaveLength(1);
    expect(events[0]!.key).toBe("email");
  });
});
