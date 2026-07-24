# PROGRESS.md — State & Working Memory

> Living document. The single source of truth for **where the project is right now**.
> Update this at the end of every working session and whenever a milestone/task changes state.
> Keep entries terse and factual. History goes in the Log (bottom); current truth goes up top.

**Last updated:** 2026-07-24
**Current phase:** M2 — CV & profile (near complete). CV upload, profile edit, and the confirm-before-apply gate all landed (in-memory + pgx repos). Only CV parsing (AC-CV-2) remains, deferred pending the parser-strategy decision.
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
| M0 Foundations | repo scaffold, CI, DB schema+migrations, object storage, health API, React shell | ✅ | Health API, web+i18n shell, embedded DB migrations, AES-256-GCM object storage, docker-compose (API+DB+web). |
| M1 Accounts | AUTH-1..4 | 🟡 | AUTH-1/1b/2/3/4/4b ✅ (email/password + Google OAuth via mocked OIDC, sessions, reset, hardened rate limiting). Now backed by **both** in-memory and **pgx** repos; migrations apply at startup (verified against real Postgres). Remaining: real Google OIDC client (needs live credentials). |
| M2 CV & profile | CV-1..4 + confirm-before-apply gate | 🟡 | CV upload (AC-CV-1/1b) ✅, profile edit (AC-CV-3/4) ✅, confirm-before-apply gate (AC-CV-5) ✅ — all on in-memory **and** pgx repos (validated vs real Postgres). Remaining: CV parsing (CV-2) deferred pending parser-strategy decision. |
| M3 Ingestion + feed | SCR-1..4, FEED-1..3, seed ≥5k listings | ⬜ | |
| M4 Extension autofill | fill-mappings, APP-1,2,4,5 | ⬜ | **End of MVP** |
| M5 P1 enhancements | SCR-5,6 · FEED-4,5,6 · APP-6,7 · CV-5,6 · AUTH-5 | ⬜ | Post-MVP |
| M6 Android fast-follow | MOB-1..4 | ⬜ | After desktop mappings proven |

## 3. Component readiness
| Component | Path | Exists | verify.sh checks | State |
|---|---|---|---|---|
| Backend (Go) | `/backend` | yes | build, vet, gofmt, test | health API + auth (in-memory + pgx repos, startup migrations); all green |
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
- **Now:** M2 profile edit + confirm-before-apply gate landed. `GET/PATCH /profile` (structured + added-info fields, AC-CV-3/4, editing resets `confirmed`), `POST /profile/confirm`, and `GET /profile/arm` (AC-CV-5: refuses to arm until user confirms). Both in-memory and pgx repos, validated vs real Postgres. verify.sh green (8/8).
- **Next:** CV parsing (AC-CV-2) — deferred pending the hosted-API-vs-Python-sidecar decision (open question #3). Otherwise M2 (parser-independent) is complete; the next milestone is M3 (ingestion + feed: SCR-1..4, FEED-1..3).
- **Blocked:** none. Pending decisions: CV parser strategy (#3) and real Google OIDC client credentials (M1 finish).

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
- **2026-07-24** — [DONE] M2 profile edit + confirm-before-apply gate (AC-CV-3/4/5): new `/backend/internal/profile` package — `GET /profile` returns structured + added-info fields; `PATCH /profile` validates and persists edits (expected_salary ≥0, notice_period_days 0–365, employment_type enum, JSON-array fields) and **resets `confirmed=false`** so edits force re-review; `POST /profile/confirm` sets the confirm flag; `GET /profile/arm` returns 200 `{can_arm:true}` only when confirmed else 403 (AC-CV-5, prime-directive-critical — arming refused until the human confirms). In-memory **and** pgx repos (upsert on `user_id`, JSONB round-trip); pgx validated green against real Postgres. `cmd/api` shares one pgx pool across auth+profile and mounts `/profile` behind `RequireVerified`. verify.sh green (8/8).
- **2026-07-24** — [DONE] M2 CV upload (AC-CV-1/1b): new `/backend/internal/cv` package — `POST /cv` multipart upload gated behind `RequireVerified`, validates type by extension **and** magic bytes (PDF `%PDF`, DOCX zip `PK\x03\x04`; spoofed headers fail closed) and size ≤5MB → 201/415/413; stores via encrypted `ObjectStore` (AES-256-GCM at rest, AC-CV-1b) and persists metadata only. `GET /cv` lists a user's files. `cmd/api` wires it with an encrypted store (CV_ENCRYPTION_KEY or ephemeral dev key); `api.NewRouter` now takes extra gated mounts; added `auth.ContextWithUser` for cross-package composition. Table-driven upload matrix + at-rest-ciphertext tests. verify.sh green (8/8).
- **2026-07-24** — [DONE] M1 pgx-backed auth persistence: added `PgxRepo` (implements `auth.Repo` over pgx; UUIDs cast to text, unique-violation→`ErrEmailTaken`, single-use token consumption via `DELETE ... RETURNING`), version-tracked idempotent migrator `db.Apply` (+`schema_migrations`), and `0002_auth_tokens` migration (`email_verifications`, `sessions`, `password_resets`). `cmd/api` now selects pgx when `DATABASE_URL` is set and applies migrations at startup, else in-memory. DB-guarded integration tests skip offline (keep gate green) and were verified green against a real Postgres 16 (5/5). verify.sh green (8/8).
- **2026-07-24** — [DONE] M1 auth rate-limiter hardening (security review): (1) bounded the limiter map — expired windows are now swept lazily once per window (`ratelimit.go`), fixing unbounded growth / memory-exhaustion DoS; (2) added a per-client-IP limiter (default 30/15min) to `register`, `login`, and `password-reset` request/confirm, capping unauthenticated PBKDF2 CPU burn and single-source password-spray; (3) login now keyed by composite IP+email (`clientIP()` helper, RemoteAddr only — X-Forwarded-For untrusted), preventing targeted global account-lockout DoS while the per-IP limiter blocks email rotation. New tests: register/reset-confirm IP throttling, composite-key no-global-lockout, per-IP spray cap, expired-window eviction. verify.sh green (8/8).
- **2026-07-24** — [DONE] M1 AUTH-2 Google OAuth: added `OIDCProvider` interface (code→verified claims) + `POST /auth/oauth/google/callback` handler that links-or-creates a verified account and issues a session; refactored session issuance into shared `startSession`. Rejects unverified/absent email. Route only registers when a provider is configured. Tested with a fake OIDC provider (create, link-existing, unverified-reject, exchange-failure). verify.sh green (8/8).
- **2026-07-24** — [DONE] M1 email/password auth (AUTH-1/1b/3/4/4b): `/backend/internal/auth` — PBKDF2-SHA256 hashing (stdlib crypto/pbkdf2), opaque tokens, in-memory `Repo` behind interface, fixed-window login rate limiter, `RequireVerified` middleware (401 no session / 403 unverified). Endpoints: register, verify, login, logout, me, password-reset request/confirm. Mounted in `cmd/api`. HTTP integration tests green + server smoke test. verify.sh green (8/8).
- **2026-07-24** — [DONE] M0 object storage + docker-compose: `/backend/internal/storage` ObjectStore + in-memory backend + AES-256-GCM `EncryptedStore` (encryption at rest); tests cover round-trip, at-rest ciphertext≠plaintext (AC-CV-1b), tamper detection, missing key. Added Dockerfiles (backend distroless, web nginx) + root `docker-compose.yml` (Postgres+API+web), validated via `docker compose config`. README documents dev + gate. verify.sh green (8/8).
- **2026-07-24** — [DONE] M0 DB migrations: added `/backend/internal/db` embedded SQL migrations (`0001_init` up/down) for users, profiles, cv_files, jobs, applications — Jabodetabek-first, stated-salary-only (no estimated columns), `profiles.confirmed` gate, `applications.status` never defaults to 'submitted' (prime directive). `LoadMigrations()` validates contiguous versions + up/down pairing; deterministic offline Go tests assert invariants. verify.sh green (8/8).
- **2026-07-24** — [DONE] M0 web shell: provisioned Node 26 + npm 11 (Homebrew). Scaffolded `/web` Vite+React+TS with dependency-light i18n module (id-ID default + en, IDR currency) and Vitest locale test (AC-NFR-I18N ✅: no missing/stray keys, IDR default). Fixed `scripts/verify.sh` `has_script` bug (require needed `./` prefix; was silently skipping present Node components). Aligned vite→^5 to dedupe with vitest. verify.sh green (8/8).
- **2026-07-24** — [DONE] M0 backend health API: scaffolded `/backend` Go module (stdlib `net/http`), `GET /healthz` JSON endpoint, table-driven handler test (ok/404/405), graceful-shutdown `cmd/api`. verify.sh green (backend 4/4). Satisfies AC-GATE-1..3.
- **2026-07-24** — [BLOCKED→RESOLVED] Node/npm was not installed; user approved Homebrew install. Unblocked all TS components.
- **2026-07-24** — Added `PROGRESS.md` + `ACCEPTANCE.md`. Repo now has full doc/governance set. Still pre-implementation.
- **2026-07-24** — Added `PRD.md`, `.github/copilot-instructions.md`, `scripts/verify.sh` (commit 471d3db). verify.sh green (all skipped).
- **2026-07-24** — Initialized repo; added `plan.md` + `README.md` (commit d2f5a35). Locked source/salary/geo decisions.
