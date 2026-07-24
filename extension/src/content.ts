/**
 * Content-script orchestration for Open & Fill (APP-5). On a supported source
 * page it asks the background whether this page was armed via Open & Fill; only
 * then does it load the confirmed profile and run the fill. It never submits.
 */
import type { ProfileData } from "@auto-applier/fill-mappings";
import { runFill, type RunFillResult } from "./fill.js";
import type { IsArmedMessage, IsArmedResult } from "./messaging.js";
import type { ApplyOptions } from "./apply.js";

/** Sends a message to the background and resolves its reply. */
export type SendMessage = (msg: IsArmedMessage) => Promise<IsArmedResult>;

/** Loads the user's confirmed profile (+ optional CV file) on demand. */
export type LoadProfile = () => Promise<{
  profile: ProfileData;
  file?: File | null;
}>;

export type ContentFillResult =
  | { ran: false; reason: "not-armed" | "profile-unconfirmed" }
  | { ran: true; report: RunFillResult };

/**
 * Run the Open & Fill flow for the current page. Returns without filling if the
 * page was not armed, or if the profile is not confirmed (defense in depth — the
 * engine also enforces the confirm gate, AC-CV-5).
 */
export async function runContentFill(
  send: SendMessage,
  doc: Document,
  loc: Pick<Location, "hostname" | "href">,
  loadProfile: LoadProfile,
): Promise<ContentFillResult> {
  const { armed } = await send({ type: "isArmed", url: loc.href });
  if (!armed) return { ran: false, reason: "not-armed" };

  const { profile, file } = await loadProfile();
  if (profile.confirmed !== true) {
    return { ran: false, reason: "profile-unconfirmed" };
  }

  const opts: ApplyOptions = file ? { file } : {};
  const report = runFill(doc, loc, profile, opts);
  return { ran: true, report };
}
