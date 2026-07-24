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
import type { ProfileKey } from "../types.js";

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

  it("is case-insensitive", () => {
    expect(getMapForHost("BOARDS.GREENHOUSE.IO").id).toBe(greenhouse.id);
  });
});
