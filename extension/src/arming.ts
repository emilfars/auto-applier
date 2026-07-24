/**
 * The "Open & Fill" arming store (APP-5). When the user clicks Open & Fill on a
 * feed card, the web app asks the extension to open the job's source page and
 * autofill it. We record that URL as "armed" for a short window; when the source
 * page's content script loads, it consumes the arm exactly once and runs the
 * fill. This keeps autofill opt-in per action — the extension does not fill
 * arbitrary pages the user merely browses to.
 *
 * PRIME DIRECTIVE unaffected: arming only gates whether we *fill*; it never
 * submits.
 */

/** Default time an arm stays valid before it expires (ms). */
export const ARM_TTL_MS = 60_000;

/**
 * Normalize a URL for matching: scheme + host + path only. Query and hash are
 * dropped so a redirect that appends tracking params still matches, and a
 * trailing slash is ignored. Invalid URLs return the trimmed input.
 */
export function normalizeUrl(url: string): string {
  try {
    const u = new URL(url);
    const path = u.pathname.replace(/\/+$/, "");
    return `${u.protocol}//${u.host}${path}`.toLowerCase();
  } catch {
    return url.trim().toLowerCase();
  }
}

/** In-memory store of armed URLs with expiry. Single-consume per arm. */
export class ArmingStore {
  private readonly armed = new Map<string, number>();

  constructor(private readonly ttlMs: number = ARM_TTL_MS) {}

  /** Arm `url` starting at `now`. */
  arm(url: string, now: number = Date.now()): void {
    this.armed.set(normalizeUrl(url), now + this.ttlMs);
  }

  /**
   * If `url` is currently armed, consume it (so it can't be reused) and return
   * true; otherwise false. Expired arms are treated as absent.
   */
  consume(url: string, now: number = Date.now()): boolean {
    const key = normalizeUrl(url);
    const expiresAt = this.armed.get(key);
    if (expiresAt === undefined) return false;
    this.armed.delete(key);
    return now <= expiresAt;
  }

  /** Non-destructive check, mainly for tests/telemetry. */
  isArmed(url: string, now: number = Date.now()): boolean {
    const expiresAt = this.armed.get(normalizeUrl(url));
    return expiresAt !== undefined && now <= expiresAt;
  }

  /** Drop expired entries so the map can't grow without bound. */
  sweep(now: number = Date.now()): void {
    for (const [k, expiresAt] of this.armed) {
      if (now > expiresAt) this.armed.delete(k);
    }
  }

  /** Number of tracked arms (for tests). */
  size(): number {
    return this.armed.size;
  }
}
