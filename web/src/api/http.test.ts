import { describe, it, expect, vi, afterEach } from "vitest";
import { ApiError, apiFetch } from "./http";

function stubFetch(status: number, text: string) {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue({ ok: status >= 200 && status < 300, status, text: async () => text }),
  );
}

afterEach(() => vi.unstubAllGlobals());

describe("apiFetch with non-JSON bodies", () => {
  it("keeps the status when a gateway answers an error with HTML", async () => {
    stubFetch(502, "<html><body>Bad Gateway</body></html>");

    const error = await apiFetch("/feed").catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(502);
    expect((error as ApiError).message).toBe("request failed: 502");
  });

  it("rejects a 2xx HTML body instead of throwing a SyntaxError", async () => {
    stubFetch(200, "<!doctype html><html></html>");

    const error = await apiFetch("/snippets").catch((e: unknown) => e);

    expect(error).toBeInstanceOf(ApiError);
    expect((error as ApiError).status).toBe(200);
  });

  it("still returns null for an empty 2xx body and the server message for a JSON error", async () => {
    stubFetch(204, "");
    expect(await apiFetch("/saved-filters/1", { method: "DELETE" })).toBeNull();

    stubFetch(403, '{"error":"profile not confirmed"}');
    await expect(apiFetch("/profile/arm")).rejects.toMatchObject({ status: 403, message: "profile not confirmed" });
  });
});
