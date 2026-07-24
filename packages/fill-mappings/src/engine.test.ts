import { describe, it, expect } from "vitest";
import { buildFillPlan, UNCERTAIN_BELOW } from "./engine.js";
import type { FillElement, QueryRoot } from "./dom.js";
import type { PortalMap, ProfileData } from "./types.js";

/** A fake element exposing only what the engine reads. */
function el(opts: Partial<FillElement> & { tagName?: string } = {}): FillElement {
  return {
    tagName: opts.tagName ?? "INPUT",
    ...(opts.value !== undefined ? { value: opts.value } : {}),
    ...(opts.disabled !== undefined ? { disabled: opts.disabled } : {}),
    getAttribute: () => null,
  };
}

/** A fake root that matches selectors by exact string key. */
function root(bySelector: Record<string, FillElement>): QueryRoot {
  return {
    querySelector: (sel: string) => bySelector[sel] ?? null,
  };
}

const testMap: PortalMap = {
  id: "test",
  version: "1.0.0",
  hosts: ["test.example"],
  fields: [
    { key: "full_name", selectors: ["#name", 'input[name="name"]'], confidence: 0.9 },
    { key: "email", selectors: ["#email"], confidence: 0.9 },
    { key: "city", selectors: ["#city", ".city-fallback"], confidence: 0.7 },
    { key: "cv_file", selectors: ["#resume"], confidence: 0.9, file: true },
  ],
};

const confirmed: ProfileData = {
  confirmed: true,
  full_name: "Budi Santoso",
  email: "budi@example.com",
  city: "Jakarta Selatan",
  cv_file: "cv-budi.pdf",
};

describe("AC-CV-5: fill is not armed for an unconfirmed profile", () => {
  it("returns all-empty outcomes and zero coverage", () => {
    const plan = buildFillPlan(testMap, { ...confirmed, confirmed: false }, root({ "#name": el(), "#email": el() }));
    expect(plan.filled).toBe(0);
    expect(plan.coverage).toBe(0);
    for (const o of plan.outcomes) {
      expect(o.state).toBe("empty");
      expect(o.note).toMatch(/not confirmed/);
    }
  });
});

describe("AC-SAFE-2: per-field filled | uncertain | empty", () => {
  it("fills high-confidence fields whose element is present and empty", () => {
    const plan = buildFillPlan(testMap, confirmed, root({ "#name": el(), "#email": el() }));
    const name = plan.outcomes.find((o) => o.key === "full_name")!;
    expect(name.state).toBe("filled");
    expect(name.value).toBe("Budi Santoso");
    expect(name.confidence).toBeGreaterThanOrEqual(UNCERTAIN_BELOW);
  });

  it("marks a field empty when the element is absent", () => {
    const plan = buildFillPlan(testMap, confirmed, root({ "#name": el() }));
    const email = plan.outcomes.find((o) => o.key === "email")!;
    expect(email.state).toBe("empty");
    expect(email.value).toBeUndefined();
  });

  it("marks a field empty when no profile value exists", () => {
    const noCity: ProfileData = { confirmed: true, full_name: "A B" };
    const plan = buildFillPlan(testMap, noCity, root({ "#name": el(), "#city": el() }));
    const city = plan.outcomes.find((o) => o.key === "city")!;
    expect(city.state).toBe("empty");
  });

  it("flags a fallback-selector match as uncertain (confidence decays)", () => {
    // Only the fallback ".city-fallback" is present, not the primary "#city".
    const plan = buildFillPlan(testMap, confirmed, root({ ".city-fallback": el() }));
    const city = plan.outcomes.find((o) => o.key === "city")!;
    expect(city.state).toBe("uncertain");
    expect(city.confidence).toBeLessThan(UNCERTAIN_BELOW);
    expect(city.value).toBe("Jakarta Selatan");
  });

  it("never overwrites a user-entered value; flags uncertain instead", () => {
    const plan = buildFillPlan(testMap, confirmed, root({ "#name": el({ value: "Typed Already" }) }));
    const name = plan.outcomes.find((o) => o.key === "full_name")!;
    expect(name.state).toBe("uncertain");
    expect(name.note).toMatch(/already has a value/);
  });

  it("skips disabled fields", () => {
    const plan = buildFillPlan(testMap, confirmed, root({ "#name": el({ disabled: true }) }));
    const name = plan.outcomes.find((o) => o.key === "full_name")!;
    expect(name.state).toBe("empty");
    expect(name.note).toMatch(/disabled/);
  });

  it("plans a CV attach when a file input and a CV are present", () => {
    const plan = buildFillPlan(testMap, confirmed, root({ "#resume": el({ tagName: "INPUT" }) }));
    const cv = plan.outcomes.find((o) => o.key === "cv_file")!;
    expect(cv.file).toBe(true);
    expect(cv.state).toBe("filled");
    expect(cv.value).toBe("cv-budi.pdf");
  });

  it("invariant: no outcome labeled filled has confidence below the threshold", () => {
    const plan = buildFillPlan(testMap, confirmed, root({ "#name": el(), "#email": el(), "#resume": el() }));
    for (const o of plan.outcomes) {
      if (o.state === "filled") expect(o.confidence).toBeGreaterThanOrEqual(UNCERTAIN_BELOW);
    }
  });
});

describe("coverage metrics", () => {
  it("counts filled/uncertain/empty and computes coverage", () => {
    const plan = buildFillPlan(testMap, confirmed, root({ "#name": el(), "#email": el() }));
    expect(plan.filled + plan.uncertain + plan.empty).toBe(testMap.fields.length);
    expect(plan.coverage).toBeCloseTo(plan.filled / testMap.fields.length);
  });
});
