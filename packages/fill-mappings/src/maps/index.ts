import type { PortalMap } from "../types.js";
import { greenhouse } from "./greenhouse.js";
import { lever } from "./lever.js";
import { workable } from "./workable.js";
import { jobstreet } from "./jobstreet.js";
import { glints } from "./glints.js";
import { kalibrr } from "./kalibrr.js";
import { generic } from "./generic.js";

export { greenhouse, lever, workable, jobstreet, glints, kalibrr, generic };

/** All portal-specific maps (excludes the generic fallback). */
export const portalMaps: PortalMap[] = [
  greenhouse,
  lever,
  workable,
  jobstreet,
  glints,
  kalibrr,
];

/** The generic fallback used when no portal-specific map matches the host. */
export const genericMap: PortalMap = generic;

/**
 * Select the best map for a hostname. Returns the portal-specific map whose
 * host suffix matches, else the generic fallback. Never returns undefined so a
 * page always has a (low-confidence) plan.
 */
export function getMapForHost(hostname: string): PortalMap {
  const host = hostname.toLowerCase();
  for (const map of portalMaps) {
    if (map.hosts.some((h) => host === h || host.endsWith(`.${h}`) || host.endsWith(h))) {
      return map;
    }
  }
  return generic;
}
