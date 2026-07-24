/**
 * Fill-correction telemetry. When the extension fills a field with high
 * confidence (`filled`) and the user then changes that value, that is a signal
 * the mapping guessed wrong — the core "wrong field filled" risk. We emit a
 * privacy-safe event (portal, field key, selector — never the value or any PII)
 * so mappings can be improved.
 *
 * PRIME DIRECTIVE: this module only observes. It attaches change listeners and
 * calls a sink; it never writes values and never submits.
 */

/** A single fill-correction signal. Carries NO field value / PII. */
export interface FillCorrectionEvent {
  type: "fill_correction";
  portalId: string;
  version: string;
  /** Canonical profile key that was mis-filled. */
  key: string;
  selector?: string;
  /** Epoch millis when the correction was observed. */
  at: number;
}

/** Where correction events are delivered. */
export type TelemetrySink = (event: FillCorrectionEvent) => void;

/** Metadata identifying which mapping produced the field. */
export interface CorrectionMeta {
  portalId: string;
  version: string;
  key: string;
  selector?: string;
}

/** Injectable clock (defaults to Date.now) so tests are deterministic. */
export type Clock = () => number;

/**
 * Watch a field we just filled with `filledValue`. If the user changes it to a
 * different value, emit exactly one correction event to `sink`. Returns a detach
 * function that removes the listeners.
 */
export function watchForCorrection(
  el: EventTarget & { value?: string },
  filledValue: string,
  meta: CorrectionMeta,
  sink: TelemetrySink,
  clock: Clock = Date.now,
): () => void {
  let emitted = false;

  const handler = (): void => {
    if (emitted) return;
    const current = typeof el.value === "string" ? el.value : "";
    if (current === filledValue) return;
    emitted = true;
    sink({
      type: "fill_correction",
      portalId: meta.portalId,
      version: meta.version,
      key: meta.key,
      ...(meta.selector ? { selector: meta.selector } : {}),
      at: clock(),
    });
    detach();
  };

  el.addEventListener("input", handler);
  el.addEventListener("change", handler);

  function detach(): void {
    el.removeEventListener("input", handler);
    el.removeEventListener("change", handler);
  }

  return detach;
}
