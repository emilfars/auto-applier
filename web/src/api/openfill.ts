// Open & Fill orchestration for the web feed (APP-5).
//
// PRIME DIRECTIVE: the system never submits an application. This flow only
// arms the browser extension to *autofill* the posting; the user reviews every
// field and clicks Apply themselves. Nothing here (or in the extension) submits.

import { ApiError } from "./http";
import { checkArm, getFillSnapshot, type FillSnapshot } from "./profile";

export type OpenFillStatus =
  | "armed"
  | "noExtension"
  | "needLogin"
  | "needProfile"
  | "error";

export interface OpenFillResult {
  status: OpenFillStatus;
}

export interface OpenFillDeps {
  /** Server-side gate: resolves whether the profile is confirmed (can_arm). */
  arm: () => Promise<boolean>;
  /**
   * Ask the extension to arm + open the posting. Resolves the armed flag, or
   * `null` when no Auto Applier extension is reachable.
   */
  sendToExtension: (url: string, snapshot: FillSnapshot) => Promise<boolean | null>;
  /** Loads the confirmed profile + selected/latest uploaded CV bytes. */
  snapshot: () => Promise<FillSnapshot>;
  /** Open the posting in a new tab (used as a fallback when no extension). */
  openTab: (url: string) => void;
}

/**
 * Arm autofill for a posting and hand off to the extension. The UI already
 * gates the button on login + confirmed profile; the server `arm` check here is
 * defense-in-depth. Never submits — the extension only fills.
 */
export async function requestOpenAndFill(
  url: string,
  deps: OpenFillDeps,
): Promise<OpenFillResult> {
  try {
    const canArm = await deps.arm();
    if (!canArm) return { status: "needProfile" };
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) return { status: "needLogin" };
    if (err instanceof ApiError && err.status === 403) return { status: "needProfile" };
    return { status: "error" };
  }

  let snapshot: FillSnapshot;
  try {
    snapshot = await deps.snapshot();
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) return { status: "needLogin" };
    if (err instanceof ApiError && err.status === 403) return { status: "needProfile" };
    return { status: "error" };
  }
  const armed = await deps.sendToExtension(url, snapshot);
  if (armed === null) {
    // No extension installed: still let the user reach the posting to apply
    // manually. We never auto-open a submit.
    deps.openTab(url);
    return { status: "noExtension" };
  }
  return { status: armed ? "armed" : "error" };
}

/** The extension id the web app may message (set at build time for the demo). */
const EXTENSION_ID = import.meta.env.VITE_EXTENSION_ID as string | undefined;

interface ExternalRuntime {
  sendMessage: (
    id: string,
    message: unknown,
    callback: (response: unknown) => void,
  ) => void;
  lastError?: { message?: string };
}

function chromeRuntime(): ExternalRuntime | null {
  const c = (globalThis as { chrome?: { runtime?: ExternalRuntime } }).chrome;
  if (c?.runtime && typeof c.runtime.sendMessage === "function") return c.runtime;
  return null;
}

const WEB_BRIDGE_SOURCE = "auto-applier-web";
const WEB_BRIDGE_RESPONSE = "openAndFillResult";
const CLEAR_STATE_RESPONSE = "clearStateResult";

function sendViaPageBridge(
  url: string,
  snapshot: FillSnapshot,
): Promise<boolean | null> {
  if (typeof window === "undefined") return Promise.resolve(null);
  const requestId = `${Date.now()}-${Math.random()}`;
  const origin = window.location.origin;
  return new Promise((resolve) => {
    const finish = (value: boolean | null) => {
      window.clearTimeout(timeout);
      window.removeEventListener("message", onMessage);
      resolve(value);
    };
    const onMessage = (event: MessageEvent<unknown>) => {
      if (event.source !== window || event.origin !== origin) return;
      const data = event.data as {
        source?: unknown;
        type?: unknown;
        requestId?: unknown;
        armed?: unknown;
      };
      if (
        data?.source !== WEB_BRIDGE_SOURCE ||
        data.type !== WEB_BRIDGE_RESPONSE ||
        data.requestId !== requestId
      ) {
        return;
      }

      finish(typeof data.armed === "boolean" ? data.armed : false);
    };
    const timeout = window.setTimeout(() => finish(null), 3_000);
    window.addEventListener("message", onMessage);
    window.postMessage(
      {
        source: WEB_BRIDGE_SOURCE,
        type: "openAndFill",
        requestId,
        url,
        snapshot: toExtensionSnapshot(snapshot),
      },
      origin,
    );
  });
}

function clearViaPageBridge(): Promise<boolean> {
  if (typeof window === "undefined") return Promise.resolve(false);
  const requestId = `${Date.now()}-${Math.random()}`;
  const origin = window.location.origin;
  return new Promise((resolve) => {
    const finish = (value: boolean) => {
      window.clearTimeout(timeout);
      window.removeEventListener("message", onMessage);
      resolve(value);
    };
    const onMessage = (event: MessageEvent<unknown>) => {
      if (event.source !== window || event.origin !== origin) return;
      const data = event.data as {
        source?: unknown;
        type?: unknown;
        requestId?: unknown;
        cleared?: unknown;
      };
      if (
        data?.source !== WEB_BRIDGE_SOURCE ||
        data.type !== CLEAR_STATE_RESPONSE ||
        data.requestId !== requestId ||
        typeof data.cleared !== "boolean"
      ) {
        return;
      }
      finish(data.cleared);
    };
    const timeout = window.setTimeout(() => finish(false), 3_000);
    window.addEventListener("message", onMessage);
    window.postMessage(
      {
        source: WEB_BRIDGE_SOURCE,
        type: "clearState",
        requestId,
      },
      origin,
    );
  });
}

/**
 * Real extension bridge: posts an `openAndFill` message to the extension via
 * externally_connectable when Chrome exposes that API to the page, or through
 * the extension's page content-script bridge. Resolves the armed flag, or `null`
 * when no Auto Applier extension is installed/reachable.
 */
export function sendToExtension(
  url: string,
  snapshot: FillSnapshot,
): Promise<boolean | null> {
  const runtime = chromeRuntime();
  if (!runtime || !EXTENSION_ID) return sendViaPageBridge(url, snapshot);

  return new Promise((resolve) => {
    try {
      runtime.sendMessage(
        EXTENSION_ID,
        { type: "openAndFill", url, snapshot: toExtensionSnapshot(snapshot) },
        (response) => {
          if (runtime.lastError) {
            resolve(null);
            return;
          }
          const armed = (response as { armed?: unknown } | undefined)?.armed;
          resolve(typeof armed === "boolean" ? armed : false);
        },
      );
    } catch {
      resolve(null);
    }

  });
}

function toExtensionSnapshot(snapshot: FillSnapshot) {
  return {
    profile: snapshot.profile,
    cv: snapshot.cv
      ? {
          name: snapshot.cv.filename,
          type: snapshot.cv.content_type,
          bytesBase64: snapshot.cv.bytes_base64,
        }
      : null,
  };
}

/** Clear any unconsumed ephemeral extension snapshot on logout. */
export function clearExtensionState(): Promise<boolean> {
  const runtime = chromeRuntime();
  if (!runtime || !EXTENSION_ID) return clearViaPageBridge();
  return new Promise((resolve) => {
    const fallback = () => {
      void clearViaPageBridge().then(resolve);
    };
    try {
      runtime.sendMessage(EXTENSION_ID, { type: "clearState" }, (response) => {
        if (runtime.lastError) {
          fallback();
          return;
        }
        resolve(
          (response as { cleared?: unknown } | undefined)?.cleared === true,
        );
      });
    } catch {
      fallback();
    }
  });
}

/** Default dependencies wiring the real server + extension. */
export const openFillDeps: OpenFillDeps = {
  arm: async () => (await checkArm()).can_arm,
  snapshot: () => getFillSnapshot(),
  sendToExtension,
  openTab: (url) => {
    window.open(url, "_blank", "noopener,noreferrer");
  },
};
