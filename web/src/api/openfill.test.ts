import { describe, it, expect, vi } from "vitest";
import { clearExtensionState, requestOpenAndFill } from "./openfill";
import { ApiError } from "./http";
import { getFillSnapshot, type FillSnapshot } from "./profile";

const snapshot: FillSnapshot = {
  profile: { confirmed: true, email: "u@example.com" },
  cv: null,
};

function deps(over: Partial<Parameters<typeof requestOpenAndFill>[1]> = {}) {
  return {
    arm: vi.fn().mockResolvedValue(true),
    sendToExtension: vi.fn().mockResolvedValue(true),
    snapshot: vi.fn().mockResolvedValue(snapshot),
    openTab: vi.fn(),
    ...over,
  };
}

describe("requestOpenAndFill", () => {
  it("arms via the extension when the profile can arm and an extension responds", async () => {
    const d = deps();
    const res = await requestOpenAndFill("https://x/1", d);
    expect(res.status).toBe("armed");
    expect(d.sendToExtension).toHaveBeenCalledWith("https://x/1", snapshot);
    // Extension opens the tab itself; the web app must not double-open.
    expect(d.openTab).not.toHaveBeenCalled();
  });

  it("falls back to opening the tab when no extension is reachable", async () => {
    const d = deps({ sendToExtension: vi.fn().mockResolvedValue(null) });
    const res = await requestOpenAndFill("https://x/2", d);
    expect(res.status).toBe("noExtension");
    expect(d.openTab).toHaveBeenCalledWith("https://x/2");
  });

  it("reports needProfile when the server says the profile cannot arm", async () => {
    const d = deps({ arm: vi.fn().mockResolvedValue(false) });
    const res = await requestOpenAndFill("https://x/3", d);
    expect(res.status).toBe("needProfile");
    expect(d.sendToExtension).not.toHaveBeenCalled();
  });

  it("maps a 401 to needLogin", async () => {
    const d = deps({ arm: vi.fn().mockRejectedValue(new ApiError(401, "unauthorized")) });
    expect((await requestOpenAndFill("https://x/4", d)).status).toBe("needLogin");
  });

  it("maps a 403 to needProfile", async () => {
    const d = deps({ arm: vi.fn().mockRejectedValue(new ApiError(403, "not confirmed")) });
    expect((await requestOpenAndFill("https://x/5", d)).status).toBe("needProfile");
  });

  it("reports error on an unexpected failure", async () => {
    const d = deps({ arm: vi.fn().mockRejectedValue(new Error("boom")) });
    expect((await requestOpenAndFill("https://x/6", d)).status).toBe("error");
  });

  it("NEVER submits — it only arms and opens (Prime Directive)", async () => {
    // The flow's only side effects are arm + open. There is no submit path.
    const d = deps({ sendToExtension: vi.fn().mockResolvedValue(null) });
    await requestOpenAndFill("https://x/7", d);
    expect(d.openTab).toHaveBeenCalledTimes(1);
  });

  it("passes the production profile/CV snapshot into the extension arm", async () => {
    const response: FillSnapshot = {
      profile: {
        confirmed: true,
        first_name: "Sri",
        last_name: "Wahyuni",
        email: "sri@example.com",
        linkedin_url: "https://linkedin.com/in/sri",
        github_url: "https://github.com/sri",
        portfolio_url: "https://sri.example.com",
        address: "Jl. Sudirman 1",
        city: "Jakarta",
        summary: "Software engineer",
        current_company: "Acme",
        current_title: "Engineer",
        highest_education: "S.Kom",
      },
      cv: {
        id: "cv-1",
        filename: "resume.pdf",
        content_type: "application/pdf",
        bytes_base64: "Y3YtYnl0ZXM=",
      },
    };
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(new Response(JSON.stringify(response), { status: 200 }));
    const d = deps({
      snapshot: () => getFillSnapshot(),
      sendToExtension: vi.fn().mockResolvedValue(true),
    });
    expect((await requestOpenAndFill("https://x/real", d)).status).toBe("armed");
    expect(fetchMock).toHaveBeenCalledWith(
      "/profile/fill",
      expect.objectContaining({ credentials: "include" }),
    );
    expect(d.sendToExtension).toHaveBeenCalledWith("https://x/real", response);
    fetchMock.mockRestore();
  });

  it("clears extension state through the origin- and request-id-checked page bridge", async () => {
    const postMessage = vi
      .spyOn(window, "postMessage")
      .mockImplementation((message, targetOrigin) => {
        const request = message as { requestId: string };
        expect(targetOrigin).toBe(window.location.origin);
        window.dispatchEvent(
          new MessageEvent("message", {
            source: window,
            origin: "https://evil.example",
            data: {
              source: "auto-applier-web",
              type: "clearStateResult",
              requestId: request.requestId,
              cleared: true,
            },
          }),
        );
        window.dispatchEvent(
          new MessageEvent("message", {
            source: window,
            origin: window.location.origin,
            data: {
              source: "auto-applier-web",
              type: "clearStateResult",
              requestId: "wrong-request",
              cleared: true,
            },
          }),
        );
        window.dispatchEvent(
          new MessageEvent("message", {
            source: window,
            origin: window.location.origin,
            data: {
              source: "auto-applier-web",
              type: "clearStateResult",
              requestId: request.requestId,
              cleared: true,
            },
          }),
        );
      });

    await expect(clearExtensionState()).resolves.toBe(true);
    expect(postMessage).toHaveBeenCalledWith(
      expect.objectContaining({ type: "clearState", requestId: expect.any(String) }),
      window.location.origin,
    );
    postMessage.mockRestore();
  });
});
