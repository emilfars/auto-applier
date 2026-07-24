import { describe, it, expect, vi, afterEach } from "vitest";
import {
  buildFeedPath,
  fetchFeed,
  hasStatedSalary,
  type FeedResponse,
  type Salary,
} from "./feed";

describe("buildFeedPath", () => {
  it("returns bare /feed with no filters", () => {
    expect(buildFeedPath()).toBe("/feed");
    expect(buildFeedPath({})).toBe("/feed");
  });

  it("omits empty/whitespace filters", () => {
    expect(buildFeedPath({ q: "  ", location: "" })).toBe("/feed");
  });

  it("encodes provided filters", () => {
    const path = buildFeedPath({
      q: "backend engineer",
      location: "Jakarta",
      remote: true,
      limit: 24,
    });
    const params = new URLSearchParams(path.split("?")[1]);
    expect(params.get("q")).toBe("backend engineer");
    expect(params.get("location")).toBe("Jakarta");
    expect(params.get("remote")).toBe("true");
    expect(params.get("limit")).toBe("24");
  });
});

describe("hasStatedSalary", () => {
  const base: Salary = {
    stated_min: null,
    stated_max: null,
    currency: "IDR",
    label: "",
    stated: false,
  };

  it("is true only for a stated, labeled salary", () => {
    expect(hasStatedSalary({ ...base, stated: true, label: "Rp 8.000.000" })).toBe(true);
  });

  it("is false when not stated or unlabeled (never an estimate)", () => {
    expect(hasStatedSalary(base)).toBe(false);
    expect(hasStatedSalary({ ...base, stated: true, label: "" })).toBe(false);
  });
});

describe("fetchFeed", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("requests the built path and returns the parsed feed", async () => {
    const body: FeedResponse = { total: 0, limit: 24, offset: 0, jobs: [] };
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => body,
    });
    vi.stubGlobal("fetch", fetchMock);

    const res = await fetchFeed({ q: "data", limit: 24 });
    expect(res).toEqual(body);
    expect(fetchMock).toHaveBeenCalledWith(
      "/feed?q=data&limit=24",
      expect.objectContaining({ headers: { Accept: "application/json" } }),
    );
  });

  it("throws on a non-ok response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 500 }));
    await expect(fetchFeed()).rejects.toThrow(/500/);
  });
});
