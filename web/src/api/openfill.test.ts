import { describe, it, expect, vi } from "vitest";
import { requestOpenAndFill } from "./openfill";
import { ApiError } from "./http";

function deps(over: Partial<Parameters<typeof requestOpenAndFill>[1]> = {}) {
  return {
    arm: vi.fn().mockResolvedValue(true),
    sendToExtension: vi.fn().mockResolvedValue(true),
    openTab: vi.fn(),
    ...over,
  };
}

describe("requestOpenAndFill", () => {
  it("arms via the extension when the profile can arm and an extension responds", async () => {
    const d = deps();
    const res = await requestOpenAndFill("https://x/1", d);
    expect(res.status).toBe("armed");
    expect(d.sendToExtension).toHaveBeenCalledWith("https://x/1");
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
});
