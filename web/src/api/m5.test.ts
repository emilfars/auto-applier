import { describe, it, expect, vi, afterEach } from "vitest";
import {
  confirmSubmitted,
  createSnippet,
  deleteSavedFilter,
  deleteSnippet,
  getCompleteness,
  listApplications,
  listSavedFilters,
  listSnippets,
  recordApplication,
  saveFilter,
  setJobState,
  updateApplication,
} from "./m5";

function mockFetch(status: number, body: unknown) {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    text: async () => (body === undefined ? "" : JSON.stringify(body)),
  });
}

function lastCall() {
  const f = vi.mocked(globalThis.fetch);
  return f.mock.calls.at(-1) as [string, { method?: string; body?: string } | undefined];
}

afterEach(() => vi.unstubAllGlobals());

describe("m5 job state client", () => {
  it("POSTs dismiss/applied and DELETEs undismiss", async () => {
    vi.stubGlobal(
      "fetch",
      mockFetch(200, { job_key: "k1", dismissed: true, already_applied: false }),
    );

    await setJobState("k1", "dismiss");
    expect(lastCall()[0]).toBe("/jobs/k1/dismiss");
    expect(lastCall()[1]?.method).toBe("POST");

    await setJobState("k1", "applied");
    expect(lastCall()[0]).toBe("/jobs/k1/applied");

    await setJobState("k1", "undismiss");
    expect(lastCall()[0]).toBe("/jobs/k1/dismiss");
    expect(lastCall()[1]?.method).toBe("DELETE");
  });

  it("URL-encodes job keys", async () => {
    vi.stubGlobal("fetch", mockFetch(200, {}));
    await setJobState("source:https://x/?a=1", "applied");
    expect(lastCall()[0]).toBe("/jobs/source%3Ahttps%3A%2F%2Fx%2F%3Fa%3D1/applied");
  });

  it("records an application as a job_id body", async () => {
    vi.stubGlobal("fetch", mockFetch(201, { id: "a1" }));
    await recordApplication("dedup-1");
    expect(lastCall()[0]).toBe("/applications");
    expect(lastCall()[1]?.method).toBe("POST");
    expect(JSON.parse(lastCall()[1]?.body ?? "{}")).toEqual({ job_id: "dedup-1" });
  });
});

describe("m5 application tracker client", () => {
  it("unwraps the applications envelope", async () => {
    vi.stubGlobal(
      "fetch",
      mockFetch(200, {
        applications: [{ id: "a1", company: "Acme", status: "form_filled" }],
      }),
    );
    const apps = await listApplications();
    expect(apps).toHaveLength(1);
    expect(apps[0].id).toBe("a1");
    expect(lastCall()[0]).toBe("/applications");
  });

  it("PATCHes status and POSTs the submitted confirmation", async () => {
    vi.stubGlobal("fetch", mockFetch(200, { id: "a1", status: "interview" }));
    await updateApplication("a1", "interview");
    expect(lastCall()[0]).toBe("/applications/a1");
    expect(lastCall()[1]?.method).toBe("PATCH");
    expect(JSON.parse(lastCall()[1]?.body ?? "{}")).toEqual({ status: "interview" });

    await confirmSubmitted("a1");
    expect(lastCall()[0]).toBe("/applications/a1/confirm");
    expect(lastCall()[1]?.method).toBe("POST");
  });
});

describe("m5 saved filters, snippets, completeness", () => {
  it("unwraps saved filters and creates one with its query", async () => {
    vi.stubGlobal(
      "fetch",
      mockFetch(200, {
        filters: [{ id: "f1", name: "Remote Go", query: { remote: true }, created_at: "", last_alerted_at: null }],
      }),
    );
    const filters = await listSavedFilters();
    expect(filters[0].name).toBe("Remote Go");
    expect(lastCall()[0]).toBe("/saved-filters");

    await saveFilter("Remote Go", { q: "go", remote: true });
    expect(lastCall()[0]).toBe("/saved-filters");
    expect(JSON.parse(lastCall()[1]?.body ?? "{}")).toEqual({
      name: "Remote Go",
      query: { q: "go", remote: true },
    });
  });

  it("DELETEs a saved filter by id", async () => {
    vi.stubGlobal("fetch", mockFetch(204, undefined));
    await deleteSavedFilter("f1");
    expect(lastCall()[0]).toBe("/saved-filters/f1");
    expect(lastCall()[1]?.method).toBe("DELETE");
  });

  it("unwraps snippets and creates/deletes them", async () => {
    vi.stubGlobal(
      "fetch",
      mockFetch(200, { snippets: [{ id: "s1", name: "Notice", body: "Hello", created_at: "", updated_at: "" }] }),
    );
    const snippets = await listSnippets();
    expect(snippets[0].body).toBe("Hello");
    expect(lastCall()[0]).toBe("/snippets");

    vi.stubGlobal("fetch", mockFetch(201, { id: "s2", name: "Notice", body: "Hi" }));
    await createSnippet("Notice", "Hi");
    expect(lastCall()[0]).toBe("/snippets");
    expect(JSON.parse(lastCall()[1]?.body ?? "{}")).toEqual({ name: "Notice", body: "Hi" });

    vi.stubGlobal("fetch", mockFetch(204, undefined));
    await deleteSnippet("s2");
    expect(lastCall()[0]).toBe("/snippets/s2");
    expect(lastCall()[1]?.method).toBe("DELETE");
  });

  it("reads the completeness score", async () => {
    vi.stubGlobal("fetch", mockFetch(200, { score: 80, complete: false, missing: ["phone"] }));
    const c = await getCompleteness();
    expect(c.score).toBe(80);
    expect(c.missing).toEqual(["phone"]);
    expect(lastCall()[0]).toBe("/profile/completeness");
  });
});
