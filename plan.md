# Auto Applier — Build Plan

> Status: **APPROVED (2026-07-25).** Implementation proceeding. MVP-solidification decisions recorded below.
> Based on PRD v0.1 (Owner: Hanif, 17 Jul 2026).

## Core product decision (non-negotiable)
Semi-automated, **human-in-the-loop**. The system fills forms; the **user always clicks Apply/Submit**. No headless/background submission, ever, in any phase. This is an architectural boundary, not a feature toggle.

**One-liner:** Upload your CV once; the browser fills every application for you, you just click apply.

## Decisions locked (previously open questions)
| Question | Decision |
|---|---|
| Source strategy | **Tier 1 + Tier 2** only (APIs/partner feeds + public boards without login walls). Exclude Tier 3 login-walled sources (LinkedIn etc.). |
| Salary estimation | **Launch stated-salary only.** Add estimation model post-MVP. |
| Geo scope | **Jabodetabek-first** (listing density, mobile-first market). |
| Monetization | Out of MVP (free). |
| Employer side | Out of scope. |

## MVP-solidification decisions (locked 2026-07-25)
Recorded so the loop does not re-litigate them. These solidify the MVP before any post-MVP milestone.

| Topic | Decision |
|---|---|
| plan.md | **Approved** — implementation proceeds. |
| GitHub remote | Create a remote and push (done: `emilfars/auto-applier`). |
| Job queue | **River** (`github.com/riverqueue/river`, Postgres-backed) — chosen over asynq. Drives scheduled scrape + parse. |
| Jobs storage | **Postgres (pgx-backed job store)** — approved. Replaces the in-memory feed store in prod; in-memory kept for tests/perf-gate. |
| CV metadata | **Postgres (pgx-backed CV repo)** — approved. (CV *bytes* stay in encrypted object storage; only metadata in PG.) |
| Seed | **Real listings, not synthetic** for MVP — pull a *small* volume (~25 per working job board) via the real sources. Synthetic generator retained only for the 50k perf gate. |
| Database (dev) | **Run Postgres locally** for now (homebrew `postgresql@18`); `DATABASE_URL` wired. No cloud DB yet. |
| Google OAuth (AUTH-2) | **Deferred** — needs live Google credentials. Email+password path is the MVP auth. |
| Tier 2 sources | Target list: **Jobstreet, Glints, Kalibrr, Indeed.** Reality (2026-07-25 probe): **Kalibrr** exposes a usable public JSON API (shipped). **Glints** = WAF firewall, **Jobstreet/SEEK** = anti-bot HTML, **Indeed** = no public API + ToS prohibits scraping → all three are **blocked for static ingestion** and deferred pending official API/partner access or a ToS-cleared headless approach. Do not ship fragile/ToS-violating scrapers. |
| Tier 1 sources | Seek a free API with Indonesian coverage. Findings: Adzuna/The Muse lack Indonesia; **Careerjet** (locale `id_ID`) and **Jooble** (id) cover Indonesia but need a free `affid`/API key. Implement a keyed, env-gated, mockable Tier-1 REST source (degrades when unset, like the CV parser) and document how to obtain the free key. |
| Held until MVP is solid | Extension runtime bundler (already shipped — no further work), backend telemetry ingestion endpoint for `fill_correction`, and perf-stage NFRs (AC-FEED-1p 4G, AC-NFR-SCALE). |
| Web UI (in MVP) | Ship the seeker web UI: **profile + CV-upload screens, feed pagination, and an "Open & Fill" button on feed cards.** |

## Current implementation checkpoint (2026-08-15)

M0→M4 component code exists and the full local build/lint/typecheck/unit-test gate passes after dependencies are installed. The MVP is still in hardening; do not start M5 until these launch blockers are closed:

| Blocker | Current state |
|---|---|
| Web → extension data path | Open & Fill transfers only a URL. No runtime path writes the confirmed profile or CV file into extension storage. |
| Docker full stack | Production nginx serves the SPA but does not proxy `/auth`, `/profile`, `/cv`, `/feed`, `/account`, or `/healthz` to the API. |
| CV object persistence | CV metadata persists in Postgres, but encrypted CV bytes use an in-memory object store and disappear on API restart. |
| SCR-4 staleness | The shared rule is 48 hours, but River runtime configuration defaults to 14 days. |

Before declaring MVP complete, add one browser-level Open & Fill test using the real web profile/CV path, one Docker/API smoke test, and Postgres-backed verification that does not silently skip.

## Tech stack
- **Backend:** Go (net/http or chi/gin), `pgx` for Postgres, job queue: **River** (Postgres-backed) for scrape + parse, `colly`/`chromedp` for scrapers.
- **DB / storage:** Postgres (users, profiles, jobs, applications) + Postgres full-text search (defer OpenSearch until latency demands) + S3-compatible object storage for CV files (encrypted at rest).
- **Frontend:** React + TypeScript (Vite), TanStack Query, Tailwind, i18n (id-ID + en, IDR default).
- **Extension:** Chrome MV3 + TypeScript.
- **Shared fill logic:** `packages/fill-mappings` — versioned per-portal field maps + tests, consumed by extension now and Android WebView later.
- **Mobile (fast-follow):** Kotlin/Android WebView reusing `fill-mappings`.
- **CV parsing:** Go service → parser (hosted resume-parse API or Python sidecar) → structured JSON, always user-reviewed.

## Repo layout
```
/backend                  Go services: auth, profile, parsing, ingestion, feed
/web                      React + TS web app
/extension                Chrome MV3 + TS
/packages/fill-mappings   Shared TS: portal field maps + tests
/android                  Fast-follow (later)
```

---

# Strategy: ship a working MVP first, add components later

The plan is ordered so that **each milestone is independently shippable and demoable**. We build the thinnest end-to-end working slice first, then layer capability on top. Do **not** build web + extension + mobile in parallel — prove the fill engine on desktop before expanding surface area.

## Milestone 0 — Foundations (working skeleton)
**Goal:** an empty but running system: API up, DB migrated, web app loads, CI green.
- Repo scaffolding, CI, linting, env config, secrets handling.
- Postgres schema + migrations (users, profiles, cv_files, jobs, applications).
- Object storage wired for CV files, encrypted at rest; HTTPS-only.
- Health-check API + minimal React shell with i18n scaffolding.

**Done when:** `docker compose up` runs API + DB + web; CI passes.

## Milestone 1 — Accounts (P0)
**Goal:** a user can register, verify, log in, log out.
- AUTH-1 email + password with email verification.
- AUTH-2 Google OAuth.
- AUTH-3 password reset.
- AUTH-4 sessions, secure token handling, logout; rate limiting on auth.

**Done when:** a real user can create an account and log back in.

## Milestone 2 — CV & profile (P0)
**Goal:** a user uploads a CV and ends up with a confirmed structured profile.
- CV-1 upload (PDF/DOCX, max 5MB).
- CV-2 async parse → contact, education, work history, skills.
- CV-3 in-app editing of all parsed fields.
- CV-4 added-info form: expected salary, notice period, work authorization, relocation, preferred locations, employment type.
- **Confirm-before-apply gate:** user must review + confirm parsed data before first apply (data-quality mechanism).

**Done when:** a user finishes with a complete, self-confirmed profile ready to fill forms.

## Milestone 3 — Ingestion + feed (P0)
**Goal:** a browsable, filterable feed of real Jabodetabek listings.
- SCR-1 scheduled Tier 1+2 scrapers with **per-source circuit breakers** (one source failing must not take down the feed).
- SCR-2 normalization: title, company, location, stated salary, requirements, seniority, employment type, posting date, source URL.
- SCR-3 cross-source deduplication.
- SCR-4 staleness: mark expired/removed listings within 48h.
- FEED-1 card-view list (infinite scroll); **stated pay only at MVP, clearly labeled**.
- FEED-2 filters: pay range, location (city + remote), skills, YoE, employment type, posting date, source.
- FEED-3 free-text search on title + company.
- **Seed >= 5,000 listings before opening signup** (avoid empty-feed churn).

**Done when:** feed shows >=5k fresh listings and filters respond < 500ms.

## Milestone 4 — Chrome extension autofill (P0) — the core value
**Goal:** user opens a supported application page and all common fields fill; user clicks apply.
- Build `packages/fill-mappings` first: versioned field maps + tests for Jobstreet, Glints, Kalibrr, Greenhouse, Workable, Lever, common career forms.
- APP-1 detect supported forms + autofill common fields.
- APP-2 CV file auto-attach on upload fields.
- APP-4 visual fill-review: filled fields highlighted; uncertain/unfilled fields flagged with per-field confidence. **Submit is never triggered by the extension.**
- APP-5 "Open & Fill" from the feed (arm the extension on the source page).
- Telemetry on fill corrections (guards the wrong-field-fill risk).

**Done when:** fill coverage > 80% of fields on supported ATS; user completes a real application by clicking apply themselves.

> **End of MVP.** At this point the product delivers its promise end-to-end on desktop.

---

# Post-MVP (add components later)

## Milestone 5 — P1 enhancements
- SCR-5 salary estimation model (title x location x seniority); label estimates distinctly from stated pay (trust-critical).
- SCR-6 requirement extraction into structured tags.
- FEED-4 match score vs user's CV.
- FEED-5 saved filters + email alerts.
- FEED-6 hide/dismiss + "already applied" state.
- APP-6 application tracker (applied/viewed/rejected/interview, manual updates).
- APP-7 cover-note/answer snippets with variable substitution.
- CV-5 multiple CV versions; CV-6 completeness score.
- AUTH-5 account deletion + data export (UU PDP).

## Milestone 6 — Fast-follow: Android app (1–2 quarters)
- MOB-1 feed + filters parity with web.
- MOB-2 in-app WebView injecting the **same** `fill-mappings`; same fill-review; user taps apply.
- MOB-3 CV attach from device storage.
- MOB-4 shared mapping layer (a portal fix ships to both desktop + mobile).
- Sequenced **after** desktop mappings are proven.

## Later (P2 / Phase 2, indicative)
- CV-7 regenerate downloadable CV from profile.
- APP-8 unknown-form heuristic fallback.
- MOB-5 iOS; MOB-6 Android Autofill Service.
- AI CV tailoring, AI screening-answer drafts, batch queue, Firefox/Edge, monetization (freemium).

---

# Non-functional requirements (apply throughout)
- **Performance:** feed < 2s p95 on 4G; filters < 500ms.
- **Scale (MVP):** 10k users, 50k listings, 1k concurrent.
- **Security:** CV/PII encrypted at rest; HTTPS only; auth rate limiting.
- **Privacy (UU PDP No. 27/2022):** consent capture, data deletion + export, no selling CV data without explicit opt-in.
- **Scraper resilience:** per-source circuit breakers; a source failure must not take down the feed.
- **Localization:** Bahasa Indonesia + English; IDR default.

# Key risks to design around
| Risk | Mitigation |
|---|---|
| Scraping ToS / legal + IP blocks | Tier 1+2 only; prefer APIs/partnerships; exclude login-walled at MVP; legal review before launch. |
| Wrong data in wrong field -> broken applications | Per-field confidence flags; visual review before submit; per-ATS mapping tests; fill-correction telemetry. |
| Salary estimates wrong -> trust collapse | MVP is stated-salary only; when added, always label + show confidence range. |
| ATS/portal DOM changes break fills silently | Mapping version per site; fill-failure telemetry; fast mapping-update pipeline. |
| Scraper breaks silently on layout change | Monitoring + freshness SLA alerts per source. |
| Low listing volume -> empty-feed churn | Seed >= 5k listings before signup; Jabodetabek-first. |

# Success metrics (first 90 days)
5,000 registered users; CV->confirmed-profile > 60%; weekly active applier > 25% of WAU; >= 5 applications/active user/week; median listing age at apply < 5 days; track "got interview via platform" from day 1 (north-star input).
