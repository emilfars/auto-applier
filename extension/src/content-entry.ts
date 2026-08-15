/**
 * Content-script entry: injected into supported source pages. It binds the pure
 * `runContentFill` orchestrator to the chrome runtime (message the background,
 * load the confirmed profile from storage) and runs after the DOM is ready.
 *
 * It never submits — filling is delegated to the shared engine + applier.
 */
import { renderReviewSummary, runContentFill, type LoadProfile } from "./content.js";
import {
  asOpenAndFill,
  asClearState,
  isTrustedExternalOrigin,
  type IsArmedMessage,
  type IsArmedResult,
} from "./messaging.js";

const WEB_BRIDGE_SOURCE = "auto-applier-web";
const WEB_BRIDGE_RESPONSE = "openAndFillResult";
const CLEAR_STATE_RESPONSE = "clearStateResult";

function installWebBridge(): void {
  if (!isTrustedExternalOrigin(window.location.href)) return;
  window.addEventListener("message", (event) => {
    if (event.source !== window || event.origin !== window.location.origin) return;
    const data = event.data as { source?: unknown; type?: unknown; requestId?: unknown };
    if (
      data?.source !== WEB_BRIDGE_SOURCE ||
      typeof data.requestId !== "string" ||
      data.requestId.length === 0
    ) {
      return;
    }
    const respond = (type: string, value: boolean) => {
      window.postMessage(
        {
          source: WEB_BRIDGE_SOURCE,
          type,
          requestId: data.requestId,
          ...(type === WEB_BRIDGE_RESPONSE ? { armed: value } : { cleared: value }),
        },
        event.origin,
      );
    };
    const open = data.type === "openAndFill" ? asOpenAndFill(event.data) : null;
    const clear = data.type === "clearState" ? asClearState(event.data) : null;
    if (!open && !clear) return;
    try {
      chrome.runtime.sendMessage(open ?? clear, (response) => {
        const key = open ? "armed" : "cleared";
        respond(
          open ? WEB_BRIDGE_RESPONSE : CLEAR_STATE_RESPONSE,
          chrome.runtime.lastError === undefined &&
            (response as { armed?: unknown; cleared?: unknown } | undefined)?.[key] === true,
        );
      });
    } catch {
      respond(open ? WEB_BRIDGE_RESPONSE : CLEAR_STATE_RESPONSE, false);
    }
  });
}

/** Ask the background whether this page was armed via Open & Fill. */
async function send(msg: IsArmedMessage): Promise<IsArmedResult> {
  const reply = (await chrome.runtime.sendMessage(msg)) as
    | IsArmedResult
    | undefined;
  return reply ?? { armed: false };
}

/** Load the confirmed profile (and CV bytes) the user reviewed in the web app. */
const loadProfile: LoadProfile = async () => {
  return { profile: {} };
};

async function main(): Promise<void> {
  const result = await runContentFill(
    send,
    document,
    window.location,
    loadProfile,
    {
      onCorrection: (event) => {
        void chrome.runtime
          .sendMessage({ type: "fillCorrection", event })
          .catch(() => undefined);
      },
    },
  );
  if (result.ran) renderReviewSummary(document, result.report);
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", () => void main());
} else {
  void main();
}

installWebBridge();
