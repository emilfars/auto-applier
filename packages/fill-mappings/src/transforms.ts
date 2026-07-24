/** Value transforms applied before a field is filled. Pure and reversible-free. */

/** Keep digits only — for numeric salary inputs that reject formatting. */
export function digitsOnly(value: string): string {
  return value.replace(/\D+/g, "");
}

/** Collapse whitespace and trim. */
export function tidy(value: string): string {
  return value.replace(/\s+/g, " ").trim();
}
