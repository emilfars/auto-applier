import { afterEach, describe, expect, it, vi } from "vitest";
import { deleteAccount, exportAccount, type AccountExport } from "./account";

function mockFetch(status: number, body: unknown) {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    text: async () => (body === undefined ? "" : JSON.stringify(body)),
  });
}

afterEach(() => vi.unstubAllGlobals());

const sampleExport: AccountExport = {
  account: {
    email: "sri@example.com",
    verified: true,
    created_at: "2026-01-01T00:00:00Z",
    consent_at: "2026-01-01T00:00:00Z",
  },
  profile: null,
  cv_files: [{ filename: "cv.pdf", content_type: "application/pdf", size_bytes: 10, created_at: "" }],
  exported_at: "2026-01-02T00:00:00Z",
};

describe("account client", () => {
  it("exports account data via GET /account/export", async () => {
    const f = mockFetch(200, sampleExport);
    vi.stubGlobal("fetch", f);

    const data = await exportAccount();

    expect(data.account.email).toBe("sri@example.com");
    const [path, init] = f.mock.calls[0] as [string, { method: string }];
    expect(path).toBe("/account/export");
    expect(init.method).toBe("GET");
  });

  it("deletes the account via DELETE /account", async () => {
    const f = mockFetch(204, undefined);
    vi.stubGlobal("fetch", f);

    await deleteAccount();
    const [path, init] = f.mock.calls[0] as [string, { method: string }];
    expect(path).toBe("/account");
    expect(init.method).toBe("DELETE");
  });
});
