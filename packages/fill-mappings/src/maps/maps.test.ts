import { describe, it, expect } from "vitest";
import {
  portalMaps,
  genericMap,
  getMapForHost,
  greenhouse,
  lever,
  jobstreet,
  glints,
  kalibrr,
  generic,
} from "./index.js";
import { buildFillPlan, UNCERTAIN_BELOW } from "../engine.js";
import type { FillElement } from "../dom.js";
import type { ProfileData, ProfileKey } from "../types.js";

const allMaps = [...portalMaps, genericMap];

const validKeys = new Set<ProfileKey>([
  "full_name", "first_name", "last_name", "email", "phone", "city", "address",
  "linkedin_url", "portfolio_url", "github_url", "expected_salary",
  "notice_period", "years_experience", "current_company", "current_title",
  "highest_education", "university", "major", "graduation_year", "summary",
  "cv_file",
]);

describe("AC-MAP-1: each supported portal map is versioned and well-formed", () => {
  it("covers the seven required portals", () => {
    const ids = allMaps.map((m) => m.id).sort();
    expect(ids).toEqual(
      ["generic", "glints", "greenhouse", "jobstreet", "kalibrr", "lever", "workable"].sort(),
    );
  });

  for (const map of allMaps) {
    describe(`map: ${map.id}`, () => {
      it("has a semver-ish version", () => {
        expect(map.version).toMatch(/^\d+\.\d+\.\d+$/);
      });

      it("has at least one field", () => {
        expect(map.fields.length).toBeGreaterThan(0);
      });

      it("every field has a valid key and non-empty selectors", () => {
        for (const f of map.fields) {
          expect(validKeys.has(f.key)).toBe(true);
          expect(f.selectors.length).toBeGreaterThan(0);
          for (const s of f.selectors) expect(s.trim()).not.toBe("");
        }
      });

      it("has no duplicate profile keys", () => {
        const keys = map.fields.map((f) => f.key);
        expect(new Set(keys).size).toBe(keys.length);
      });

      it("confidence values are within [0,1]", () => {
        for (const f of map.fields) {
          expect(f.confidence).toBeGreaterThan(0);
          expect(f.confidence).toBeLessThanOrEqual(1);
        }
      });
    });
  }

  it("portal ids are unique", () => {
    const ids = allMaps.map((m) => m.id);
    expect(new Set(ids).size).toBe(ids.length);
  });

  it("portal-specific maps declare hosts; generic does not", () => {
    for (const m of portalMaps) expect(m.hosts.length).toBeGreaterThan(0);
    expect(generic.hosts.length).toBe(0);
  });

  it("file selectors never fall back to arbitrary uploads", () => {
    for (const map of allMaps) {
      for (const field of map.fields.filter((candidate) => candidate.file)) {
        expect(field.selectors.every((selector) => selector.includes("resume") || selector.includes("cv"))).toBe(
          true,
        );
      }
    }
  });

  it("resolves profile-backed controls in each supported fixture", () => {
    const profile: ProfileData = {
      confirmed: true,
      full_name: "Sri Wahyuni",
      first_name: "Sri",
      last_name: "Wahyuni",
      email: "sri@example.com",
      phone: "+628123456789",
      linkedin_url: "https://linkedin.com/in/sri",
      github_url: "https://github.com/sri",
      portfolio_url: "https://sri.example.com",
      address: "Jl. Sudirman 1",
      city: "Jakarta",
      summary: "Software engineer",
      expected_salary: "12000000",
      current_company: "Acme",
      current_title: "Engineer",
      highest_education: "S.Kom",
      cv_file: "resume.pdf",
    };
    const fixtureElement: FillElement = {
      tagName: "INPUT",
      disabled: false,
      value: "",
      getAttribute: () => null,
    };
    for (const map of allMaps) {
      const selectors = new Set(
        map.fields
          .filter((field) => profile[field.key] !== undefined)
          .flatMap((field) => field.selectors),
      );
      const plan = buildFillPlan(map, profile, {
        querySelector: (selector) => (selectors.has(selector) ? fixtureElement : null),
      });
      const expectedKeys = map.fields
        .filter((field) => profile[field.key] !== undefined)
        .map((field) => field.key);
      const resolvedKeys = plan.outcomes
        .filter((outcome) => outcome.selector !== undefined)
        .map((outcome) => outcome.key);
      expect(resolvedKeys, map.id).toEqual(expectedKeys);
    }
  });

  it("keeps every unknown-site match uncertain", () => {
    const profile: ProfileData = {
      confirmed: true,
      full_name: "Sri Wahyuni",
      email: "sri@example.com",
      phone: "+628123456789",
      city: "Jakarta",
      linkedin_url: "https://linkedin.com/in/sri",
      expected_salary: "12000000",
      cv_file: "resume.pdf",
    };
    const fixtureElement: FillElement = {
      tagName: "INPUT",
      disabled: false,
      value: "",
      getAttribute: () => null,
    };
    const plan = buildFillPlan(generic, profile, {
      querySelector: (selector) =>
        generic.fields.some((field) => field.selectors.includes(selector))
          ? fixtureElement
          : null,
    });

    for (const outcome of plan.outcomes) {
      if (outcome.selector === undefined) continue;
      expect(outcome.state).toBe("uncertain");
      expect(outcome.confidence).toBeLessThan(UNCERTAIN_BELOW);
    }
    expect(plan.outcomes.find((outcome) => outcome.key === "cv_file")?.state).toBe(
      "uncertain",
    );
  });
});

describe("getMapForHost resolves the right map", () => {
  it("matches known ATS hosts", () => {
    expect(getMapForHost("boards.greenhouse.io").id).toBe(greenhouse.id);
    expect(getMapForHost("jobs.lever.co").id).toBe(lever.id);
    expect(getMapForHost("id.jobstreet.com").id).toBe(jobstreet.id);
    expect(getMapForHost("www.glints.com").id).toBe(glints.id);
    expect(getMapForHost("kalibrr.com").id).toBe(kalibrr.id);
  });

  it("falls back to generic for unknown hosts", () => {
    expect(getMapForHost("careers.some-random-company.co.id").id).toBe(generic.id);
  });

  it("does NOT match look-alike / typosquat hosts (safety split)", () => {
    // Bare-suffix look-alikes must fall through to the generic map so their PII
    // lands as `uncertain`, never as auto-commit-eligible `filled`.
    expect(getMapForHost("notlever.co").id).toBe(generic.id);
    expect(getMapForHost("fakejobstreet.com").id).toBe(generic.id);
    expect(getMapForHost("phishingglints.com").id).toBe(generic.id);
    expect(getMapForHost("evil-greenhouse.io").id).toBe(generic.id);
    expect(getMapForHost("kalibrr.com.evil.example").id).toBe(generic.id);
  });

  it("still matches legitimate subdomains on a dot boundary", () => {
    expect(getMapForHost("careers.lever.co").id).toBe(lever.id);
    expect(getMapForHost("www.glints.com").id).toBe(glints.id);
    expect(getMapForHost("boards.greenhouse.io").id).toBe(greenhouse.id);
  });

  it("is case-insensitive", () => {
    expect(getMapForHost("BOARDS.GREENHOUSE.IO").id).toBe(greenhouse.id);
  });
});
