import { describe, expect, it } from "vitest";
import { ApiError } from "../api/http";
import { errorMessage } from "./errors";
import { t } from "./index";

describe("errorMessage", () => {
  it.each([
    [new TypeError("Failed to fetch"), "error.network"],
    [new ApiError(401, "invalid session"), "error.session"],
    [new ApiError(429, "too many requests"), "error.rateLimit"],
    [new ApiError(502, "request failed: 502"), "error.server"],
    [new ApiError(200, "unexpected response from server"), "error.server"],
  ] as const)("maps %s to %s in both locales", (err, key) => {
    expect(errorMessage("id-ID", err)).toBe(t("id-ID", key));
    expect(errorMessage("en", err)).toBe(t("en", key));
  });

  it("lets a screen name a status in its own terms", () => {
    const err = new ApiError(401, "invalid credentials");
    expect(errorMessage("en", err, { 401: "auth.error.credentials" })).toBe(t("en", "auth.error.credentials"));
  });

  it("keeps the server's specific message for other 4xx", () => {
    expect(errorMessage("en", new ApiError(400, "notice_period_days must be 0-365"))).toBe(
      "notice_period_days must be 0-365",
    );
  });
});
