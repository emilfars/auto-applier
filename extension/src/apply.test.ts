import { describe, it, expect, beforeEach } from "vitest";
import { Window } from "happy-dom";
import type { ProfileData } from "@auto-applier/fill-mappings";
import { applyPlan, FILL_ATTR } from "./apply.js";
import { runFill } from "./fill.js";
import { buildFillPlan, getMapForHost } from "@auto-applier/fill-mappings";

/** A confirmed profile (armed) with realistic Jabodetabek data. */
const profile: ProfileData = {
  confirmed: true,
  full_name: "Sri Wahyuni",
  first_name: "Sri",
  last_name: "Wahyuni",
  email: "sri.wahyuni@example.com",
  phone: "081234567890",
  city: "Jakarta Selatan",
  linkedin_url: "https://linkedin.com/in/sriwahyuni",
  cv_file: "cv-sri-wahyuni.pdf",
};

/** Build a fresh happy-dom window/document per test. */
function makeDoc(html: string): { doc: Document; win: Window } {
  const win = new Window({ url: "https://boards.greenhouse.io/acme/jobs/1" });
  win.document.body.innerHTML = html;
  return { doc: win.document as unknown as Document, win };
}

const greenhouseForm = `
  <form id="application_form">
    <input id="first_name" name="first_name" type="text" />
    <input id="last_name" name="last_name" type="text" />
    <input id="email" name="email" type="email" />
    <input id="phone" name="phone" type="tel" />
    <input id="resume" name="resume" type="file" />
    <button type="submit">Submit Application</button>
  </form>`;

describe("applyPlan on a greenhouse form (AC-APP-1)", () => {
  let doc: Document;
  beforeEach(() => {
    ({ doc } = makeDoc(greenhouseForm));
  });

  it("fills matched text fields from the confirmed profile", () => {
    const portal = getMapForHost("boards.greenhouse.io");
    const plan = buildFillPlan(portal, profile, doc);
    applyPlan(plan, doc, { file: new File(["cv"], "cv.pdf") });

    expect((doc.querySelector("#first_name") as HTMLInputElement).value).toBe(
      "Sri",
    );
    expect((doc.querySelector("#last_name") as HTMLInputElement).value).toBe(
      "Wahyuni",
    );
    expect((doc.querySelector("#email") as HTMLInputElement).value).toBe(
      "sri.wahyuni@example.com",
    );
  });

  it("reaches >=80% coverage of mappable fields present (AC-APP-1b)", () => {
    const portal = getMapForHost("boards.greenhouse.io");
    const plan = buildFillPlan(portal, profile, doc);
    const report = applyPlan(plan, doc, { file: new File(["cv"], "cv.pdf") });

    // Fields whose element exists on this form and had profile data.
    const present = report.fields.filter(
      (f) => f.selector && f.state !== "empty",
    );
    const appliedOk = present.filter((f) => f.applied).length;
    expect(appliedOk / present.length).toBeGreaterThanOrEqual(0.8);
  });

  it("dispatches input/change so framework-controlled inputs update", () => {
    const el = doc.querySelector("#email") as HTMLInputElement;
    const events: string[] = [];
    el.addEventListener("input", () => events.push("input"));
    el.addEventListener("change", () => events.push("change"));

    const portal = getMapForHost("boards.greenhouse.io");
    const plan = buildFillPlan(portal, profile, doc);
    applyPlan(plan, doc);

    expect(events).toContain("input");
    expect(events).toContain("change");
  });
});

describe("CV attach (AC-APP-2)", () => {
  it("attaches the CV file to the file input", () => {
    const { doc } = makeDoc(greenhouseForm);
    const portal = getMapForHost("boards.greenhouse.io");
    const plan = buildFillPlan(portal, profile, doc);
    const file = new File(["binary"], "resume.pdf", {
      type: "application/pdf",
    });
    applyPlan(plan, doc, { file });

    const input = doc.querySelector("#resume") as HTMLInputElement;
    expect(input.files?.length).toBe(1);
    expect(input.files?.[0]?.name).toBe("resume.pdf");
  });

  it("does not attach when no CV file is provided", () => {
    const { doc } = makeDoc(greenhouseForm);
    const portal = getMapForHost("boards.greenhouse.io");
    const plan = buildFillPlan(portal, profile, doc);
    applyPlan(plan, doc);
    const input = doc.querySelector("#resume") as HTMLInputElement;
    expect(input.files?.length ?? 0).toBe(0);
  });
});

describe("highlight states (AC-APP-4 review affordance)", () => {
  it("marks filled fields green and does not submit the form", () => {
    const { doc } = makeDoc(greenhouseForm);

    let submitted = false;
    const form = doc.querySelector("#application_form") as HTMLFormElement;
    form.addEventListener("submit", () => {
      submitted = true;
    });

    const portal = getMapForHost("boards.greenhouse.io");
    const plan = buildFillPlan(portal, profile, doc);
    const report = applyPlan(plan, doc, { file: new File(["cv"], "cv.pdf") });

    const email = doc.querySelector("#email") as HTMLInputElement;
    expect(email.getAttribute(FILL_ATTR)).toBe("filled");
    expect(email.style.outline).toContain("solid");

    // PRIME DIRECTIVE: the form must never be submitted by the fill.
    expect(submitted).toBe(false);
    expect(report.submitted).toBe(false);
  });

  it("flags an already-filled field as uncertain and never overwrites it", () => {
    const { doc } = makeDoc(greenhouseForm);
    const email = doc.querySelector("#email") as HTMLInputElement;
    email.value = "user-typed@example.com";

    const portal = getMapForHost("boards.greenhouse.io");
    const plan = buildFillPlan(portal, profile, doc);
    applyPlan(plan, doc);

    // Value preserved; state marked uncertain for review.
    expect(email.value).toBe("user-typed@example.com");
    expect(email.getAttribute(FILL_ATTR)).toBe("uncertain");
  });
});

describe("runFill orchestrator on lever (AC-APP-1)", () => {
  it("routes host -> map -> plan -> apply and reports no submit", () => {
    const win = new Window({ url: "https://jobs.lever.co/acme/1/apply" });
    win.document.body.innerHTML = `
      <form>
        <input name="name" type="text" />
        <input name="email" type="email" />
        <input name="phone" type="tel" />
      </form>`;
    const doc = win.document as unknown as Document;

    const report = runFill(doc, { hostname: "jobs.lever.co" }, profile);

    expect(report.host).toBe("jobs.lever.co");
    expect(report.submitted).toBe(false);
    expect(report.applied).toBeGreaterThan(0);
  });
});

describe("confirm-before-apply gate (AC-CV-5)", () => {
  it("writes nothing when the profile is not confirmed", () => {
    const { doc } = makeDoc(greenhouseForm);
    const unconfirmed: ProfileData = { ...profile, confirmed: false };
    const portal = getMapForHost("boards.greenhouse.io");
    const plan = buildFillPlan(portal, unconfirmed, doc);
    const report = applyPlan(plan, doc, { file: new File(["cv"], "cv.pdf") });

    expect(report.applied).toBe(0);
    expect((doc.querySelector("#email") as HTMLInputElement).value).toBe("");
  });
});
