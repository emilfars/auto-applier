/**
 * Minimal structural DOM abstraction so the fill engine stays framework- and
 * environment-agnostic: the Chrome extension passes the real `document`, the
 * Android WebView passes its injected document, and tests pass a tiny fake.
 * Only the read/query surface the engine needs is modeled here — deliberately
 * NOT any submit/click capability (Prime Directive).
 */

/** The subset of an element the engine reads to decide field state. */
export interface FillElement {
  /** Uppercase tag name, e.g. "INPUT", "SELECT", "TEXTAREA". */
  readonly tagName: string;
  /** Current value of the control, if any. */
  readonly value?: string;
  /** Whether the control is disabled (never fill). */
  readonly disabled?: boolean;
  getAttribute(name: string): string | null;
}

/** The subset of a query root (document/element) the engine uses. */
export interface QueryRoot {
  querySelector(selectors: string): FillElement | null;
}

/** True when an element already holds a non-empty value. */
export function hasExistingValue(el: FillElement): boolean {
  return typeof el.value === "string" && el.value.trim() !== "";
}
