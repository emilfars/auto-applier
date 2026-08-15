/**
 * Background service worker: routes the explicit Open & Fill action, keeps its
 * profile/CV snapshot in storage.session, and dynamically injects only into the
 * tab the user just opened. It never submits.
 */
import {
  ARM_EXPIRY_ALARM,
  installArmExpiryAlarmListener,
  SessionArmingStore,
  type SessionStorageLike,
  type ArmExpiryAlarmScheduler,
} from "./arming.js";
import {
  asClearState,
  asFillCorrection,
  asIsArmed,
  asOpenAndFill,
  isTrustedExternalOrigin,
} from "./messaging.js";

const alarmScheduler: ArmExpiryAlarmScheduler = {
  schedule: (expiresAt) =>
    chrome.alarms.create(ARM_EXPIRY_ALARM, { when: expiresAt }),
  clear: async () => {
    await chrome.alarms.clear(ARM_EXPIRY_ALARM);
  },
};
const session = new SessionArmingStore(
  chrome.storage.session as unknown as SessionStorageLike,
  undefined,
  undefined,
  alarmScheduler,
);
const TELEMETRY_URL = "https://autoapplier.id/telemetry/fill-correction";
let armGeneration = 0;

installArmExpiryAlarmListener(chrome.alarms, () => session.sweep());
void session.sweep();

function openable(url: string): boolean {
  try {
    return new URL(url).protocol === "https:";
  } catch {
    return false;
  }
}

async function openAndArm(
  url: string,
  snapshot?: Parameters<typeof session.arm>[3],
): Promise<{ armed: boolean }> {
  if (!openable(url)) return { armed: false };
  const generation = armGeneration;
  const tab = await chrome.tabs.create({ url });
  if (tab.id === undefined || generation !== armGeneration) return { armed: false };
  await session.arm(url, Date.now(), tab.id, snapshot);
  try {
    const current = await chrome.tabs.get(tab.id);
    if (current.status === "complete" && current.url) {
      await inject(tab.id, current.url);
    }
  } catch {
    // The tab may have closed while the arm was being persisted.
  }
  return { armed: true };
}

async function inject(tabId: number, url: string): Promise<void> {
  if (!(await session.isArmed(url, Date.now(), tabId))) return;
  try {
    await chrome.scripting.executeScript({
      target: { tabId },
      files: ["dist/guard-entry.js"],
      world: "MAIN",
    });
    await chrome.scripting.executeScript({
      target: { tabId },
      files: ["dist/content-entry.js"],
    });
  } catch {
    // The tab may have navigated or disallowed script execution; the arm expires.
  }
}

function handleOpenAndFill(
  message: unknown,
  sender: chrome.runtime.MessageSender,
  sendResponse: (response?: unknown) => void,
): boolean {
  if (!isTrustedExternalOrigin(sender.url)) {
    sendResponse({ armed: false });
    return false;
  }
  const open = asOpenAndFill(message);
  if (!open) return false;
  void openAndArm(open.url, open.snapshot).then(sendResponse, () => sendResponse({ armed: false }));
  return true;
}

chrome.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
  if (changeInfo.status === "complete" && tab.url) {
    void inject(tabId, tab.url);
  }
});

chrome.runtime.onMessageExternal.addListener((message, sender, sendResponse) => {
  if (!isTrustedExternalOrigin(sender.url)) {
    sendResponse({ armed: false });
    return false;
  }
  if (asOpenAndFill(message)) return handleOpenAndFill(message, sender, sendResponse);
  if (asClearState(message)) {
    armGeneration++;
    void session.clear().then(() => sendResponse({ cleared: true }));
    return true;
  }
  return false;
});

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (asOpenAndFill(message)) return handleOpenAndFill(message, sender, sendResponse);
  if (asClearState(message)) {
    if (!isTrustedExternalOrigin(sender.url)) {
      sendResponse({ cleared: false });
      return false;
    }
    armGeneration++;
    void session.clear().then(
      () => sendResponse({ cleared: true }),
      () => sendResponse({ cleared: false }),
    );
    return true;
  }
  const armed = asIsArmed(message);
  if (armed) {
    const tabId = sender.tab?.id;
    if (tabId === undefined) {
      sendResponse({ armed: false });
      return false;
    }
    void session.consume(armed.url, Date.now(), tabId).then((entry) => {
      sendResponse(
        entry?.snapshot
          ? { armed: true, snapshot: entry.snapshot }
          : { armed: entry !== null },
      );
    });
    return true;
  }

  const correction = asFillCorrection(message);
  if (correction) {
    void fetch(TELEMETRY_URL, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(correction.event),
      credentials: "omit",
      keepalive: true,
    }).catch(() => undefined);
    return false;
  }
  return false;
});
