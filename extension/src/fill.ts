/**
 * Fill orchestrator: the single entry point a content script calls to autofill
 * the current page. It routes the host to a portal map, asks the shared engine
 * for a plan, and applies it. It never submits — see apply.ts / safety.test.ts.
 */
import {
  buildFillPlan,
  getMapForHost,
  type ProfileData,
} from "@auto-applier/fill-mappings";
import { applyPlan, type ApplyOptions, type ApplyReport } from "./apply.js";

export interface RunFillResult extends ApplyReport {
  host: string;
}

/**
 * Autofill `doc` for the page at `loc` using the confirmed `profile`. An
 * unconfirmed profile yields an all-empty plan (the engine enforces AC-CV-5), so
 * nothing is written until the user has confirmed their CV data.
 */
export function runFill(
  doc: Document,
  loc: Pick<Location, "hostname">,
  profile: ProfileData,
  opts: ApplyOptions = {},
): RunFillResult {
  const host = loc.hostname.toLowerCase();
  const portal = getMapForHost(host);
  const plan = buildFillPlan(portal, profile, doc);
  const report = applyPlan(plan, doc, opts);
  return { host, ...report };
}
