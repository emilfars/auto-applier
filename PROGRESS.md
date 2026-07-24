# PROGRESS.md — State & Working Memory

> Living document. The single source of truth for **where the project is right now**.
> Update this at the end of every working session and whenever a milestone/task changes state.
> Keep entries terse and factual. History goes in the Log (bottom); current truth goes up top.

**Last updated:** 2026-07-24
**Current phase:** M0 — Foundations (in progress). Backend health API + React/i18n web shell landed. Node toolchain provisioned.
**Verify gate:** `./scripts/verify.sh` → green (backend 4/4, web 4/4; extension/fill-mappings/android skipped).

---

## 1. Snapshot
| Field | Value |
|---|---|
| Repo | `auto-applier` (local, branch `main`, no GitHub remote yet) |
| Product | Auto Applier — human-in-the-loop job-application autofill |
| Stage | Pre-implementation (docs + scaffolding decisions only) |
| Active milestone | **M0 — Foundations** (not started) |
| Blocking gate | User must review & approve `plan.md` before any code |
| Prime directive | System never submits; user always clicks Apply |

## 2. Milestone status
Legend: ⬜ not started · 🟡 in progress · ✅ done · ⛔ blocked

| Milestone | Scope | Status | Notes |
|---|---|---|---|
| M0 Foundations | repo scaffold, CI, DB schema+migrations, object storage, health API, React shell | 🟡 | Backend health API + tests landed (Go stdlib). React shell blocked: Node/npm not installed in env. |
| M1 Accounts | AUTH-1..4 | ⬜ | |
| M2 CV & profile | CV-1..4 + confirm-before-apply gate | ⬜ | |
| M3 Ingestion + feed | SCR-1..4, FEED-1..3, seed ≥5k listings | ⬜ | |
| M4 Extension autofill | fill-mappings, APP-1,2,4,5 | ⬜ | **End of MVP** |
| M5 P1 enhancements | SCR-5,6 · FEED-4,5,6 · APP-6,7 · CV-5,6 · AUTH-5 | ⬜ | Post-MVP |
| M6 Android fast-follow | MOB-1..4 | ⬜ | After desktop mappings proven |

## 3. Component readiness
| Component | Path | Exists | verify.sh checks | State |
|---|---|---|---|---|
| Backend (Go) | `/backend` | yes | build, vet, gofmt, test | health API (GET /healthz) + tests, all green |
| Web (React+TS) | `/web` | yes | lint, typecheck, test, build | Vite+React+TS shell, i18n (id-ID/en, IDR), all green |
| Fill mappings | `/packages/fill-mappings` | no | lint, typecheck, test, build | not scaffolded |
| Extension (MV3) | `/extension` | no | lint, typecheck, test, build | not scaffolded |
| Android | `/android` | no | gradle assembleDebug | not scaffolded |

## 4. Repo artifacts present
- `plan.md` — MVP-first build plan (awaiting approval)
- `PRD.md` — compact detailed PRD with requirement IDs
- `README.md` — stub
- `.github/copilot-instructions.md` — System Protocol
- `scripts/verify.sh` — Deterministic Feedback Gate
- `PROGRESS.md` — this file
- `ACCEPTANCE.md` — machine-verifiable test specs

## 5. Locked decisions (do not re-litigate)
- Sources: **Tier 1 + Tier 2 only** (no login-walled Tier 3).
- Salary: **stated-only at MVP**; estimation post-MVP, always labeled distinctly.
- Geo: **Jabodetabek-first**. Monetization: none at MVP. Employer side: out of scope.
- CV data always user-confirmed before first apply.

## 6. Now / Next / Blocked
- **Now:** M0 backend health API + React/i18n web shell implemented, tested; verify.sh green (8/8). Node/npm provisioned via Homebrew.
- **Next:** M0 remaining foundations — DB schema + migrations, object storage wiring (backend), then M1 Accounts (AUTH-1..4). Also can scaffold `packages/fill-mappings` (core value) when M4 begins.
- **Blocked:** none. (Note: env required Node install; done. `npm` here uses an allow-scripts policy — esbuild install scripts must be approved via `npm approve-scripts esbuild` after installs.)

## 7. Open questions / decisions needed
| # | Question | Owner | Status |
|---|---|---|---|
| 1 | Approve `plan.md`? | User | Open |
| 2 | Create GitHub remote + push? | User | Open |
| 3 | CV parser: hosted API vs Python sidecar (M2) | TBD | Deferred to M2 |
| 4 | Job queue: `asynq` vs `River` (M3) | TBD | Deferred to M3 |

## 8. How to update this file
1. Flip milestone/component status cells as state changes.
2. Keep **Snapshot** and **Now/Next/Blocked** current — they are read first.
3. Append a dated line to the Log for every meaningful change.
4. After finishing work, run `./scripts/verify.sh` and record the result in the Snapshot.

## 9. Log (newest first)
- **2026-07-24** — [DONE] M0 web shell: provisioned Node 26 + npm 11 (Homebrew). Scaffolded `/web` Vite+React+TS with dependency-light i18n module (id-ID default + en, IDR currency) and Vitest locale test (AC-NFR-I18N ✅: no missing/stray keys, IDR default). Fixed `scripts/verify.sh` `has_script` bug (require needed `./` prefix; was silently skipping present Node components). Aligned vite→^5 to dedupe with vitest. verify.sh green (8/8).
- **2026-07-24** — [DONE] M0 backend health API: scaffolded `/backend` Go module (stdlib `net/http`), `GET /healthz` JSON endpoint, table-driven handler test (ok/404/405), graceful-shutdown `cmd/api`. verify.sh green (backend 4/4). Satisfies AC-GATE-1..3.
- **2026-07-24** — [BLOCKED→RESOLVED] Node/npm was not installed; user approved Homebrew install. Unblocked all TS components.
- **2026-07-24** — Added `PROGRESS.md` + `ACCEPTANCE.md`. Repo now has full doc/governance set. Still pre-implementation.
- **2026-07-24** — Added `PRD.md`, `.github/copilot-instructions.md`, `scripts/verify.sh` (commit 471d3db). verify.sh green (all skipped).
- **2026-07-24** — Initialized repo; added `plan.md` + `README.md` (commit d2f5a35). Locked source/salary/geo decisions.
