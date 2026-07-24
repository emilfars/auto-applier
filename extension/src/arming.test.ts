import { describe, it, expect } from "vitest";
import { ArmingStore, normalizeUrl } from "./arming.js";

describe("normalizeUrl", () => {
  it("drops query, hash, and trailing slash and lowercases", () => {
    expect(normalizeUrl("https://Jobs.Lever.co/acme/1/?utm=x#top")).toBe(
      "https://jobs.lever.co/acme/1",
    );
    expect(normalizeUrl("https://jobs.lever.co/acme/1")).toBe(
      "https://jobs.lever.co/acme/1",
    );
  });

  it("matches the armed URL after tracking params are appended", () => {
    const a = normalizeUrl("https://boards.greenhouse.io/acme/jobs/9");
    const b = normalizeUrl("https://boards.greenhouse.io/acme/jobs/9?src=feed");
    expect(a).toBe(b);
  });
});

describe("ArmingStore", () => {
  it("arms a URL and consumes it exactly once", () => {
    const s = new ArmingStore();
    s.arm("https://jobs.lever.co/acme/1", 1000);
    expect(s.consume("https://jobs.lever.co/acme/1", 1500)).toBe(true);
    // Second consume returns false (single-use).
    expect(s.consume("https://jobs.lever.co/acme/1", 1600)).toBe(false);
  });

  it("does not consume an unrelated URL", () => {
    const s = new ArmingStore();
    s.arm("https://jobs.lever.co/acme/1", 1000);
    expect(s.consume("https://jobs.lever.co/other/2", 1500)).toBe(false);
  });

  it("treats an expired arm as absent", () => {
    const s = new ArmingStore(60_000);
    s.arm("https://x.io/a", 1000);
    expect(s.consume("https://x.io/a", 1000 + 60_001)).toBe(false);
  });

  it("sweeps expired entries", () => {
    const s = new ArmingStore(1000);
    s.arm("https://x.io/a", 0);
    s.arm("https://x.io/b", 0);
    expect(s.size()).toBe(2);
    s.sweep(2000);
    expect(s.size()).toBe(0);
  });
});
