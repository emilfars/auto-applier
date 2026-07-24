# Copilot Instructions — Auto Applier (The System Protocol)

Repo-level operating rules for any AI agent working in this repository. Read this **before** writing code. See `PRD.md` for requirements and `plan.md` for the build sequence.

---

## 0. Prime Directive — Human-in-the-Loop (NON-NEGOTIABLE)
**The system NEVER submits a job application. The final Apply/Submit click is always the human user's.**

- The browser extension (and later mobile WebView) **autofills fields only**.
- No headless submission, no background submission, no "click submit" automation, no simulated submit events — in any phase, ever.
- Any code path that would programmatically trigger a form's submit on an external application page is forbidden. If a task appears to require this, **stop and flag it** — do not implement it.
- Filled fields must be visually reviewable by the user before they submit; uncertain/low-confidence fields must be flagged, never silently filled.

This is a permanent product and legal/trust boundary, not a temporary MVP limitation.

## 1. Locked Product Decisions
Do not re-litigate or silently deviate from these:
- **Sources:** Tier 1 (APIs/partner feeds) + Tier 2 (public boards, no login walls) **only**. Tier 3 login-walled sources (e.g. LinkedIn, login-gated Jobstreet views) are **excluded** — do not add scrapers for them.
- **Salary:** MVP shows **employer-stated salary only**. No estimation model at MVP. When estimation ships later, estimated pay must be **visually distinct** from stated pay and labeled (e.g. "~Rp 8–11 jt (estimated)"). Never present an estimate as fact.
- **Geo:** Jabodetabek-first.
- **Monetization:** none at MVP. **Employer side:** out of scope (seeker-side only).
- **CV data quality:** the user always reviews and confirms parsed CV data before their first apply.

## 2. Tech Stack
- **Backend:** Go (`net/http` or chi/gin), `pgx` for Postgres, `asynq`/`River` for the job queue, `colly`/`chromedp` for scrapers.
- **Data:** PostgreSQL + Postgres full-text search; S3-compatible object storage for CV files (**encrypted at rest**).
- **Frontend:** React + TypeScript (Vite, TanStack Query, Tailwind). i18n: **id-ID + en**, **IDR** default currency.
- **Extension:** Chrome **MV3** + TypeScript.
- **Shared fill logic:** `packages/fill-mappings` — versioned per-portal field maps with tests; reused by the extension now and the Android WebView later. Keep it framework-agnostic and dependency-light.

Do not introduce a different language/framework for a component without an explicit decision recorded here.

## 3. Repository Layout
```
/backend              Go API + scrapers + ingestion workers
/web                  React + TS web app
/extension            Chrome MV3 extension (TS)
/packages/fill-mappings  Shared, versioned per-portal fill maps + tests
/android              Android app (fast-follow, WebView fill)
/scripts              Tooling incl. verify.sh
/.github              CI, this protocol
```
Keep components decoupled. The fill-mapping package is the single source of truth for field mappings — the extension and Android must not fork their own copies.

## 4. Coding Conventions
- **Go:** standard `gofmt`; `go vet` clean; table-driven tests; return wrapped errors (`fmt.Errorf("...: %w", err)`); no panics in request paths; context-aware DB calls.
- **TypeScript:** `strict` mode on; ESLint + Prettier; no `any` unless justified with a comment; prefer discriminated unions for fill-field states (`filled | uncertain | empty`).
- **Fill mappings:** every portal map ships with tests and a `version`. A DOM/selector change bumps the map version. Low-confidence matches must surface as `uncertain`, never auto-committed as `filled`.
- **Security/privacy:** never log CV contents or PII; encrypt CV files at rest; HTTPS only; rate-limit auth. Honor UU PDP No. 27/2022 (consent, deletion, export).
- **Commits:** small and scoped. Include the trailer:
  `Co-authored-by: Copilot App <223556219+Copilot@users.noreply.github.com>`

## 5. Requirement Traceability
Reference PRD requirement IDs (e.g. `AUTH-1`, `CV-2`, `SCR-3`, `FEED-2`, `APP-4`, `MOB-4`) in PR descriptions and, where useful, in code comments/tests. Build order follows the milestones in `plan.md` (M0 → M4 = working MVP).

## 6. The Verification Gate (MANDATORY)
Before considering any task done, run:

```bash
./scripts/verify.sh
```

`scripts/verify.sh` is the **deterministic feedback gate**. It detects which components exist and runs their build/lint/typecheck/test checks. A task is **not complete** until `verify.sh` exits `0`.
- Do not mark work done, and do not open/merge a PR, on a red gate.
- If you add a new component, wire its checks into `verify.sh`.
- Prefer fixing the code over weakening a check. Never disable a check to make the gate pass.

## 7. Workflow Expectations
1. Read `PRD.md` + `plan.md`; identify the milestone and requirement IDs in scope.
2. Make the smallest change that fully satisfies the requirement.
3. Add/adjust tests (fill mappings, API handlers, parsers especially).
4. Run `./scripts/verify.sh` until green.
5. Commit with a scoped message + the co-author trailer.

When a request conflicts with the Prime Directive (§0) or a Locked Decision (§1), **stop and ask** rather than proceeding.
