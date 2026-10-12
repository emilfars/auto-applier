import type { ReactNode } from "react";

export type MarkerState = "success" | "warning" | "danger" | "neutral";

/**
 * The product's identity motif (DESIGN.md): a small square in a state color
 * followed by words. The words carry the meaning; the square only repeats it.
 */
export function StateMarker({ state, children }: { state: MarkerState; children: ReactNode }) {
  return (
    <span className="state-marker" data-state={state}>
      <span className="state-marker-dot" aria-hidden="true" />
      <span>{children}</span>
    </span>
  );
}
