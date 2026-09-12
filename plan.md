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

## Sourcing-expansion decisions (locked 2026-08-21)

Verified against live endpoints on 2026-08-21. Supersedes the earlier
"Tier 1 sources" finding where needed.

| Topic | Decision |
|---|---|
| Remote listings | **Non-Indonesia roles are acceptable if the role is remote.** Location must render as "Remote"; FEED-2's remote filter applies. Stated-salary-only rule unchanged (drop unstated pay, never estimate). |
| Paid job-board APIs | **Deferred.** Techmap, TheirStack, and jobdata API ($345/mo entry) are out of scope until the free stack is proven insufficient for the 5k gate. |
| Jooble | **Reclassified backfill-only.** Verified 2026-08-21: free key = **500 requests lifetime** (not monthly), per-key, per-country-domain. Do not spend it on recurring sweeps; reserve for initial backfill and gap-filling. |
| Careerjet | **Approved, implement next Tier 1 source.** Confirmed: `careerjet.co.id` serves Indonesian listings; free affiliate `affid`, locale `id_ID`, frequency-limited (no lifetime quota) — safe at 6h polling. Env var `CAREERJET_AFFID` is already scaffolded in README. Verify HTTPS support of the search endpoint during implementation; degrade gracefully when unset (existing keyed-source pattern). |
| ATS public-board harvester | **Approved as a supplement.** Greenhouse (`boards-api.greenhouse.io/v1/boards/{slug}/jobs`), Lever (`api.lever.co/v0/postings/{slug}?mode=json`), Workable widget API, Ashby posting API: public JSON, **no keys, no documented rate limits**. Requires a curated list of company slugs (start from confirmed hits such as Xendit/Greenhouse; expect tens of ID companies, not hundreds — large local hirers use Workday/Glints). Store the slug list as data (e.g. `backend/internal/ingest/atscompanies/*.json`), one entry per ATS. |
| Remote-only boards | **Approved as supplements.** Remotive (`remotive.com/api/remote-jobs`), Jobicy (`jobicy.com/api/v2/remote-jobs`), RemoteOK — free, keyless, verified live. Volume is small and roles are worldwide-remote; tag location "Remote". |
| Kalibrr | Unchanged: primary free source. |

**Milestone placement:** these sources are an **extension of M3 (Ingestion +
feed)** — they exist to close the 5,000-real-listing launch gate and are
tracked under `AC-SCR-7..10` in ACCEPTANCE.md. They are not part of M4.5
(UI-only) and do not reopen M3's completed FEED scope. API keys
(`CAREERJET_AFFID`, Jooble key) are supplied by the owner after
implementation; every source must degrade gracefully when its key is unset.

## Current implementation checkpoint (2026-08-21)

M0-M5, the M3 sourcing extensions (AC-SCR-7..10), and M4.5 web modernization
are complete. The native Chrome MV3 Open & Fill browser gate is green. Launch
still has one operational blocker:

| Blocker | Current state |
|---|---|
| Web → extension data path | Closed. The confirmed profile/CV snapshot is transferred through the web-page bridge into tab-scoped `chrome.storage.session`, with CV bytes in IndexedDB. |
| Live listing threshold | Registration remains closed until Postgres contains at least 5,000 active real listings; synthetic fixtures never count. |

Before declaring MVP complete, seed at least 5,000 active real listings in Postgres and keep registration closed below that threshold. The browser-level Open & Fill test now uses the real web profile/CV path; Docker/API/S3 smoke and non-skipping Postgres verification are enforced by `./scripts/verify.sh`.

## Agentic loop protocol

This project loop uses `gpt-5.6-sol` at `medium` effort for orchestration and
code review, and `gpt-5.6-luna` at `xhigh` effort for bounded implementation.
The preference is project-local and does not apply to other repositories or
ordinary interactive sessions.

Each implementation loop gets a quick Sol review of the complete diff,
callers, scope, failure paths, security/privacy boundaries, and test relevance.
A full end-to-end Sol review runs only at milestone completion, using real
browser, API, database, deployment, and data-lifecycle boundaries. Findings
must be fixed before the milestone is marked done or dependent work begins.

## Tech stack
- **Backend:** Go (net/http or chi/gin), `pgx` for Postgres, job queue: **River** (Postgres-backed) for scrape + parse, `colly`/`chromedp` for scrapers.
- **DB / storage:** Postgres (users, profiles, jobs, applications) + Postgres full-text search (defer OpenSearch until latency demands) + S3-compatible object storage for CV files (encrypted at rest).
- **Frontend:** React + TypeScript (Vite), Mantine v7 (with PostCSS `postcss-preset-mantine`), i18n (id-ID + en, IDR default).
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

## Milestone 4.5 — Web UI modernization + brand theme (pre-M5)
**Goal:** the web app looks like a branded product, not a prototype: a component-library UI with a palette derived from the brand's previous static site.

**Brand source of truth:** screenshots of the previous static site ("Ofrim")
belong in `design/brand/`. `design/brand/palette.md` is the **authoritative,
text-extractable token list** — the palette has been sampled and filled in
(2026-08-21), so no image reading is required: coding agents must read
`palette.md`, not the images.

### Scope
1. **Adopt Mantine v7** (superseding the original Tailwind plan; `postcss-preset-mantine` only). Brand colors stay as CSS-custom-property design tokens defined in `web/src/styles.css` so colors are never hardcoded in components. (2026-08-22 build-out: Mantine components replaced the utility-class layer; wiring the `--brand-*` tokens into Mantine's theme is a follow-up.)
2. **Brand palette tokens** from `design/brand/palette.md`: primary, secondary/accent, neutrals (bg/surface/border/text/muted), semantic success/warning/danger. Define dark (default, refined from current slate look) and light variants via `data-theme` attribute; persist the user's choice.
3. **Layout modernization:** sticky header with nav + language switcher; feed as a responsive card grid with a filters sidebar on desktop (stacked on mobile); skeleton loaders for feed/profile fetches; consistent button/input/chip styling; visible focus states.
4. **Constraint — behavior frozen:** no route/API/logic changes; all existing web tests must keep passing (update selectors only where classes changed). i18n keys untouched; both locales keep parity.

**Done when:** every color in `web/src` resolves to a token defined from `palette.md`; dark + light themes render; existing tests green; visual smoke via `npm run build` + manual check.

---

# Post-MVP (add components later)

## Milestone 5 — P1 enhancements
- **Status: complete (2026-08-21).**
- SCR-5 salary estimation model (title x location x seniority); label estimates distinctly from stated pay (trust-critical).
- SCR-6 requirement extraction into structured tags.
- FEED-4 match score vs user's CV.
- FEED-5 saved filters + email alerts.
- FEED-6 hide/dismiss + "already applied" state.
- APP-6 application tracker (applied/viewed/rejected/interview, manual updates).
- APP-7 cover-note/answer snippets with variable substitution.
- CV-5 multiple CV versions; CV-6 completeness score.
- AUTH-5 account deletion + data export (UU PDP).

## Milestone 5.5 — CV parser service (close the M2 parsing gap)
- **Status: not started.**
- **Why:** M2 ships the hosted-parser *client* (`backend/internal/cv/hosted.go`) and the profile-apply flow, but no parser service exists. With `CV_PARSER_URL` unset, `POST /cv/{id}/parse` returns 503, so the "upload CV → parsed profile" promise (CV-2) is unmet.
- **Goal:** a deployable parser service that accepts the project's documented envelope and returns the normalized shape so CV-2 works end to end.
- **Interface (fixed by `hosted.go`):** `POST` JSON `{filename, content_type, document_base64}` with optional `Authorization: Bearer <CV_PARSER_API_KEY>`; response `{data:{name,email,phone,education[],work_history[],skills[]}}`. HTTPS required except loopback; the envelope means any engine needs a thin adapter.
- **Candidate engines:**
  - `orasik/resume-parser` (**MIT**, open source) — Flask API mapping PDF/DOCX → text → structured JSON via an LLM (OpenRouter by default). Free/self-hostable; can point at a local OpenAI-compatible model (Ollama/vLLM) so CV PII never leaves our infrastructure. LLM output is variable, mitigated by the mandatory confirm-before-apply gate.
  - `affinda/resume-parser` — self-hosted Docker container, fully offline, high-accuracy ML + OCR + skills taxonomy. **Not open source**: free evaluation allowance (1,000 parses, 11,000 with a token) but production requires a paid commercial entitlement.
- **Decision to record:** open-source LLM parser on a locally hosted model (privacy-first, ~free) vs. Affinda's commercial offline container (accuracy, paid). Recommendation: start with `orasik/resume-parser` + a local model, keep the adapter interface so Affinda can be swapped in later.
- **Scope:**
  1. Adapter service exposing the envelope (thin wrapper over the chosen engine).
  2. Deploy on a **separate compute instance/container** (CPU-only for MVP), reachable over HTTPS or loopback; wire `CV_PARSER_URL` + `CV_PARSER_API_KEY`.
  3. Accuracy check against the labeled id + en fixtures (≥90% field accuracy, `AC-CV-2`); the user-confirm gate is unchanged.
  4. Never log CV contents or PII; prefer a locally hosted model over third-party clouds.
- **Out of scope:** regenerating a downloadable CV (CV-7) and AI tailoring.

## Milestone 6 — Fast-follow: Android app (1–2 quarters)
- MOB-1 feed + filters parity with web.
- MOB-2 in-app WebView injecting the **same** `fill-mappings`; same fill-review; user taps apply.
- MOB-3 CV attach from device storage.
- MOB-4 shared mapping layer (a portal fix ships to both desktop + mobile).
- Sequenced **after** desktop mappings are proven.

## Milestone 7 — Sourcing expansion via ATS board discovery (post-MVP)
- **Status: not started.** Deferred until after the desktop and Android milestones.
- **Goal:** grow ingestion well beyond the small curated `backend/internal/ingest/atscompanies/*.json` catalog and add the high-value ATS adapters, without diluting the Jabodetabek-first feed.
- **Method — external projects are references, not dependencies** (all but one are Python; this repo stays Go + its tests, and adapters are re-implemented against the existing `Source` interface):
  - `kalil0321/ats-scrapers` (MIT) — Workday, SmartRecruiters, SuccessFactors, iCIMS, Personio request/response shapes and company inventories.
  - `strelov1/freehire` (MIT, Go) — Go source-adapter and board-catalog patterns; also exposes a keyless public jobs API worth evaluating as one Tier-1 aggregator source.
  - `mherzog4/job-boards` (MIT) — Wayback CDX + urlscan slug discovery, `HEAD` validation, per-host connection pooling, ETag conditional requests, and the "only an unfiltered full run may mark a listing closed" staleness rule.
  - `Feashliaa/job-board-aggregator` (MIT code / CC-BY-NC data) — discovery-pipeline reference only; its company datasets must **not** be used commercially.
- **Scope:**
  1. Go `cmd/discover-boards` tool (Wayback CDX + `HEAD` validation) that grows `atscompanies/*.json` from a handful to thousands of validated slugs, biased toward Indonesian hirers.
  2. New Go adapters for Workday and SmartRecruiters (the highest-value gaps), following the existing `Source` interface, tier policy, and per-source circuit breakers.
  3. Operational hardening for public-board harvesting: per-host connection pooling and ETag conditional requests.
- **Out of scope:** login-walled Tier 3 sources (Indeed/LinkedIn/Glassdoor), and non-commercial datasets. The Tier 1/2-only locked decision is unchanged.
- **Guardrail:** this milestone strengthens supply; it must not be used to fill the feed with irrelevant global volume to clear the 5,000-listing gate.

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
