/**
 * The fill engine. Given a portal map, a confirmed profile, and a query root, it
 * produces a FillPlan describing what value each field should receive and how
 * confident the match is. It performs NO writes and NO submits — it only reads
 * the DOM and decides. Writing is the consumer's job and must never submit.
 */
import type { FillElement, QueryRoot } from "./dom.js";
import { hasExistingValue } from "./dom.js";
import type {
  FieldMapping,
  FieldOutcome,
  FillPlan,
  PortalMap,
  ProfileData,
} from "./types.js";

/**
 * Fields matched only by a fallback selector, or whose value we cannot fully
 * trust, are flagged `uncertain` when confidence is below this threshold. Such
 * fields are surfaced for user review and never reported as `filled`.
 */
export const UNCERTAIN_BELOW = 0.6;

/** Confidence multiplier applied per fallback-selector step past the first. */
const FALLBACK_DECAY = 0.75;

/** Resolve a single mapping against the root. */
function resolveField(
  map: FieldMapping,
  profile: ProfileData,
  root: QueryRoot,
): FieldOutcome {
  const raw = profile[map.key];

  // Locate the field, tracking which selector matched (for confidence decay).
  let el: FillElement | null = null;
  let matchedSelector: string | undefined;
  let decayed = map.confidence;
  for (let i = 0; i < map.selectors.length; i++) {
    const sel = map.selectors[i];
    if (sel === undefined) continue;
    const found = root.querySelector(sel);
    if (found) {
      el = found;
      matchedSelector = sel;
      decayed = map.confidence * Math.pow(FALLBACK_DECAY, i);
      break;
    }
  }

  const base: FieldOutcome = { key: map.key, state: "empty", confidence: 0 };
  if (map.file) base.file = true;

  // No element on the page → nothing to fill.
  if (!el || matchedSelector === undefined) {
    return { ...base, note: "no matching field on page" };
  }
  if (el.disabled) {
    return { ...base, selector: matchedSelector, note: "field disabled" };
  }

  // File-attach targets: we can't read a value; presence of the input + a CV on
  // the profile makes this a plannable (but review-worthy) attach.
  if (map.file) {
    if (!raw) {
      return { ...base, selector: matchedSelector, note: "no CV on profile" };
    }
    const confidence = decayed;
    return {
      key: map.key,
      state: confidence < UNCERTAIN_BELOW ? "uncertain" : "filled",
      confidence,
      value: raw,
      selector: matchedSelector,
      file: true,
      ...(confidence < UNCERTAIN_BELOW
        ? { note: "verify the correct CV attached" }
        : {}),
    };
  }

  // No profile data for this key → empty (but the field exists).
  if (raw === undefined || raw.trim() === "") {
    return { ...base, selector: matchedSelector, note: "no profile value" };
  }

  const value = map.transform ? map.transform(raw) : raw;

  // Never overwrite a value the user already typed; flag for review instead.
  if (hasExistingValue(el)) {
    return {
      key: map.key,
      state: "uncertain",
      confidence: Math.min(decayed, UNCERTAIN_BELOW - 0.01),
      value,
      selector: matchedSelector,
      note: "field already has a value; review before overwriting",
    };
  }

  const state = decayed < UNCERTAIN_BELOW ? "uncertain" : "filled";
  return {
    key: map.key,
    state,
    confidence: decayed,
    value,
    selector: matchedSelector,
    ...(state === "uncertain" ? { note: "low-confidence match; review" } : {}),
  };
}

/**
 * Build the fill plan for a page. `armed` guards the confirm-before-apply gate
 * (AC-CV-5): an unconfirmed profile yields an empty plan so the flow cannot arm.
 */
export function buildFillPlan(
  portal: PortalMap,
  profile: ProfileData,
  root: QueryRoot,
): FillPlan {
  const armed = profile.confirmed === true;
  const outcomes: FieldOutcome[] = [];

  for (const field of portal.fields) {
    if (!armed) {
      outcomes.push({
        key: field.key,
        state: "empty",
        confidence: 0,
        ...(field.file ? { file: true } : {}),
        note: "profile not confirmed; fill not armed",
      });
      continue;
    }
    outcomes.push(resolveField(field, profile, root));
  }

  let filled = 0;
  let uncertain = 0;
  let empty = 0;
  for (const o of outcomes) {
    if (o.state === "filled") filled++;
    else if (o.state === "uncertain") uncertain++;
    else empty++;
  }
  return {
    portalId: portal.id,
    version: portal.version,
    outcomes,
    filled,
    uncertain,
    empty,
  };
}
