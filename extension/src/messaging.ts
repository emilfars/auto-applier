/**
 * Message contracts + pure handlers for the Open & Fill flow (APP-5). Kept free
 * of any `chrome` reference so they unit-test without a browser.
 */
import type { ProfileData } from "@auto-applier/fill-mappings";
import type { ArmingStore, ArmedEntry, FillSnapshot } from "./arming.js";
import type { FillCorrectionEvent } from "./telemetry.js";

/** Web app → extension: open the source page and arm autofill on it. */
export interface OpenAndFillMessage {
  type: "openAndFill";
  url: string;
  snapshot?: FillSnapshot;
}

/** Content script → background: is the current page armed to fill? */
export interface IsArmedMessage {
  type: "isArmed";
  url: string;
}

export interface ClearStateMessage {
  type: "clearState";
}

export interface FillCorrectionMessage {
  type: "fillCorrection";
  event: FillCorrectionEvent;
}

export type ExtensionMessage =
  | OpenAndFillMessage
  | IsArmedMessage
  | ClearStateMessage
  | FillCorrectionMessage;

export interface OpenAndFillResult {
  armed: boolean;
}

/** Allow only the web app origins that may invoke the external bridge. */
export function isTrustedExternalOrigin(url: string | undefined): boolean {
  if (!url) return false;
  try {
    const origin = new URL(url);
    if (origin.username || origin.password) return false;
    if (
      (origin.protocol === "http:" || origin.protocol === "https:") &&
      origin.hostname === "localhost"
    ) {
      return true;
    }
    return (
      origin.protocol === "https:" &&
      origin.hostname === "autoapplier.id" &&
      origin.port === ""
    );
  } catch {
    return false;
  }
}

export interface IsArmedResult {
  armed: boolean;
  snapshot?: FillSnapshot;
}

/** Narrow the unknown message payload to a typed OpenAndFill request. */
export function asOpenAndFill(msg: unknown): OpenAndFillMessage | null {
  if (
    typeof msg === "object" &&
    msg !== null &&
    (msg as { type?: unknown }).type === "openAndFill" &&
    typeof (msg as { url?: unknown }).url === "string"
  ) {
    const snapshot = (msg as { snapshot?: unknown }).snapshot;
    const base = {
      type: "openAndFill" as const,
      url: (msg as { url: string }).url,
    };
    if (snapshot === undefined) return base;
    const parsedSnapshot = asSnapshot(snapshot);
    return parsedSnapshot ? { ...base, snapshot: parsedSnapshot } : null;
  }

  function asSnapshot(value: unknown): FillSnapshot | null {
    if (typeof value !== "object" || value === null) return null;
    const profile = (value as { profile?: unknown }).profile;
    if (
      typeof profile !== "object" ||
      profile === null ||
      typeof (profile as { confirmed?: unknown }).confirmed !== "boolean"
    ) {
      return null;
    }
    const cv = (value as { cv?: unknown }).cv;
    if (cv === undefined || cv === null) {
      return {
        profile: profile as ProfileData,
        ...(cv === null ? { cv: null } : {}),
      };
    }
    if (
      typeof cv !== "object" ||
      typeof (cv as { name?: unknown }).name !== "string" ||
      typeof (cv as { type?: unknown }).type !== "string" ||
      typeof (cv as { bytesBase64?: unknown }).bytesBase64 !== "string" ||
      (cv as { bytesBase64: string }).bytesBase64.length > 8_000_000
    ) {
      return null;
    }
    return {
      profile: profile as ProfileData,
      cv: {
        name: (cv as { name: string }).name,
        type: (cv as { type: string }).type,
        bytesBase64: (cv as { bytesBase64: string }).bytesBase64,
      },
    };
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

/** Narrow a request that clears ephemeral one-shot state. */
export function asClearState(msg: unknown): ClearStateMessage | null {
  return typeof msg === "object" &&
    msg !== null &&
    (msg as { type?: unknown }).type === "clearState"
    ? { type: "clearState" }
    : null;
}

/** Narrow the metadata-only correction event sent by the content script. */
export function asFillCorrection(msg: unknown): FillCorrectionMessage | null {
  if (
    typeof msg !== "object" ||
    msg === null ||
    (msg as { type?: unknown }).type !== "fillCorrection"
  ) {
    return null;
  }
  const event = (msg as { event?: unknown }).event;
  if (
    typeof event !== "object" ||
    event === null ||
    (event as { type?: unknown }).type !== "fill_correction"
  ) {
    return null;
  }
  return { type: "fillCorrection", event: event as FillCorrectionEvent };
}

/** Opens a tab at the given URL (injected so handlers stay chrome-free). */
export type OpenTab = (url: string) => void;

/** Handle a pure Open & Fill request. */
export function handleOpenAndFill(
  store: ArmingStore,
  msg: OpenAndFillMessage,
  openTab: OpenTab,
  now: number = Date.now(),
): OpenAndFillResult {
  store.arm(msg.url, now, undefined, msg.snapshot);
  openTab(msg.url);
  return { armed: true };
}

/** Handle an IsArmed query: consume the arm (single-use) and report it. */
export function handleIsArmed(
  store: ArmingStore,
  msg: IsArmedMessage,
  now: number = Date.now(),
): IsArmedResult {
  const entry: ArmedEntry | null = store.consumeEntry(msg.url, now);
  return entry?.snapshot
    ? { armed: true, snapshot: entry.snapshot }
    : { armed: entry !== null };
}

/** Type-only assertion that snapshots always carry the canonical profile type. */
export type CanonicalProfile = ProfileData;
