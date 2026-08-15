/**
 * The DOM applier. It takes a FillPlan (decided by the shared fill engine) and
 * writes values into the page, highlighting each field by state and attaching
 * the CV to file inputs.
 *
 * PRIME DIRECTIVE: this module never submits. It only sets input values and
 * dispatches `input`/`change` so site frameworks register the change — it never
 * constructs or dispatches a submit event, never invokes the form submit or
 * request-submit DOM APIs, and never programmatically activates a control. A
 * static scan (safety.test.ts) enforces this.
 */
import type { FillPlan } from "@auto-applier/fill-mappings";
import { watchForCorrection, type TelemetrySink } from "./telemetry.js";

/** Marker attribute added to touched fields for the review UI. */
export const FILL_ATTR = "data-aa-fill";

/** Outcome of applying a single field to the DOM. */
export interface AppliedField {
  key: string;
  state: "filled" | "uncertain" | "empty";
  selector?: string;
  applied: boolean;
  note?: string;
}

/** Summary of a fill application. `coverage` counts only successful DOM writes. */
export interface ApplyReport {
  portalId: string;
  version: string;
  applied: number;
  uncertain: number;
  skipped: number;
  fields: AppliedField[];
  /** Successfully written fields divided by mapped controls present in the plan. */
  coverage: number;
  /** Invariant: the extension never submits. Always false. */
  submitted: false;
}

export interface ApplyOptions {
  /** The CV file to attach to file-upload fields, when available. */
  file?: File | null;
  /**
   * Optional sink for fill-correction telemetry. When provided, each field
   * filled with high confidence is watched; if the user later changes it, a
   * privacy-safe `fill_correction` event is emitted (no field value / PII).
   */
  onCorrection?: TelemetrySink;
}

/** A queryable root: a Document or an Element. */
type Root = Pick<Document, "querySelector">;

/**
 * Set a control's value in a way React/Vue-controlled inputs observe: use the
 * native value setter, then dispatch `input` and `change`. Deliberately no
 * `submit` event is ever dispatched.
 */
function setControlValue(el: Element, value: string): void {
  const proto =
    el instanceof HTMLTextAreaElement
      ? HTMLTextAreaElement.prototype
      : HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(proto, "value")?.set;
  if (setter) setter.call(el, value);
  else (el as HTMLInputElement).value = value;
  el.dispatchEvent(new Event("input", { bubbles: true }));
  el.dispatchEvent(new Event("change", { bubbles: true }));
}

/** Attach a File to a file input via DataTransfer, then dispatch `change`. */
function attachFile(el: HTMLInputElement, file: File): boolean {
  if (typeof DataTransfer === "undefined") return false;
  const dt = new DataTransfer();
  dt.items.add(file);
  el.files = dt.files;
  el.dispatchEvent(new Event("input", { bubbles: true }));
  el.dispatchEvent(new Event("change", { bubbles: true }));
  return true;
}

/** True when a text/textarea control already holds a non-empty value. */
function hasTypedValue(el: Element): boolean {
  if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement) {
    return el.value.trim() !== "";
  }
  return false;
}

/** Mark a field for the review UI (filled=green, uncertain=amber, empty=gray). */
function highlight(el: Element, state: "filled" | "uncertain" | "empty"): void {
  el.setAttribute(FILL_ATTR, state);
  const color =
    state === "filled" ? "#16a34a" : state === "uncertain" ? "#d97706" : "#6b7280";
  if (el instanceof HTMLElement) {
    el.style.outline = `2px solid ${color}`;
    el.style.outlineOffset = "1px";
  }
}

/**
 * Apply a fill plan to the DOM. Returns a report of what was touched. Fields the
 * engine marked `empty` are skipped; uncertain text fields may be written but
 * remain flagged for review, while unsafe targets are skipped. Nothing here can
 * submit the form.
 */
export function applyPlan(
  plan: FillPlan,
  root: Root,
  opts: ApplyOptions = {},
): ApplyReport {
  const fields: AppliedField[] = [];
  let applied = 0;
  let uncertain = 0;
  let skipped = 0;

  for (const o of plan.outcomes) {
    const rec: AppliedField = {
      key: o.key,
      state: o.state,
      applied: false,
      ...(o.selector ? { selector: o.selector } : {}),
      ...(o.note ? { note: o.note } : {}),
    };

    if (!o.selector) {
      skipped++;
      fields.push(rec);
      continue;
    }

    const el = root.querySelector(o.selector);
    if (!el) {
      skipped++;
      rec.applied = false;
      rec.note = "element vanished before fill";
      fields.push(rec);
      continue;
    }

    if (o.state === "empty") {
      highlight(el, "empty");
      skipped++;
      fields.push(rec);
      continue;
    }
    if (o.value === undefined) {
      skipped++;
      fields.push(rec);
      continue;
    }
    const value = o.value;

    let didApply = false;
    if (o.file) {
      if (o.state === "filled" && opts.file && el instanceof HTMLInputElement) {
        didApply = attachFile(el, opts.file);
      } else {
        highlight(el, "uncertain");
        rec.state = "uncertain";
        rec.note = "file target requires review before attachment";
        uncertain++;
        fields.push(rec);
        continue;
      }
    } else if (hasTypedValue(el)) {
      // Never overwrite a value the user already typed: highlight for review
      // and leave the existing content untouched.
      highlight(el, "uncertain");
      rec.state = "uncertain";
      rec.applied = false;
      rec.note = "kept existing value; review before overwriting";
      uncertain++;
      fields.push(rec);
      continue;
    } else {
      setControlValue(el, value);
      didApply = true;
    }

    if (didApply) {
      highlight(el, o.state === "uncertain" ? "uncertain" : "filled");
      rec.applied = true;
      if (o.state === "uncertain") uncertain++;
      else applied++;

      // Watch high-confidence text fills for user corrections (AC-APP-TEL).
      if (
        opts.onCorrection &&
        o.state === "filled" &&
        !o.file &&
        (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement)
      ) {
        watchForCorrection(
          el,
          value,
          {
            portalId: plan.portalId,
            version: plan.version,
            key: o.key,
            ...(o.selector ? { selector: o.selector } : {}),
          },
          opts.onCorrection,
        );
      }
    } else {
      skipped++;
      rec.applied = false;
    }
    fields.push(rec);
  }

  const mappable = fields.filter((field) => field.selector !== undefined).length;
  const appliedFields = fields.filter((field) => field.applied).length;

  return {
    portalId: plan.portalId,
    version: plan.version,
    applied,
    uncertain,
    skipped,
    fields,
    coverage: mappable === 0 ? 0 : appliedFields / mappable,
    submitted: false,
  };
}
