import { describe, expect, it, vi } from "vitest";

describe("background Open & Fill navigation", () => {
  it("rechecks a tab that completed before the arm was persisted", async () => {
    vi.resetModules();
    const state: Record<string, unknown> = {};
    let onUpdated:
      | ((tabId: number, changeInfo: chrome.tabs.TabChangeInfo, tab: chrome.tabs.Tab) => void)
      | undefined;
    let onExternal:
      | ((
          message: unknown,
          sender: chrome.runtime.MessageSender,
          sendResponse: (response?: unknown) => void,
        ) => boolean | void)
      | undefined;
    const url = "https://jobs.lever.co/acme/1/apply";
    const tab = { id: 7, status: "complete", url } as chrome.tabs.Tab;
    const tabs = {
      create: vi.fn(async () => {
        onUpdated?.(7, { status: "complete" }, tab);
        return tab;
      }),
      get: vi.fn(async () => tab),
      onUpdated: {
        addListener: vi.fn((listener) => {
          onUpdated = listener;
        }),
      },
    };
    const executeScript = vi.fn(async () => []);
    const fakeChrome = {
      storage: {
        session: {
          get: vi.fn(async (key: string) =>
            key in state ? { [key]: state[key] } : {},
          ),
          set: vi.fn(async (items: Record<string, unknown>) => {
            Object.assign(state, items);
          }),
          remove: vi.fn(async (key: string) => {
            delete state[key];
          }),
        },
      },
      alarms: {
        create: vi.fn(async () => undefined),
        clear: vi.fn(async () => false),
        onAlarm: { addListener: vi.fn() },
      },
      tabs,
      scripting: { executeScript },
      runtime: {
        onMessageExternal: {
          addListener: vi.fn((listener) => {
            onExternal = listener;
          }),
        },
        onMessage: { addListener: vi.fn() },
      },
    } as unknown as typeof chrome;
    vi.stubGlobal("chrome", fakeChrome);

    await import("./background.js");
    const response = new Promise<unknown>((resolve) => {
      const keepAlive = onExternal?.(
        { type: "openAndFill", url },
        { url: "http://localhost:5173/feed" },
        resolve,
      );
      expect(keepAlive).toBe(true);
    });

    await expect(response).resolves.toEqual({ armed: true });
    expect(tabs.get).toHaveBeenCalledWith(7);
    expect(executeScript).toHaveBeenCalledTimes(2);
    vi.unstubAllGlobals();
  });

  it("does not repersist an arm when clear races a pending tab creation", async () => {
    vi.resetModules();
    const state: Record<string, unknown> = {};
    let onExternal:
      | ((
          message: unknown,
          sender: chrome.runtime.MessageSender,
          sendResponse: (response?: unknown) => void,
        ) => boolean | void)
      | undefined;
    let onInternal:
      | ((
          message: unknown,
          sender: chrome.runtime.MessageSender,
          sendResponse: (response?: unknown) => void,
        ) => boolean | void)
      | undefined;
    let resolveTab!: (tab: chrome.tabs.Tab) => void;
    const tabPromise = new Promise<chrome.tabs.Tab>((resolve) => {
      resolveTab = resolve;
    });
    const tab = {
      id: 11,
      status: "complete",
      url: "https://jobs.lever.co/acme/2/apply",
    } as chrome.tabs.Tab;
    const fakeChrome = {
      storage: {
        session: {
          get: vi.fn(async (key: string) =>
            key in state ? { [key]: state[key] } : {},
          ),
          set: vi.fn(async (items: Record<string, unknown>) => {
            Object.assign(state, items);
          }),
          remove: vi.fn(async (key: string) => {
            delete state[key];
          }),
        },
      },
      alarms: {
        create: vi.fn(async () => undefined),
        clear: vi.fn(async () => false),
        onAlarm: { addListener: vi.fn() },
      },
      tabs: {
        create: vi.fn(async () => tabPromise),
        get: vi.fn(async () => tab),
        onUpdated: { addListener: vi.fn() },
      },
      scripting: { executeScript: vi.fn(async () => []) },
      runtime: {
        onMessageExternal: {
          addListener: vi.fn((listener) => {
            onExternal = listener;
          }),
        },
        onMessage: {
          addListener: vi.fn((listener) => {
            onInternal = listener;
          }),
        },
      },
    } as unknown as typeof chrome;
    vi.stubGlobal("chrome", fakeChrome);

    await import("./background.js");
    const openResponse = new Promise<unknown>((resolve) => {
      expect(
        onExternal?.(
          { type: "openAndFill", url: tab.url },
          { url: "http://localhost:5173/feed" },
          resolve,
        ),
      ).toBe(true);
    });
    const clearResponse = new Promise<unknown>((resolve) => {
      expect(
        onExternal?.(
          { type: "clearState" },
          { url: "http://localhost:5173/feed" },
          resolve,
        ),
      ).toBe(true);
    });
    resolveTab(tab);

    await expect(clearResponse).resolves.toEqual({ cleared: true });
    await expect(openResponse).resolves.toEqual({ armed: false });
    expect(state.autoApplierArms).toBeUndefined();

    const internalClear = new Promise<unknown>((resolve) => {
      expect(
        onInternal?.(
          { type: "clearState" },
          { url: "http://localhost:5173/feed" },
          resolve,
        ),
      ).toBe(true);
    });
    await expect(internalClear).resolves.toEqual({ cleared: true });
    vi.unstubAllGlobals();
  });
});
