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
import type { ProfileData } from "@auto-applier/fill-mappings";

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

/** Structured-clone-safe CV bytes used at the message/content boundaries. */
export interface SerializedFile {
  name: string;
  type: string;
  bytesBase64: string;
}

export interface FillSnapshot {
  profile: ProfileData;
  cv?: SerializedFile | null;
}

export interface ArmedEntry {
  url: string;
  expiresAt: number;
  tabId?: number;
  snapshot?: FillSnapshot;
}

/** In-memory pure store used by tests and as the storage-state model. */
export class ArmingStore {
  private readonly armed = new Map<string, ArmedEntry>();

  constructor(private readonly ttlMs: number = ARM_TTL_MS) {}

  /** Arm `url` starting at `now`, optionally scoped to a tab and snapshot. */
  arm(
    url: string,
    now: number = Date.now(),
    tabId?: number,
    snapshot?: FillSnapshot,
  ): void {
    this.armed.set(this.key(url, tabId), {
      url: normalizeUrl(url),
      expiresAt: now + this.ttlMs,
      ...(tabId === undefined ? {} : { tabId }),
      ...(snapshot === undefined ? {} : { snapshot }),
    });
  }

  /**
   * Consume a matching arm exactly once. A tab mismatch leaves the arm for the
   * intended tab, while expired entries are removed.
   */
  consumeEntry(
    url: string,
    now: number = Date.now(),
    tabId?: number,
  ): ArmedEntry | null {
    const key = normalizeUrl(url);
    const entry = this.armed.get(this.key(key, tabId));
    if (!entry) return null;
    if (entry.url !== key) return null;
    if (now >= entry.expiresAt) {
      this.armed.delete(this.key(key, tabId));
      return null;
    }
    this.armed.delete(this.key(key, tabId));
    return entry;
  }

  consume(url: string, now: number = Date.now(), tabId?: number): boolean {
    return this.consumeEntry(url, now, tabId) !== null;
  }

  /** Non-destructive check that preserves a valid matching arm. */
  peek(url: string, now: number = Date.now(), tabId?: number): ArmedEntry | null {
    const key = normalizeUrl(url);
    const entry = this.armed.get(this.key(key, tabId));
    if (!entry || entry.url !== key) return null;
    if (now >= entry.expiresAt) {
      this.armed.delete(this.key(key, tabId));
      return null;
    }
    return entry;
  }

  isArmed(url: string, now: number = Date.now(), tabId?: number): boolean {
    return this.peek(url, now, tabId) !== null;
  }

  /** Drop expired entries so the map can't grow without bound. */
  sweep(now: number = Date.now()): void {
    for (const [key, entry] of this.armed) {
      if (now >= entry.expiresAt) this.armed.delete(key);
    }
  }

  clear(tabId?: number): void {
    if (tabId === undefined) {
      this.armed.clear();
      return;
    }
    for (const [key, entry] of this.armed) {
      if (entry.tabId === tabId) this.armed.delete(key);
    }
  }

  /** Number of tracked arms (for tests). */
  size(): number {
    return this.armed.size;
  }

  private key(url: string, tabId?: number): string {
    return tabId === undefined ? `url:${normalizeUrl(url)}` : `tab:${tabId}`;
  }
}

/** Narrow storage surface used so durable arming remains unit-testable. */
export interface SessionStorageLike {
  get(keys: string): Promise<Record<string, unknown>>;
  set(items: Record<string, unknown>): Promise<void>;
  remove(keys: string): Promise<void>;
}

export const ARM_EXPIRY_ALARM = "autoApplier-arm-expiry";

export interface ArmExpiryAlarmScheduler {
  schedule(expiresAt: number): Promise<void>;
  clear(): Promise<void>;
}

export interface AlarmEventSource {
  onAlarm: {
    addListener(listener: (alarm: { name: string }) => void): void;
  };
}

export function installArmExpiryAlarmListener(
  alarms: AlarmEventSource,
  sweep: () => Promise<void>,
): void {
  alarms.onAlarm.addListener((alarm) => {
    if (alarm.name === ARM_EXPIRY_ALARM) void sweep().catch(() => undefined);
  });
}

/** Small binary store boundary so CV bytes never need to enter session metadata. */
export interface BinaryPayloadStore {
  put(bytes: Uint8Array, expiresAt: number): Promise<string>;
  get(id: string): Promise<Uint8Array | null>;
  delete(id: string): Promise<void>;
  sweep(now: number, referencedIds?: ReadonlySet<string>): Promise<void>;
}

interface IndexedPayload {
  id: string;
  bytes: ArrayBuffer;
  expiresAt: number;
}

const BINARY_DB_NAME = "autoApplierCv";
const BINARY_STORE_NAME = "payloads";

/**
 * IndexedDB-backed CV payload store. Payload ids are content hashes, so two
 * simultaneous arms for the same snapshot share one record.
 */
export class IndexedDbBinaryPayloadStore implements BinaryPayloadStore {
  private dbPromise: Promise<IDBDatabase> | undefined;

  constructor(
    private readonly factory: IDBFactory | undefined =
      typeof indexedDB === "undefined" ? undefined : indexedDB,
    private readonly dbName: string = BINARY_DB_NAME,
  ) {}

  async put(bytes: Uint8Array, expiresAt: number): Promise<string> {
    const id = await payloadId(bytes);
    const db = await this.open();
    await new Promise<void>((resolve, reject) => {
      const transaction = db.transaction(BINARY_STORE_NAME, "readwrite");
      const store = transaction.objectStore(BINARY_STORE_NAME);
      const request = store.get(id);
      request.onerror = () => reject(request.error ?? new Error("failed to read CV payload"));
      request.onsuccess = () => {
        const existing = request.result as IndexedPayload | undefined;
        store.put({
          id,
          bytes: existing?.bytes ?? bytes.slice().buffer,
          expiresAt: Math.max(existing?.expiresAt ?? 0, expiresAt),
        } satisfies IndexedPayload);
      };
      transaction.oncomplete = () => resolve();
      transaction.onerror = () =>
        reject(transaction.error ?? new Error("failed to store CV payload"));
      transaction.onabort = () =>
        reject(transaction.error ?? new Error("aborted CV payload store"));
    });
    return id;
  }

  async get(id: string): Promise<Uint8Array | null> {
    const db = await this.open();
    const record = await this.request<IndexedPayload | undefined>(
      db.transaction(BINARY_STORE_NAME, "readonly").objectStore(BINARY_STORE_NAME).get(id),
    );
    return record ? new Uint8Array(record.bytes.slice(0)) : null;
  }

  async delete(id: string): Promise<void> {
    if (!this.factory) return;
    const db = await this.open();
    await this.transaction(db, "readwrite", (store) => {
      store.delete(id);
    });
  }

  async sweep(now: number, referencedIds?: ReadonlySet<string>): Promise<void> {
    if (!this.factory) return;
    const db = await this.open();
    await new Promise<void>((resolve, reject) => {
      const transaction = db.transaction(BINARY_STORE_NAME, "readwrite");
      const request = transaction.objectStore(BINARY_STORE_NAME).openCursor();
      request.onerror = () => reject(request.error ?? new Error("failed to sweep CV payloads"));
      request.onsuccess = () => {
        const cursor = request.result;
        if (!cursor) return;
        const record = cursor.value as IndexedPayload;
        if (
          record.expiresAt <= now ||
          (referencedIds !== undefined && !referencedIds.has(record.id))
        ) {
          cursor.delete();
        }
        cursor.continue();
      };
      transaction.oncomplete = () => resolve();
      transaction.onerror = () =>
        reject(transaction.error ?? new Error("failed to sweep CV payloads"));
      transaction.onabort = () =>
        reject(transaction.error ?? new Error("aborted CV payload sweep"));
    });
  }

  private open(): Promise<IDBDatabase> {
    if (!this.factory) {
      return Promise.reject(new Error("IndexedDB is unavailable"));
    }
    if (this.dbPromise) return this.dbPromise;
    this.dbPromise = new Promise<IDBDatabase>((resolve, reject) => {
      const request = this.factory!.open(this.dbName, 1);
      request.onupgradeneeded = () => {
        request.result.createObjectStore(BINARY_STORE_NAME, { keyPath: "id" });
      };
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error ?? new Error("failed to open CV payload store"));
      request.onblocked = () => reject(new Error("CV payload store is blocked"));
    });
    return this.dbPromise;
  }

  private request<T>(request: IDBRequest<T>): Promise<T> {
    return new Promise<T>((resolve, reject) => {
      request.onsuccess = () => resolve(request.result);
      request.onerror = () => reject(request.error ?? new Error("IndexedDB request failed"));
    });
  }

  private transaction(
    db: IDBDatabase,
    mode: IDBTransactionMode,
    operation: (store: IDBObjectStore) => void,
  ): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      const transaction = db.transaction(BINARY_STORE_NAME, mode);
      operation(transaction.objectStore(BINARY_STORE_NAME));
      transaction.oncomplete = () => resolve();
      transaction.onerror = () =>
        reject(transaction.error ?? new Error("IndexedDB transaction failed"));
      transaction.onabort = () =>
        reject(transaction.error ?? new Error("IndexedDB transaction aborted"));
    });
  }
}

async function payloadId(bytes: Uint8Array): Promise<string> {
  const digest = await globalThis.crypto.subtle.digest(
    "SHA-256",
    bytes as unknown as BufferSource,
  );
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
}

const SESSION_KEY = "autoApplierArms";

interface StoredFileReference {
  name: string;
  type: string;
  payloadId: string;
}

interface StoredFillSnapshot {
  profile: ProfileData;
  cv?: StoredFileReference | null;
}

interface StoredArmedEntry {
  url: string;
  expiresAt: number;
  tabId: number;
  snapshot?: StoredFillSnapshot;
}

/**
 * MV3 worker-safe arming store. Operations are serialized because storage.session
 * has no compare-and-swap primitive; the queue makes consume one-shot across
 * concurrent message events and the persisted entry survives worker suspension.
 */
export class SessionArmingStore {
  private pending: Promise<unknown> = Promise.resolve();

  constructor(
    private readonly storage: SessionStorageLike,
    private readonly ttlMs: number = ARM_TTL_MS,
    private readonly binaryStore: BinaryPayloadStore = new IndexedDbBinaryPayloadStore(),
    private readonly alarmScheduler?: ArmExpiryAlarmScheduler,
  ) {}

  arm(
    url: string,
    now: number = Date.now(),
    tabId: number,
    snapshot?: FillSnapshot,
  ): Promise<void> {
    return this.serial(async () => {
      const state = await this.read();
      const released = this.removeExpired(state, now);
      const expiresAt = now + this.ttlMs;
      const storedSnapshot =
        snapshot === undefined
          ? undefined
          : await this.storeSnapshot(snapshot, expiresAt);
      const key = this.key(tabId);
      const previous = state[key];
      const next: StoredArmedEntry = {
        url: normalizeUrl(url),
        expiresAt,
        tabId,
      };
      if (storedSnapshot !== undefined) next.snapshot = storedSnapshot;
      state[key] = next;
      if (previous?.snapshot?.cv) released.push(previous.snapshot.cv.payloadId);
      await this.storage.set({ [SESSION_KEY]: state });
      await this.cleanup(state, released, now);
      await this.rescheduleAlarm(state);
    });
  }

  consume(
    url: string,
    now: number = Date.now(),
    tabId: number,
  ): Promise<ArmedEntry | null> {
    return this.serial(async () => {
      const state = await this.read();
      const key = this.key(tabId);
      const entry = state[key];
      if (!entry) return null;
      if (entry.url !== normalizeUrl(url)) return null;
      if (now >= entry.expiresAt) {
        delete state[key];
        await this.storage.set({ [SESSION_KEY]: state });
        await this.cleanup(state, this.entryPayloadIds(entry), now);
        await this.rescheduleAlarm(state);
        return null;
      }
      const materialized = await this.materialize(entry);
      delete state[key];
      await this.storage.set({ [SESSION_KEY]: state });
      await this.cleanup(state, this.entryPayloadIds(entry), now);
      await this.rescheduleAlarm(state);
      return materialized;
    });
  }

  peek(url: string, now: number = Date.now(), tabId: number): Promise<ArmedEntry | null> {
    return this.serial(async () => {
      const state = await this.read();
      const key = this.key(tabId);
      const entry = state[key];
      if (!entry) return null;
      if (entry.url !== normalizeUrl(url)) return null;
      if (now >= entry.expiresAt) {
        delete state[key];
        await this.storage.set({ [SESSION_KEY]: state });
        await this.cleanup(state, this.entryPayloadIds(entry), now);
        await this.rescheduleAlarm(state);
        return null;
      }
      const materialized = await this.materialize(entry);
      await this.rescheduleAlarm(state);
      return materialized;
    });
  }

  isArmed(url: string, now: number, tabId: number): Promise<boolean> {
    return this.peek(url, now, tabId).then((entry) => entry !== null);
  }

  sweep(now: number = Date.now()): Promise<void> {
    return this.serial(async () => {
      const state = await this.read();
      const released = this.removeExpired(state, now);
      await this.storage.set({ [SESSION_KEY]: state });
      await this.cleanup(state, released, now);
      await this.rescheduleAlarm(state);
    });
  }

  clear(tabId?: number): Promise<void> {
    return this.serial(async () => {
      if (tabId === undefined) {
        const state = await this.read();
        await this.storage.remove(SESSION_KEY);
        await this.binaryStore.sweep(Date.now(), new Set());
        for (const id of this.payloadIds(state)) await this.binaryStore.delete(id);
        await this.rescheduleAlarm({});
        return;
      }
      const state = await this.read();
      const key = this.key(tabId);
      const released = state[key] ? this.entryPayloadIds(state[key]) : [];
      delete state[key];
      await this.storage.set({ [SESSION_KEY]: state });
      const active = this.payloadIds(state);
      for (const id of new Set(released)) {
        if (!active.has(id)) await this.binaryStore.delete(id);
      }
      await this.rescheduleAlarm(state);
    });
  }

  private async storeSnapshot(
    snapshot: FillSnapshot,
    expiresAt: number,
  ): Promise<StoredFillSnapshot> {
    const stored: StoredFillSnapshot = { profile: snapshot.profile };
    if (snapshot.cv === undefined) return stored;
    if (snapshot.cv === null) {
      stored.cv = null;
      return stored;
    }
    const bytes = decodeBase64(snapshot.cv.bytesBase64);
    stored.cv = {
      name: snapshot.cv.name,
      type: snapshot.cv.type,
      payloadId: await this.binaryStore.put(bytes, expiresAt),
    };
    return stored;
  }

  private async materialize(entry: StoredArmedEntry): Promise<ArmedEntry> {
    const result: ArmedEntry = {
      url: entry.url,
      expiresAt: entry.expiresAt,
      tabId: entry.tabId,
    };
    if (!entry.snapshot) return result;
    const snapshot: FillSnapshot = { profile: entry.snapshot.profile };
    if (entry.snapshot.cv === null) {
      snapshot.cv = null;
    } else if (entry.snapshot.cv) {
      const bytes = await this.binaryStore.get(entry.snapshot.cv.payloadId);
      snapshot.cv = bytes
        ? {
            name: entry.snapshot.cv.name,
            type: entry.snapshot.cv.type,
            bytesBase64: encodeBase64(bytes),
          }
        : null;
    }
    result.snapshot = snapshot;
    return result;
  }

  private removeExpired(
    state: Record<string, StoredArmedEntry>,
    now: number,
  ): string[] {
    const released: string[] = [];
    for (const [key, entry] of Object.entries(state)) {
      if (now >= entry.expiresAt) {
        released.push(...this.entryPayloadIds(entry));
        delete state[key];
      }
    }
    return released;
  }

  private async cleanup(
    state: Record<string, StoredArmedEntry>,
    candidates: Iterable<string>,
    now: number,
  ): Promise<void> {
    const active = this.payloadIds(state);
    for (const id of new Set(candidates)) {
      if (!active.has(id)) await this.binaryStore.delete(id);
    }
    await this.binaryStore.sweep(now, active);
  }

  private async rescheduleAlarm(
    state: Record<string, StoredArmedEntry>,
  ): Promise<void> {
    if (!this.alarmScheduler) return;
    const nextExpiry = Object.values(state).reduce<number | undefined>(
      (next, entry) =>
        next === undefined ? entry.expiresAt : Math.min(next, entry.expiresAt),
      undefined,
    );
    if (nextExpiry === undefined) {
      await this.alarmScheduler.clear();
      return;
    }
    await this.alarmScheduler.schedule(nextExpiry);
  }

  private async read(): Promise<Record<string, StoredArmedEntry>> {
    const stored = await this.storage.get(SESSION_KEY);
    const value = stored[SESSION_KEY];
    if (!value || typeof value !== "object") return {};
    const state: Record<string, StoredArmedEntry> = {};
    for (const [key, entry] of Object.entries(value)) {
      if (
        /^-?\d+$/.test(key) &&
        entry &&
        typeof entry === "object" &&
        typeof (entry as StoredArmedEntry).url === "string" &&
        typeof (entry as StoredArmedEntry).expiresAt === "number" &&
        typeof (entry as StoredArmedEntry).tabId === "number" &&
        (entry as StoredArmedEntry).tabId === Number(key)
      ) {
        const parsed = this.storedEntry(entry as StoredArmedEntry);
        if (parsed) state[key] = parsed;
      }
    }
    return state;
  }

  private storedEntry(entry: StoredArmedEntry): StoredArmedEntry | null {
    if (!entry.snapshot) return entry;
    const snapshot = entry.snapshot as StoredFillSnapshot;
    if (!snapshot.profile || typeof snapshot.profile !== "object") return null;
    if (snapshot.cv === undefined || snapshot.cv === null) return entry;
    if (
      typeof snapshot.cv !== "object" ||
      typeof snapshot.cv.name !== "string" ||
      typeof snapshot.cv.type !== "string" ||
      typeof snapshot.cv.payloadId !== "string"
    ) {
      return null;
    }
    return entry;
  }

  private payloadIds(state: Record<string, StoredArmedEntry>): Set<string> {
    const ids = new Set<string>();
    for (const entry of Object.values(state)) {
      for (const id of this.entryPayloadIds(entry)) ids.add(id);
    }
    return ids;
  }

  private entryPayloadIds(entry: StoredArmedEntry): string[] {
    return entry.snapshot?.cv?.payloadId ? [entry.snapshot.cv.payloadId] : [];
  }

  private key(tabId: number): string {
    return String(tabId);
  }

  private serial<T>(fn: () => Promise<T>): Promise<T> {
    const next = this.pending.then(fn, fn);
    this.pending = next.then(
      () => undefined,
      () => undefined,
    );
    return next;
  }
}

function decodeBase64(value: string): Uint8Array {
  const binary = globalThis.atob(value);
  return Uint8Array.from(binary, (character) => character.charCodeAt(0));
}

function encodeBase64(bytes: Uint8Array): string {
  let binary = "";
  for (let offset = 0; offset < bytes.length; offset += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + 0x8000));
  }
  return globalThis.btoa(binary);
}
