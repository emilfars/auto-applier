/**
 * Message contracts + pure handlers for the Open & Fill flow (APP-5). Kept free
 * of any `chrome` reference so they unit-test without a browser; background.ts
 * binds them to the real chrome runtime.
 */
import type { ArmingStore } from "./arming.js";

/** Web app → extension: open the source page and arm autofill on it. */
export interface OpenAndFillMessage {
  type: "openAndFill";
  url: string;
}

/** Content script → background: is the current page armed to fill? */
export interface IsArmedMessage {
  type: "isArmed";
  url: string;
}

export type ExtensionMessage = OpenAndFillMessage | IsArmedMessage;

export interface OpenAndFillResult {
  armed: boolean;
}
export interface IsArmedResult {
  armed: boolean;
}

/** Narrow the unknown message payload to a typed OpenAndFill request. */
export function asOpenAndFill(msg: unknown): OpenAndFillMessage | null {
  if (
    typeof msg === "object" &&
    msg !== null &&
    (msg as { type?: unknown }).type === "openAndFill" &&
    typeof (msg as { url?: unknown }).url === "string"
  ) {
    return msg as OpenAndFillMessage;
  }
  return null;
}

/** Narrow the unknown message payload to a typed IsArmed request. */
export function asIsArmed(msg: unknown): IsArmedMessage | null {
  if (
    typeof msg === "object" &&
    msg !== null &&
    (msg as { type?: unknown }).type === "isArmed" &&
    typeof (msg as { url?: unknown }).url === "string"
  ) {
    return msg as IsArmedMessage;
  }
  return null;
}

/** Opens a tab at the given URL (injected so handlers stay chrome-free). */
export type OpenTab = (url: string) => void;

/**
 * Handle an Open & Fill request: arm the URL, then open it. Returns the result
 * to send back to the web app.
 */
export function handleOpenAndFill(
  store: ArmingStore,
  msg: OpenAndFillMessage,
  openTab: OpenTab,
  now: number = Date.now(),
): OpenAndFillResult {
  store.arm(msg.url, now);
  openTab(msg.url);
  return { armed: true };
}

/** Handle an IsArmed query: consume the arm (single-use) and report it. */
export function handleIsArmed(
  store: ArmingStore,
  msg: IsArmedMessage,
  now: number = Date.now(),
): IsArmedResult {
  return { armed: store.consume(msg.url, now) };
}
