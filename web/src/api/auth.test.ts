import { describe, it, expect, vi, afterEach } from "vitest";
import { register, login, verify, me, requestReset } from "./auth";
import { ApiError } from "./http";

function mockFetch(status: number, body: unknown) {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    text: async () => (body === undefined ? "" : JSON.stringify(body)),
  });
}

afterEach(() => vi.unstubAllGlobals());

describe("auth client", () => {
  it("registers with consent and credentials", async () => {
    const f = mockFetch(201, { id: "u1", email: "a@b.co", verified: false, consent: true });
    vi.stubGlobal("fetch", f);

    const res = await register("a@b.co", "password123", true);
    expect(res.email).toBe("a@b.co");
    expect(f).toHaveBeenCalledWith(
      "/auth/register",
      expect.objectContaining({ method: "POST", credentials: "include" }),
    );
    const init = f.mock.calls[0][1] as { body: string };
    expect(JSON.parse(init.body)).toEqual({
      email: "a@b.co",
      password: "password123",
      consent: true,
    });
  });

  it("logs in and posts credentials to /auth/login", async () => {
    const f = mockFetch(200, { status: "authenticated" });
    vi.stubGlobal("fetch", f);
    await login("a@b.co", "password123");
    expect(f.mock.calls[0][0]).toBe("/auth/login");
  });

  it("verifies with a token", async () => {
    const f = mockFetch(200, { status: "verified" });
    vi.stubGlobal("fetch", f);
    const res = await verify("tok");
    expect(res.status).toBe("verified");
    expect(f.mock.calls[0][0]).toBe("/auth/verify");
  });

  it("me() returns null when unauthenticated", async () => {
    vi.stubGlobal("fetch", mockFetch(401, { error: "authentication required" }));
    expect(await me()).toBeNull();
  });

  it("surfaces the server error message as ApiError", async () => {
    vi.stubGlobal("fetch", mockFetch(409, { error: "email already registered" }));
    await expect(register("a@b.co", "password123", true)).rejects.toBeInstanceOf(ApiError);
    await expect(register("a@b.co", "password123", true)).rejects.toThrow(/already registered/);
  });

  it("requestReset always resolves (no account enumeration)", async () => {
    vi.stubGlobal("fetch", mockFetch(200, { status: "ok" }));
    const res = await requestReset("nobody@b.co");
    expect(res.status).toBe("ok");
  });
});
