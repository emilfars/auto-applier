// Open & Fill orchestration for the web feed (APP-5).
//
// PRIME DIRECTIVE: the system never submits an application. This flow only
// arms the browser extension to *autofill* the posting; the user reviews every
// field and clicks Apply themselves. Nothing here (or in the extension) submits.

import { ApiError } from "./http";
import { checkArm } from "./profile";

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
  sendToExtension: (url: string) => Promise<boolean | null>;
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

  const armed = await deps.sendToExtension(url);
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

/**
 * Real extension bridge: posts an `openAndFill` message to the extension via
 * externally_connectable. Resolves the armed flag, or `null` when the extension
 * isn't installed / reachable.
 */
export function sendToExtension(url: string): Promise<boolean | null> {
  const runtime = chromeRuntime();
  if (!runtime || !EXTENSION_ID) return Promise.resolve(null);

  return new Promise((resolve) => {
    try {
      runtime.sendMessage(EXTENSION_ID, { type: "openAndFill", url }, (response) => {
        if (runtime.lastError) {
          resolve(null);
          return;
        }
        const armed = (response as { armed?: unknown } | undefined)?.armed;
        resolve(typeof armed === "boolean" ? armed : false);
      });
    } catch {
      resolve(null);
    }
  });
}

/** Default dependencies wiring the real server + extension. */
export const openFillDeps: OpenFillDeps = {
  arm: async () => (await checkArm()).can_arm,
  sendToExtension,
  openTab: (url) => {
    window.open(url, "_blank", "noopener,noreferrer");
  },
};
