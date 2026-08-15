import { describe, expect, it } from "vitest";
import { asClearState, isTrustedExternalOrigin } from "./messaging.js";

describe("external message origin policy", () => {
  it("accepts the production origin and localhost dev ports", () => {
    expect(isTrustedExternalOrigin("https://autoapplier.id/feed")).toBe(true);
    expect(isTrustedExternalOrigin("http://localhost:5173/feed")).toBe(true);
    expect(isTrustedExternalOrigin("http://localhost:8080")).toBe(true);
  });

  it("rejects lookalike, insecure, and wildcard subdomain origins", () => {
    for (const url of [
      "https://www.autoapplier.id/feed",
      "https://evil.autoapplier.id/feed",
      "http://autoapplier.id/feed",
      "https://autoapplier.id.evil.example/feed",
      "https://autoapplier.id:8443/feed",
      "http://localhost.evil.example/feed",
      undefined,
    ]) {
      expect(isTrustedExternalOrigin(url)).toBe(false);
    }
  });
});

describe("clear-state page bridge message", () => {
  it("accepts only the clear-state command envelope", () => {
    expect(asClearState({ type: "clearState" })).toEqual({ type: "clearState" });
    expect(asClearState({ type: "clearState", requestId: "ignored" })).toEqual({
      type: "clearState",
    });
    expect(asClearState({ type: "openAndFill" })).toBeNull();
  });
});
