/**
 * Background service worker: routes Open & Fill messages (APP-5). The web app
 * (feed) sends an external `openAndFill` message; we arm the URL and open it.
 * The source page's content script later asks `isArmed` and, if so, fills.
 *
 * This is thin chrome glue over the pure handlers in messaging.ts — it never
 * submits.
 */
import { ArmingStore } from "./arming.js";
import {
  asIsArmed,
  asOpenAndFill,
  handleIsArmed,
  handleOpenAndFill,
} from "./messaging.js";

const store = new ArmingStore();

// From the web app (feed): open the source page and arm autofill on it.
chrome.runtime.onMessageExternal.addListener((message, _sender, sendResponse) => {
  const req = asOpenAndFill(message);
  if (!req) return false;
  const result = handleOpenAndFill(store, req, (url) => {
    void chrome.tabs.create({ url });
  });
  sendResponse(result);
  return true;
});

// From the source page's content script: consume the arm (single-use).
chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  const req = asIsArmed(message);
  if (!req) return false;
  sendResponse(handleIsArmed(store, req));
  return true;
});
