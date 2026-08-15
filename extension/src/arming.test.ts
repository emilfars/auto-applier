import { describe, it, expect } from "vitest";
import {
  ARM_EXPIRY_ALARM,
  ArmingStore,
  installArmExpiryAlarmListener,
  normalizeUrl,
  SessionArmingStore,
  type ArmExpiryAlarmScheduler,
  type BinaryPayloadStore,
  type SessionStorageLike,
} from "./arming.js";

describe("normalizeUrl", () => {
  it("drops query, hash, and trailing slash and lowercases", () => {
    expect(normalizeUrl("https://Jobs.Lever.co/acme/1/?utm=x#top")).toBe(
      "https://jobs.lever.co/acme/1",
    );
    expect(normalizeUrl("https://jobs.lever.co/acme/1")).toBe(
      "https://jobs.lever.co/acme/1",
    );
  });

  it("matches the armed URL after tracking params are appended", () => {
    const a = normalizeUrl("https://boards.greenhouse.io/acme/jobs/9");
    const b = normalizeUrl("https://boards.greenhouse.io/acme/jobs/9?src=feed");
    expect(a).toBe(b);
  });
});

describe("ArmingStore", () => {
  it("arms a URL and consumes it exactly once", () => {
    const s = new ArmingStore();
    s.arm("https://jobs.lever.co/acme/1", 1000);
    expect(s.consume("https://jobs.lever.co/acme/1", 1500)).toBe(true);
    // Second consume returns false (single-use).
    expect(s.consume("https://jobs.lever.co/acme/1", 1600)).toBe(false);
  });

  it("does not consume an unrelated URL", () => {
    const s = new ArmingStore();
    s.arm("https://jobs.lever.co/acme/1", 1000);
    expect(s.consume("https://jobs.lever.co/other/2", 1500)).toBe(false);
  });

  it("treats an expired arm as absent", () => {
    const s = new ArmingStore(60_000);
    s.arm("https://x.io/a", 1000);
    expect(s.consume("https://x.io/a", 1000 + 60_001)).toBe(false);
  });

  it("sweeps expired entries", () => {
    const s = new ArmingStore(1000);
    s.arm("https://x.io/a", 0);
    s.arm("https://x.io/b", 0);
    expect(s.size()).toBe(2);
    s.sweep(2000);
    expect(s.size()).toBe(0);
  });

  it("binds an arm to its created tab", () => {
    const s = new ArmingStore();
    s.arm("https://x.io/a", 0, 7);
    expect(s.consume("https://x.io/a", 1, 8)).toBe(false);
    expect(s.consume("https://x.io/a", 1, 7)).toBe(true);
  });

  it("keeps same-URL arms independent across tabs", () => {
    const s = new ArmingStore();
    s.arm("https://x.io/a", 0, 7);
    s.arm("https://x.io/a", 0, 8);
    expect(s.consume("https://x.io/a", 1, 7)).toBe(true);
    expect(s.consume("https://x.io/a", 1, 8)).toBe(true);
  });

  it("does not consume a same-tab arm for the wrong URL", () => {
    const s = new ArmingStore();
    s.arm("https://x.io/a", 0, 7);
    expect(s.consume("https://x.io/other", 1, 7)).toBe(false);
    expect(s.consume("https://x.io/a", 1, 7)).toBe(true);
  });
});

describe("SessionArmingStore", () => {
  function fakeStorage(): SessionStorageLike & { state: Record<string, unknown> } {
    const state: Record<string, unknown> = {};
    return {
      state,
      get: async (key) => {
        return key in state ? { [key]: state[key] } : {};
      },
      set: async (items) => {
        Object.assign(state, items);
      },
      remove: async (key) => {
        delete state[key];
      },
    };
  }

  function fakeBinaryStore() {
    const payloads = new Map<string, { bytes: Uint8Array; expiresAt: number }>();
    let nextId = 0;
    const store: BinaryPayloadStore & {
      puts: number;
      deletes: string[];
      payloads: typeof payloads;
    } = {
      puts: 0,
      deletes: [],
      payloads,
      put: async (bytes, expiresAt) => {
        const existing = [...payloads.entries()].find(
          ([, value]) =>
            value.bytes.length === bytes.length &&
            value.bytes.every((byte, index) => byte === bytes[index]),
        );
        const id = existing?.[0] ?? `payload-${nextId++}`;
        payloads.set(id, {
          bytes: bytes.slice(),
          expiresAt: Math.max(payloads.get(id)?.expiresAt ?? 0, expiresAt),
        });
        if (!existing) store.puts++;
        return id;
      },
      get: async (id) => payloads.get(id)?.bytes.slice() ?? null,
      delete: async (id) => {
        store.deletes.push(id);
        payloads.delete(id);
      },
      sweep: async (now, referencedIds) => {
        for (const [id, payload] of payloads) {
          if (payload.expiresAt <= now || (referencedIds && !referencedIds.has(id))) {
            payloads.delete(id);
          }
        }
      },
    };
    return store;
  }

  function fakeAlarmScheduler() {
    const scheduled: number[] = [];
    let clears = 0;
    const scheduler: ArmExpiryAlarmScheduler = {
      schedule: async (expiresAt) => {
        scheduled.push(expiresAt);
      },
      clear: async () => {
        clears++;
      },
    };
    return {
      scheduler,
      scheduled,
      get clears() {
        return clears;
      },
    };
  }

  it("survives a new store instance and consumes atomically once", async () => {
    const storage = fakeStorage();
    const first = new SessionArmingStore(storage);
    await first.arm("https://x.io/a", 0, 3, {
      profile: { confirmed: true, email: "u@example.com" },
    });
    const restarted = new SessionArmingStore(storage);
    const [one, two] = await Promise.all([
      restarted.consume("https://x.io/a", 1, 3),
      restarted.consume("https://x.io/a", 1, 3),
    ]);
    expect(one?.snapshot?.profile.email).toBe("u@example.com");
    expect(two).toBeNull();
  });

  it("removes expired durable snapshots", async () => {
    const storage = fakeStorage();
    const binary = fakeBinaryStore();
    const store = new SessionArmingStore(storage, 10, binary);
    await store.arm("https://x.io/a", 0, 3, {
      profile: { confirmed: true },
      cv: { name: "cv.pdf", type: "application/pdf", bytesBase64: "Y3Y=" },
    });
    await store.sweep(11);
    expect(await store.isArmed("https://x.io/a", 11, 3)).toBe(false);
    expect(binary.payloads.size).toBe(0);
  });

  it("sweeps IndexedDB payloads from the expiry alarm without another store operation", async () => {
    const storage = fakeStorage();
    const binary = fakeBinaryStore();
    const alarms = fakeAlarmScheduler();
    const store = new SessionArmingStore(storage, 10, binary, alarms.scheduler);
    let onAlarm:
      | ((alarm: { name: string }) => void)
      | undefined;
    let alarmSweep: Promise<void> | undefined;
    installArmExpiryAlarmListener(
      {
        onAlarm: {
          addListener: (listener) => {
            onAlarm = listener;
          },
        },
      },
      () => {
        alarmSweep = store.sweep(10);
        return alarmSweep;
      },
    );

    await store.arm("https://x.io/a", 0, 3, {
      profile: { confirmed: true },
      cv: { name: "cv.pdf", type: "application/pdf", bytesBase64: "Y3Y=" },
    });
    onAlarm?.({ name: ARM_EXPIRY_ALARM });
    await alarmSweep;

    expect(binary.payloads.size).toBe(0);
    expect(alarms.clears).toBeGreaterThan(0);
  });

  it("peeks and clears exactly one tab arm", async () => {
    const storage = fakeStorage();
    const store = new SessionArmingStore(storage);
    await store.arm("https://x.io/a", 0, 3);
    await store.arm("https://x.io/a", 0, 4);
    expect((await store.peek("https://x.io/a", 1, 3))?.tabId).toBe(3);
    await store.clear(3);
    expect(await store.isArmed("https://x.io/a", 1, 3)).toBe(false);
    expect(await store.isArmed("https://x.io/a", 1, 4)).toBe(true);
  });

  it("does not consume a durable arm for the wrong URL", async () => {
    const storage = fakeStorage();
    const store = new SessionArmingStore(storage);
    await store.arm("https://x.io/a", 0, 3);
    expect(await store.consume("https://x.io/other", 1, 3)).toBeNull();
    expect(await store.consume("https://x.io/a", 1, 3)).not.toBeNull();
  });

  it("keeps two max-size CV arms out of session metadata", async () => {
    const storage = fakeStorage();
    const binary = fakeBinaryStore();
    const store = new SessionArmingStore(storage, 60_000, binary);
    const bytes = "A".repeat(5 * 1024 * 1024);
    const bytesBase64 = btoa(bytes);

    await store.arm("https://x.io/a", 0, 3, {
      profile: { confirmed: true },
      cv: { name: "a.pdf", type: "application/pdf", bytesBase64 },
    });
    await store.arm("https://x.io/b", 0, 4, {
      profile: { confirmed: true },
      cv: { name: "b.pdf", type: "application/pdf", bytesBase64 },
    });

    const metadata = JSON.stringify(storage.state);
    expect(metadata).not.toContain(bytesBase64);
    expect(binary.payloads.size).toBe(1);
    expect(binary.puts).toBe(1);
  });

  it("shares a payload and deletes it only after the final consume", async () => {
    const storage = fakeStorage();
    const binary = fakeBinaryStore();
    const store = new SessionArmingStore(storage, 60_000, binary);
    const snapshot = {
      profile: { confirmed: true },
      cv: { name: "cv.pdf", type: "application/pdf", bytesBase64: "Y3Y=" },
    };

    await store.arm("https://x.io/a", 0, 3, snapshot);
    await store.arm("https://x.io/b", 0, 4, snapshot);
    const first = await store.consume("https://x.io/a", 1, 3);

    expect(first?.snapshot?.cv?.bytesBase64).toBe("Y3Y=");
    expect(binary.payloads.size).toBe(1);
    expect(binary.deletes).toHaveLength(0);

    await store.consume("https://x.io/b", 2, 4);
    expect(binary.payloads.size).toBe(0);
    expect(binary.deletes).toHaveLength(1);
  });

  it("reschedules the next expiry and clears the alarm after the last consume", async () => {
    const storage = fakeStorage();
    const alarms = fakeAlarmScheduler();
    const store = new SessionArmingStore(storage, 10, fakeBinaryStore(), alarms.scheduler);

    await store.arm("https://x.io/a", 0, 3);
    await store.arm("https://x.io/b", 1, 4);
    await store.consume("https://x.io/a", 2, 3);

    expect(alarms.scheduled).toContain(11);
    expect(alarms.clears).toBe(0);
    await store.clear();
    expect(alarms.clears).toBeGreaterThan(0);
  });

  it("clears binary payloads on logout and sweeps expired references", async () => {
    const storage = fakeStorage();
    const binary = fakeBinaryStore();
    const store = new SessionArmingStore(storage, 10, binary);
    await store.arm("https://x.io/a", 0, 3, {
      profile: { confirmed: true },
      cv: { name: "cv.pdf", type: "application/pdf", bytesBase64: "Y3Y=" },
    });
    await store.sweep(10);
    expect(binary.payloads.size).toBe(0);

    await store.arm("https://x.io/b", 11, 4, {
      profile: { confirmed: true },
      cv: { name: "cv.pdf", type: "application/pdf", bytesBase64: "Y3Y=" },
    });
    await store.clear();
    expect(binary.payloads.size).toBe(0);
    expect(storage.state.autoApplierArms).toBeUndefined();
  });

  it("reloads metadata and payloads after a worker restart", async () => {
    const storage = fakeStorage();
    const binary = fakeBinaryStore();
    const first = new SessionArmingStore(storage, 60_000, binary);
    await first.arm("https://x.io/a", 0, 3, {
      profile: { confirmed: true, email: "u@example.com" },
      cv: { name: "cv.pdf", type: "application/pdf", bytesBase64: "Y3Y=" },
    });

    const restarted = new SessionArmingStore(storage, 60_000, binary);
    const entry = await restarted.consume("https://x.io/a", 1, 3);
    expect(entry?.snapshot?.profile.email).toBe("u@example.com");
    expect(entry?.snapshot?.cv?.bytesBase64).toBe("Y3Y=");
  });

  it("removes legacy base64 arms during the next expiry sweep", async () => {
    const storage = fakeStorage();
    storage.state.autoApplierArms = {
      "3": {
        url: "https://x.io/a",
        expiresAt: 60_000,
        tabId: 3,
        snapshot: {
          profile: { confirmed: true },
          cv: { name: "cv.pdf", type: "application/pdf", bytesBase64: "Y3Y=" },
        },
      },
    };
    const store = new SessionArmingStore(storage, 60_000, fakeBinaryStore());

    await store.sweep(1);

    expect(JSON.stringify(storage.state)).not.toContain("bytesBase64");
  });
});
