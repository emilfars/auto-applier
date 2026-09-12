# AGENTS.md — coding-agent handoff

Entry point for any AI coding agent working in this repository. Read this first,
then the four canonical documents below before changing code.

## What this is

Auto Applier — a human-in-the-loop job-application assistant. Users upload a CV,
browse a scraped job feed, and a Chrome MV3 extension fills application forms.
**The system never submits; the user always clicks Apply.** This is the Prime
Directive and an architectural boundary, not a toggle.

## Read before changing code (in order)

1. `plan.md` — approved build plan, locked decisions, milestone scope.
2. `PRD.md` — requirements with IDs (`AUTH-*`, `CV-*`, `SCR-*`, `FEED-*`, `APP-*`).
3. `PROGRESS.md` — **current state**; read sections 1–6 and the top of the Log.
4. `ACCEPTANCE.md` — machine-verifiable checks; a requirement is only "done" when
   its `AC-*` test passes.
5. `design/brand/palette.md` — authoritative design tokens (do not read the images).
6. `.github/copilot-instructions.md` — engineering rules and the module conventions.

Update `PROGRESS.md` and `ACCEPTANCE.md` at the end of every working session.

## Current state (2026-09-13)

- **M0–M5 complete.** Web UI runs on **Mantine v7** (the earlier Tailwind plan was
  superseded; dead Tailwind config/dependency removal is still pending).
- **Launch blocker:** registration stays closed until Postgres holds **≥5,000
  active real listings** (`internal/seed`.DefaultCount). Synthetic listings never count.
- **M5.5 (not started):** implement the CV parser service. Decision recorded in
  `plan.md` — `orasik/resume-parser` (MIT) with an **OpenRouter** model, deployed
  on **AWS** as a separate service; the adapter holds `OPENROUTER_API_KEY` and
  exposes the envelope in `internal/cv/hosted.go`. Without `CV_PARSER_URL`,
  `/cv/{id}/parse` returns 503.
- **M6 (planned):** Android fast-follow. **M7 (planned):** ATS board discovery
  and Workday/SmartRecruiters adapters.
- Ingestion runs on a River scheduler (`INGEST_INTERVAL`, default 6h) and now
  backfills at startup only when the feed is below target; a token-gated
  `POST /ingest/run` triggers a pass on demand (`internal/ingestctl`).

## Build and verify

- Deterministic gate: `./scripts/verify.sh`. A red gate or a skipped required
  check is **not** completion. Postgres-backed checks need `TEST_DATABASE_URL`.
- Backend: `go -C backend build ./...`, `go -C backend test ./...`.
- Web/extension/fill-mappings: `npm run lint && npm run typecheck && npm test && npm run build`.
- Local stack: `docker compose up --build` (Postgres + MinIO + API + web).
- Local Postgres without Docker: `eval "$(./scripts/dev-db.sh --export)"`.
- **Current environment caveat:** the Go toolchain is not installed on the
  machine that last edited this repo, so the gate has not been run for the most
  recent changes. Install Go before trusting the backend build.

## Config quick reference

See the table in `README.md` for the full env list. The ones that gate features:
`DATABASE_URL`, `S3_*` + `CV_ENCRYPTION_KEY`, `SMTP_*`, `CV_PARSER_URL`,
`CAREERJET_AFFID`, `INGEST_INTERVAL`, `INGEST_BACKFILL_TARGET`,
`INGEST_TRIGGER_TOKEN`, `ENFORCE_HTTPS`. Production overrides live in
`docker-compose.production.yml` (forces `ENFORCE_HTTPS=true` and dev flags off).

## Rules that are easy to get wrong

- Never add a form-submission path. Static guards exist in
  `extension/`, `packages/fill-mappings/`, and `backend/internal/safety` — they
  will fail the build if you do.
- Tier 1/2 sources only; never wire a login-walled (Tier 3) source.
- Salary is employer-stated only; never fabricate or present estimates as stated.
- A listing must be re-touched (upsert) each run so the 48h staleness sweep does
  not expire live jobs. Do not add a "skip known listings" persistence shortcut.
- Never log CV contents or PII.
