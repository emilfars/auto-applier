# Copilot Instructions — Auto Applier

Repository rules for AI agents. Read `PRD.md`, `plan.md`, `PROGRESS.md`, and
`ACCEPTANCE.md` before changing code.

## Product boundaries

- **Never submit a job application.** The extension and future mobile WebView
  may fill fields only; the user always reviews and clicks Apply/Submit.
- Never trigger form submission, click a submit control, dispatch a synthetic
  submit event, or submit in the background. Stop and flag conflicting work.
- Mark low-confidence fields `uncertain`; never present them as `filled`.
- Use Tier 1 APIs/partner feeds and Tier 2 public boards only. Do not scrape
  login-walled sources.
- MVP salary is employer-stated only. Any future estimate must be clearly
  labeled and visually distinct.
- Keep MVP Jabodetabek-first, seeker-only, and free. Parsed CV data must be
  reviewed and confirmed before first use.

## Architecture

- Backend: Go, `net/http`, `pgx`, River, PostgreSQL.
- CV objects: encrypted S3-compatible storage; store metadata only in Postgres.
- Web: React + strict TypeScript/Vite; `id-ID` and `en`; IDR by default.
- Extension: Chrome MV3 + strict TypeScript.
- `packages/fill-mappings` is the dependency-light, versioned source of truth
  shared by the extension and future Android WebView. Do not fork its maps.
- Do not change a component's language or framework without a recorded decision.

## Engineering rules

- Go: `gofmt`, `go vet`, table-driven tests, wrapped errors, context-aware DB
  calls, and no panics in request paths.
- TypeScript: strict mode, ESLint/Prettier, no unjustified `any`, and
  discriminated fill states (`filled | uncertain | empty`).
- Bump a portal map version whenever its selectors or fields change; ship tests
  with every map change.
- Never log CV contents or PII. Encrypt CV files at rest, require HTTPS,
  rate-limit auth, and preserve consent, deletion, and export rights.
- Reference PRD/acceptance IDs where useful. Keep commits small and add:
  `Co-authored-by: Copilot App <223556219+Copilot@users.noreply.github.com>`

## Verification

Run `./scripts/verify.sh` before completion or commit. A red gate or skipped
required check is not completion. Add new component checks to the gate; fix code
instead of weakening checks.

## Autonomous agentic loop

Model-agnostic. Two roles — an **orchestrator/reviewer** and a **bounded
implementer** — which may be one agent or two. No specific model is required.

1. Orchestrator selects one ready, unmet criterion from the four project documents.
2. Implementer makes the smallest complete change and its relevant tests without
   expanding scope.
3. Orchestrator reviews the complete diff and callers before commit: scope, locked
   decisions, failure paths, security/privacy, and whether tests exercise
   production behavior rather than only mocks.
4. Fix findings, run the verification gate, update `PROGRESS.md` and
   `ACCEPTANCE.md`, commit, and repeat.

Do not run a full repository review every loop. Before marking a milestone done,
run one end-to-end review across browser/runtime, API, database, deployment, data
lifecycle, security/privacy, recovery, and acceptance thresholds. A milestone
closes only on a **full green `./scripts/verify.sh` with no skipped required
check** (Docker + Postgres + extension-capable Chrome present) — a partial gate
is not completion (see `plan.md` → Launch Readiness). Fix findings before
dependent work begins.

After that review passes and the milestone is committed, end the autonomous
session. Start the next milestone in a fresh session and rebuild context from
the four project documents. Keep one session for all loops within a milestone.
