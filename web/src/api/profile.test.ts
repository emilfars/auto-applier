import { describe, it, expect, vi, afterEach } from "vitest";
import { getProfile, patchProfile, confirmProfile } from "./profile";
import { listCVs, uploadCV, parseCV } from "./cv";
import { ApiError } from "./http";

function mockFetch(status: number, body: unknown) {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    text: async () => (body === undefined ? "" : JSON.stringify(body)),
  });
}

afterEach(() => vi.unstubAllGlobals());

const sampleProfile = {
  full_name: "Sri",
  email: "sri@example.com",
  phone: "0812",
  education: [],
  work_history: [],
  skills: ["go", "react"],
  expected_salary: 15000000,
  notice_period_days: 30,
  work_authorization: "WNI",
  open_to_relocation: false,
  preferred_locations: ["Jakarta"],
  employment_type: "full_time",
  confirmed: false,
  confirmed_at: null,
};

describe("profile client", () => {
  it("gets the profile", async () => {
    const f = mockFetch(200, sampleProfile);
    vi.stubGlobal("fetch", f);
    const p = await getProfile();
    expect(p.skills).toEqual(["go", "react"]);
    expect(f.mock.calls[0][0]).toBe("/profile");
  });

  it("PATCHes all parsed fields", async () => {
    const f = mockFetch(200, sampleProfile);
    vi.stubGlobal("fetch", f);
    await patchProfile({
      full_name: "Sri",
      email: "sri@example.com",
      education: [{ institution: "UI", degree: "S.Kom", field: "", start_year: "2020", end_year: "2024" }],
      work_history: [{ company: "Acme", title: "Engineer", start_date: "2024", end_date: "" }],
      skills: ["go"],
    });
    const [path, init] = f.mock.calls[0] as [string, { method: string; body: string }];
    expect(path).toBe("/profile");
    expect(init.method).toBe("PATCH");
    expect(JSON.parse(init.body).skills).toEqual(["go"]);
    expect(JSON.parse(init.body).education[0].institution).toBe("UI");
    expect(JSON.parse(init.body).work_history[0].company).toBe("Acme");
  });

  it("confirms the profile", async () => {
    const f = mockFetch(200, { ...sampleProfile, confirmed: true });
    vi.stubGlobal("fetch", f);
    const p = await confirmProfile();
    expect(p.confirmed).toBe(true);
    expect(f.mock.calls[0][0]).toBe("/profile/confirm");
  });
});

describe("cv client", () => {
  it("lists CVs, defaulting to an empty array", async () => {
    vi.stubGlobal("fetch", mockFetch(200, { files: [] }));
    expect(await listCVs()).toEqual([]);
  });

  it("uploads a file as multipart FormData", async () => {
    const f = mockFetch(201, { id: "c1", filename: "cv.pdf", content_type: "application/pdf", size_bytes: 10, created_at: "" });
    vi.stubGlobal("fetch", f);
    const file = new File([new Uint8Array([1, 2, 3])], "cv.pdf", { type: "application/pdf" });
    const rec = await uploadCV(file);
    expect(rec.filename).toBe("cv.pdf");
    const init = f.mock.calls[0][1] as { body: unknown };
    expect(init.body).toBeInstanceOf(FormData);
  });

  it("maps a 503 parse response to an ApiError", async () => {
    vi.stubGlobal("fetch", mockFetch(503, { error: "cv parsing is not available" }));
    await expect(parseCV("c1")).rejects.toBeInstanceOf(ApiError);
  });
});
