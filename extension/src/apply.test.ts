import { describe, it, expect, beforeEach } from "vitest";
import { Window } from "happy-dom";
import type { ProfileData } from "@auto-applier/fill-mappings";
import { applyPlan, FILL_ATTR } from "./apply.js";
import { installSubmissionGuard } from "./guard.js";
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
};

const profileWithCv: ProfileData = {
  ...profile,
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
    <input id="cover_letter" name="cover_letter" type="file" />
    <input id="portfolio" name="portfolio" type="file" />
    <button type="submit">Submit Application</button>
  </form>`;

const greenhouseMappableFields = 5; // first name, last name, email, phone, and resume

describe("applyPlan on a greenhouse form (AC-APP-1)", () => {
  let doc: Document;
  beforeEach(() => {
    ({ doc } = makeDoc(greenhouseForm));
  });

  it("fills matched text fields from the confirmed profile", () => {
    const portal = getMapForHost("boards.greenhouse.io");
    const plan = buildFillPlan(portal, profileWithCv, doc);
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

  it("reaches >=80% actual writes over expected fixture fields (AC-APP-1b)", () => {
    const portal = getMapForHost("boards.greenhouse.io");
    (doc.querySelector("#email") as HTMLInputElement).value = "already-entered@example.com";
    const plan = buildFillPlan(portal, profileWithCv, doc);
    const report = applyPlan(plan, doc, { file: new File(["cv"], "cv.pdf") });

    // Use actual writes over the fixture's expected mappable denominator. Planned
    // uncertain fields are not coverage until the applier reports a write.
    const applied = report.fields.filter((field) => field.applied).length;
    expect(applied / greenhouseMappableFields).toBeGreaterThanOrEqual(0.8);
    expect(report.coverage).toBeCloseTo(applied / greenhouseMappableFields);
  });

  it("dispatches input/change so framework-controlled inputs update", () => {
    const el = doc.querySelector("#email") as HTMLInputElement;
    const events: string[] = [];
    el.addEventListener("input", () => events.push("input"));
    el.addEventListener("change", () => events.push("change"));

    const portal = getMapForHost("boards.greenhouse.io");
    const plan = buildFillPlan(portal, profileWithCv, doc);
    applyPlan(plan, doc);

    expect(events).toContain("input");
    expect(events).toContain("change");
  });
});

describe("CV attach (AC-APP-2)", () => {
  it("attaches only to a resume input, never cover letter or portfolio inputs", () => {
    const { doc } = makeDoc(greenhouseForm);
    const portal = getMapForHost("boards.greenhouse.io");
    const plan = buildFillPlan(portal, profileWithCv, doc);
    const file = new File(["binary"], "resume.pdf", {
      type: "application/pdf",
    });
    applyPlan(plan, doc, { file });

    const input = doc.querySelector("#resume") as HTMLInputElement;
    expect(input.files?.length).toBe(1);
    expect(input.files?.[0]?.name).toBe("resume.pdf");
    expect((doc.querySelector("#cover_letter") as HTMLInputElement).files?.length ?? 0).toBe(0);
    expect((doc.querySelector("#portfolio") as HTMLInputElement).files?.length ?? 0).toBe(0);
  });

  it("does not attach when no CV file is provided", () => {
    const { doc } = makeDoc(greenhouseForm);
    const portal = getMapForHost("boards.greenhouse.io");
    const plan = buildFillPlan(portal, profile, doc);
    applyPlan(plan, doc);
    const input = doc.querySelector("#resume") as HTMLInputElement;
    expect(input.files?.length ?? 0).toBe(0);
  });

  it("does not auto-attach an uncertain CV on an unknown site", () => {
    const { doc } = makeDoc(`
      <form><input name="resume" type="file" /></form>
    `);
    const plan = buildFillPlan(
      getMapForHost("unknown.example"),
      profileWithCv,
      doc,
    );
    applyPlan(plan, doc, { file: new File(["binary"], "resume.pdf") });
    expect((doc.querySelector('input[name="resume"]') as HTMLInputElement).files?.length ?? 0).toBe(0);
  });

  it("does not attach a CV when the file outcome is uncertain", () => {
    const { doc } = makeDoc(greenhouseForm);
    const plan = buildFillPlan(
      getMapForHost("boards.greenhouse.io"),
      profileWithCv,
      doc,
    );
    const outcome = plan.outcomes.find((field) => field.key === "cv_file");
    if (!outcome) throw new Error("missing cv outcome");
    outcome.state = "uncertain";
    applyPlan(plan, doc, { file: new File(["binary"], "resume.pdf") });
    expect((doc.querySelector("#resume") as HTMLInputElement).files?.length ?? 0).toBe(0);
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

  it("marks mapped controls with no profile value as empty/review-needed", () => {
    const { doc } = makeDoc(`${greenhouseForm}<input id="salary" name="salary" />`);
    const portal = getMapForHost("boards.greenhouse.io");
    const noSalary = { ...profile };
    delete noSalary.expected_salary;
    const plan = buildFillPlan(portal, noSalary, doc);
    applyPlan(plan, doc);
    expect(doc.querySelector("#salary")?.getAttribute(FILL_ATTR)).toBe("empty");
  });

  it("blocks page handlers after unrelated trusted events during fill", () => {
    const { doc } = makeDoc(greenhouseForm);
    installSubmissionGuard(doc);
    const form = doc.querySelector("#application_form") as HTMLFormElement;
    const submitter = form.querySelector("button") as HTMLButtonElement;
    let submitted = 0;
    form.addEventListener("submit", (event) => {
      submitted++;
      event.preventDefault();
    });
    const email = doc.querySelector("#email") as HTMLInputElement;
    email.addEventListener("input", () => {
      form.submit();
      form.requestSubmit();
      submitter.click();
    });

    const plan = buildFillPlan(getMapForHost("boards.greenhouse.io"), profile, doc);
    applyPlan(plan, doc);
    expect(submitted).toBe(0);

    const trusted = new Event("submit", { bubbles: true, cancelable: true });
    Object.defineProperty(trusted, "isTrusted", { value: true });
    const activation = new Event("click", { bubbles: true });
    Object.defineProperty(activation, "isTrusted", { value: true });
    form.dispatchEvent(activation);
    form.dispatchEvent(trusted);
    expect(submitted).toBe(0);
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
