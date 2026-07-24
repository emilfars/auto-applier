/**
 * @auto-applier/fill-mappings — the single source of truth for how job
 * application forms are autofilled. Framework-agnostic and dependency-free at
 * runtime so the Chrome extension and the Android WebView share one
 * implementation (a portal fix ships to both).
 *
 * PRIME DIRECTIVE: this package never submits an application. It only decides
 * what to fill and how confident it is; the human always clicks apply.
 */
export type {
  FieldState,
  ProfileKey,
  ProfileData,
  FieldMapping,
  PortalMap,
  FieldOutcome,
  FillPlan,
} from "./types.js";

export type { FillElement, QueryRoot } from "./dom.js";
export { hasExistingValue } from "./dom.js";

export { buildFillPlan, UNCERTAIN_BELOW } from "./engine.js";
export { digitsOnly, tidy } from "./transforms.js";

export {
  portalMaps,
  genericMap,
  getMapForHost,
  greenhouse,
  lever,
  workable,
  jobstreet,
  glints,
  kalibrr,
  generic,
} from "./maps/index.js";
