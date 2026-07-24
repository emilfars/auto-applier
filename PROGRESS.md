# PROGRESS.md — State & Working Memory

> Living document. The single source of truth for **where the project is right now**.
> Update this at the end of every working session and whenever a milestone/task changes state.
> Keep entries terse and factual. History goes in the Log (bottom); current truth goes up top.

**Last updated:** 2026-07-24
**Current phase:** M3 — ingestion + feed (nearly complete). Scrapers (SCR-1..4) and the feed API (FEED-1/2/3) all landed and wired. Remaining: perf gates (FEED-2b p95 <500ms on 50k, FEED-1p 4G) and seeding ≥5k listings.
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
| M2 CV & profile | CV-1..4 + confirm-before-apply gate | ✅ | CV upload (AC-CV-1/1b) ✅, hosted-API parsing (AC-CV-2, ≥90% field accuracy on id+en fixtures — 100%) ✅, profile edit (AC-CV-3/4) ✅, confirm-before-apply gate (AC-CV-5) ✅ — all on in-memory **and** pgx repos (validated vs real Postgres). |
| M3 Ingestion + feed | SCR-1..4, FEED-1..3, seed ≥5k listings | 🟡 | Scrapers **done** (SCR-1..4) and feed API **done** (FEED-1/2/3): normalizer + Tier-restricted registry (AC-SCR-2/2b), circuit-breaker Runner + HTMLSource (AC-SCR-1/1b), cross-source dedup (AC-SCR-3), 48h staleness (AC-SCR-4); paginated **stated-only** cards with no estimated pay field (FEED-1), filters for pay/location/remote/skills/YoE/employment-type/posted-date/source (FEED-2), free-text search on title+company (FEED-3). Feed wired into cmd/api over the in-memory store. Remaining: perf gates (FEED-2b p95 <500ms on 50k, FEED-1p 4G) and seed ≥5k listings — both likely need a pgx-backed job store. |
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
- **Now:** M3 feed API landed (FEED-1/2/3). Paginated cards expose **stated pay only** — the DTO has no estimated field and a test asserts the response body contains no "estimat" (locked decision). Filters cover pay overlap, location/remote, skills, YoE (nil-YoE jobs always pass), employment type, posted-after and source; free-text search matches title + company (FEED-3); newest-first sort with default limit 20 (capped 100). Feed is wired into cmd/api over the in-memory store via an `ActiveJobs` adapter (structurally satisfies `feed.Provider`, no import cycle). Migration `0003_jobs_years_experience` adds the YoE column and applies cleanly against real Postgres. verify.sh green (8/8).
- **Next:** Perf gates (FEED-2b filter p95 <500ms on a 50k seed; FEED-1p p95 <2s on simulated 4G) and seeding ≥5k Jabodetabek listings. Both likely need a pgx-backed `JobStore` + feed `Provider` (currently in-memory only) and a seed generator. Job-queue choice (#4) still deferred; runner invoked in-process for now.
- **Blocked:** none. Deferred by decision: real Google OIDC client credentials (postponed until other implementations complete, per user).

## 7. Open questions / decisions needed
| # | Question | Owner | Status |
|---|---|---|---|
| 1 | Approve `plan.md`? | User | Open |
| 2 | Create GitHub remote + push? | User | Open |
| 3 | CV parser: hosted API vs Python sidecar (M2) | User | ✅ Decided: hosted API |
| 4 | Job queue: `asynq` vs `River` (M3) | TBD | Deferred to M3 |

## 8. How to update this file
1. Flip milestone/component status cells as state changes.
2. Keep **Snapshot** and **Now/Next/Blocked** current — they are read first.
3. Append a dated line to the Log for every meaningful change.
4. After finishing work, run `./scripts/verify.sh` and record the result in the Snapshot.

## 9. Log (newest first)
- **2026-07-24** — [DONE] M3 feed API (AC-FEED-1/2/3): new `backend/internal/feed` package exposing `GET /feed`. Cards show **stated pay only** — `salaryCard` has stated_min/max/currency/label/**stated** and deliberately no estimated field; a unit test asserts the JSON response contains no "estimat" (locked decision, PRD §1). `Index.Search` applies FEED-2 filters (pay-range overlap, location substring incl. remote, skills-all-present, MaxYoE with nil-YoE always passing, employment-type exact, source exact, posted-after) and FEED-3 free-text search over title+company, then sorts newest-PostedAt-first (dated ahead of undated) and paginates (default limit 20, capped 100). IDR labels use dotted-thousands formatting; unstated pay → empty label + stated=false. Added `YearsExperience` to `ingest.Job` + `parseYearsExperience`, migration `0003_jobs_years_experience` (applies cleanly against real Postgres), and an `ActiveJobs` adapter on `MemoryStore` so it satisfies `feed.Provider` without an import cycle. Feed wired into cmd/api. httptest + unit tests. verify.sh green (8/8). **Feed track (FEED-1/2/3) complete; M3 remaining = perf gates + seed ≥5k.**
- **2026-07-24** — [DONE] M3 cross-source dedup + staleness (AC-SCR-3/4): integration proves the same listing scraped from two sources collapses to one canonical job via the source-independent dedup key (AC-SCR-3). Added a pure, clock-injectable staleness rule (`IsStale`, `StaleAfter = 48h`); `MemoryStore` now tracks each job's last-seen time, `SweepStale(maxAge)` marks not-re-seen listings stale within the window and `Active()` excludes them from the feed set, while re-seeing a listing refreshes last-seen and clears the flag (AC-SCR-4). Unit tests use an injected clock. verify.sh green (8/8). **Scraper track (SCR-1..4) complete.**
- **2026-07-24** — [DONE] M3 scraper runner + circuit breakers (AC-SCR-1/1b): `ingest.Runner.RunOnce` iterates registered sources, each guarded by a **per-source circuit breaker** (opens after N consecutive failures, half-opens after a cooldown, closes on a success) so a broken source is isolated — the run continues and healthy sources keep the feed non-empty (AC-SCR-1b). Fetched RawJobs are normalized and upserted by dedup key into a `JobStore` (in-memory impl added; pgx to follow). Added a Tier-2 `HTMLSource` (data-driven `CardSelectors`, `golang.org/x/net/html`) that parses a listing page and resolves relative/absolute URLs; validated end-to-end against an httptest fixture HTML server persisting normalized jobs (AC-SCR-1). Added `golang.org/x/net` dependency. verify.sh green (8/8).
- **2026-07-24** — [DONE] M3 ingestion domain (AC-SCR-2/2b): new `backend/internal/ingest` package. `Normalize(RawJob)→Job` validates required fields and produces the full normalized shape (title, company, location, stated salary min/max in IDR, requirements, seniority, employment_type, posted_at, source_url), with employment-type/seniority synonym mapping (id+en), remote detection, and `ParseSalaryIDR` handling dotted-thousands (`Rp 8.000.000 - 12.000.000`) and `juta`/`jt` shorthand while returning nil for negotiable/undisclosed pay (stated-only, never fabricated). A source-independent `dedupKey` (company|title|city hash) lays the SCR-3 foundation. `Registry.Register` enforces the locked sourcing policy — Tier 1/2 accepted, Tier 3 login-walled rejected with `ErrTierExcluded` (AC-SCR-2b). Table-driven unit tests. verify.sh green (8/8).
- **2026-07-24** — [DONE] M2 CV parsing (AC-CV-2): decision — **hosted resume-parse API** (over a Python sidecar). New vendor-agnostic `cv.Parser` interface with `HostedParser` (JSON envelope: base64 doc + content type; Bearer auth; response mapped through one `hostedResponse` shape) and `DisabledParser` fallback. Deterministic `normalize()` core canonicalises whitespace/casing, phone numbers (local `0…`→`+62…`), open-ended date ranges (`present`/`sekarang`/`current`→empty), and de-dups/sorts skills. New endpoint `POST /cv/{id}/parse` (ownership-checked, 404 on other users' files) reads the encrypted CV, parses, and writes contact/education/work/skills into the profile via a `ProfileWriter` (`profile.Service.ApplyParsedCV`) — leaving `confirmed=false` so the user reviews before arming (AC-CV-3/5, Prime Directive). Labeled id+en fixtures assert ≥90% field accuracy (100%); httptest round-trip + fake-parser endpoint tests cover mapping, ownership, 503/502 paths. Parser is 503 until `CV_PARSER_URL` is set (upload/edit unaffected). verify.sh green (8/8). **M2 complete (AC-CV-1..5).**
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
