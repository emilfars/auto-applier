/**
 * Content-script entry: injected into supported source pages. It binds the pure
 * `runContentFill` orchestrator to the chrome runtime (message the background,
 * load the confirmed profile from storage) and runs after the DOM is ready.
 *
 * It never submits — filling is delegated to the shared engine + applier.
 */
import { runContentFill, type LoadProfile } from "./content.js";
import type { IsArmedMessage, IsArmedResult } from "./messaging.js";
import type { ProfileData } from "@auto-applier/fill-mappings";

/** Ask the background whether this page was armed via Open & Fill. */
async function send(msg: IsArmedMessage): Promise<IsArmedResult> {
  const reply = (await chrome.runtime.sendMessage(msg)) as
    | IsArmedResult
    | undefined;
  return reply ?? { armed: false };
}

/** Load the confirmed profile (and CV bytes) the user reviewed in the web app. */
const loadProfile: LoadProfile = async () => {
  const stored = await chrome.storage.local.get(["profile"]);
  const profile = (stored["profile"] as ProfileData | undefined) ?? {};
  return { profile };
};

async function main(): Promise<void> {
  await runContentFill(send, document, window.location, loadProfile);
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", () => void main());
} else {
  void main();
}
