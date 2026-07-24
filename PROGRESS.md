# PROGRESS.md — State & Working Memory

> Living document. The single source of truth for **where the project is right now**.
> Update this at the end of every working session and whenever a milestone/task changes state.
> Keep entries terse and factual. History goes in the Log (bottom); current truth goes up top.

**Last updated:** 2026-07-24
**Current phase:** M4 — Chrome extension autofill (**started**). Built the shared `packages/fill-mappings` first (per plan): versioned per-portal maps for all 7 targets (AC-MAP-1), a framework-agnostic fill engine with per-field `filled|uncertain|empty` states (AC-SAFE-2), and a static submit-API scan over the fill layer (AC-SAFE-1 partial). Next: the Chrome MV3 extension consuming this package (APP-1/2/4/5) plus its own submit scan.
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
| M3 Ingestion + feed | SCR-1..4, FEED-1..3, seed ≥5k listings | 🟢 | Scrapers (SCR-1..4), feed API (FEED-1/2/3), a ≥5k Jabodetabek **seed job** (AC-SEED-1) and the **50k filter p95 <500ms** perf gate (AC-FEED-2b) all done. `backend/internal/seed` generates deterministic, unique-dedup-key, stated-only listings against any `ingest.JobStore` (`cmd/seed` runnable); feed p95 measured at ~19ms on 50k. Remaining are perf-stage NFRs only (AC-FEED-1p 4G, AC-NFR-SCALE) and the deferred pgx-backed job store (in-memory today). |
| M4 Extension autofill | fill-mappings, APP-1,2,4,5 | 🟡 | **End of MVP.** Shared `packages/fill-mappings` **started**: framework-agnostic, zero-runtime-dep TS package with versioned maps for all 7 targets (Greenhouse, Lever, Workable, Jobstreet, Glints, Kalibrr, generic) + `getMapForHost` (AC-MAP-1 ✅), a fill engine emitting per-field `filled\|uncertain\|empty` with confidence decay + no-overwrite + confirm-gate (AC-SAFE-2 ✅), and an allowlist-free static scan proving no submit APIs in the fill layer (AC-SAFE-1 half — extension scan pending). 65 tests; typecheck/test/build wired into verify.sh. Remaining: the Chrome MV3 extension (APP-1/2/4/5) consuming this package + its own submit scan; eslint config. |
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
- **Now:** M4 kicked off with the shared `packages/fill-mappings` (the plan mandates building it first). It is framework-agnostic with **zero runtime dependencies** so the Chrome extension and Android WebView share one implementation. Versioned maps for Greenhouse/Lever/Workable/Jobstreet/Glints/Kalibrr + a generic fallback, `getMapForHost` host routing, and a fill engine that returns per-field `filled|uncertain|empty` with confidence decay on fallback selectors, never overwrites a user-typed value, and refuses to arm an unconfirmed profile (AC-MAP-1, AC-SAFE-2). An allowlist-free static scan asserts no submit-triggering APIs exist in the fill layer (AC-SAFE-1, fill-mappings half). 65 tests; typecheck/test/build wired into verify.sh (11 ran, 0 failed).
- **Next:** The Chrome MV3 extension (APP-1 detect+fill, APP-2 CV attach, APP-4 visual fill-review, APP-5 Open&Fill) consuming `fill-mappings`, plus the extension-side submit scan to complete AC-SAFE-1, and an eslint config for the package (currently the only skipped fill-mappings check). Deferred M3 items: perf-stage NFRs (AC-FEED-1p 4G, AC-NFR-SCALE) and the pgx-backed job store.
- **Blocked:** none. Deferred by decision: real Google OIDC client credentials, job-queue library choice (#4), pgx-backed job store.

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
- **2026-07-24** — [DONE] M4 shared fill layer `packages/fill-mappings` (AC-MAP-1, AC-SAFE-2; AC-SAFE-1 partial): new framework-agnostic, **zero-runtime-dependency** TypeScript package — the single source of truth reused by the extension now and the Android WebView later. Versioned `PortalMap`s for Greenhouse, Lever, Workable, Jobstreet, Glints, Kalibrr and a generic fallback, each with a `version` and per-portal tests; `getMapForHost` routes a hostname to the best map or the generic fallback (AC-MAP-1). The `buildFillPlan` engine reads a query root and emits a per-field `filled|uncertain|empty` outcome with confidence: fallback-selector matches decay below the uncertain threshold, a field that already holds a user-typed value is flagged `uncertain` (never overwritten), disabled/absent fields are `empty`, and an **unconfirmed profile yields an all-empty, unarmed plan** (AC-SAFE-2, AC-CV-5). Per the Prime Directive, an allowlist-free static scan over the runtime source asserts no submit-triggering APIs (`.submit()`, `requestSubmit`, `.click()`, `SubmitEvent`, `dispatchEvent`, `HTMLFormElement`) appear in any fill path (AC-SAFE-1 — the fill-mappings half; the extension-side scan follows with the extension). 65 tests (typecheck strict, vitest, tsc build) wired into verify.sh — green (11 ran, 3 skipped, 0 failed). Follow-ups: eslint config (only skipped check) and the Chrome MV3 extension consuming this package.
- **2026-07-24** — [DONE] M3 seed job + filter perf gate (AC-SEED-1, AC-FEED-2b): new `backend/internal/seed` package generates deterministic synthetic **Jabodetabek-first** listings. Identity fields (company/title/city) are derived by combinatorial indexing so every listing has a **unique dedup key** — a 6,000-listing test proves zero collisions, so the seed never silently under-fills the feed. Pay is **employer-stated only** (a minority map to "negotiable" → no stated pay; never an estimate). `seed.Seed(ctx, store, n, seed, baseTime)` upserts into any `ingest.JobStore` (in-memory now, pgx later) and a runnable `cmd/seed` loads ≥5,000 listings before signup (AC-SEED-1, asserted ≥5000 active). Added a feed perf test running a spread of representative filter+search queries over a **50k seed** and asserting **p95 < 500ms** — measured ~19ms (AC-FEED-2b). verify.sh green (8/8). **M3 complete for MVP scope; remaining M3 items are perf-stage NFRs + the deferred pgx job store.**
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
