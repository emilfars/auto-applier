# Auto Applier

Semi-automated, **human-in-the-loop** job application assistant. Upload your CV
once; the browser fills every application for you — **you always click Apply**.
The system never submits. See `PRD.md` and `plan.md`.

## Repository layout
```
/backend                 Go API + ingestion workers and public source adapters
/web                     React + TypeScript web app (Vite)
/packages/fill-mappings  Shared, versioned per-portal fill maps
/extension               Chrome MV3 extension
/scripts/verify.sh       Deterministic verification gate
```

## Local development

Prerequisites: Go 1.26+, Node 20+, Docker.

Run the full stack (Postgres + API + web):
```bash
docker compose up --build
# api:  http://localhost:5173/healthz
# web:  http://localhost:5173
```

Backend only:
```bash
cd backend && go run ./cmd/api    # serves :8080
```

Web only:
```bash
cd web && npm install && npm run dev
```

### Run without Docker (local Postgres)

If you don't have Docker, run against a local Postgres instead:
```bash
brew services start postgresql@18          # or your local Postgres
eval "$(./scripts/dev-db.sh --export)"      # creates the `autoapplier` DB + exports DATABASE_URL
go -C backend run ./cmd/api                 # applies migrations on startup, serves :8080
```
Without `DATABASE_URL` the API falls back to in-memory repos (data does not
persist across restarts). With it set, jobs, profiles, CV metadata, and the
River job queue all persist to Postgres. In another terminal run `cd web &&
npm run dev`; the Vite dev server proxies API calls to `:8080`.

Production TLS terminates at the external reverse proxy. Run Compose with
`docker-compose.production.yml`, keep port 5173 reachable only by that proxy,
and have it send `X-Forwarded-Proto: https`:
```bash
CV_ENCRYPTION_KEY=... S3_ACCESS_KEY=... S3_SECRET_KEY=... \
  docker compose -f docker-compose.yml -f docker-compose.production.yml up -d
```
The production override requires those secrets and enables API HTTPS
enforcement. Direct API access is not published.

Environment variables (all optional for local dev):
| Var | Purpose |
|---|---|
| `DATABASE_URL` | Postgres DSN. Unset → in-memory repos. |
| `FEED_SEED_COUNT` | Synthetic demo listings seeded on boot (default 200; 0 disables). |
| `CAREERJET_AFFID` | Free Careerjet affiliate id enabling the Tier-1 source (unset → source disabled). |
| `CAREERJET_LIMIT` | Maximum Careerjet listings per recurring run (default 5,000 when configured). |
| `JOOBLE_API_KEY` | Jooble partner key for explicit `cmd/seed -real` backfill only; never scheduled. |
| `ATS_PUBLIC_ENABLED` | Enable curated public Greenhouse/Lever/Workable/Ashby boards (default `true`). |
| `REMOTE_SOURCES_ENABLED` | Enable Remotive, Jobicy, and RemoteOK (default `true`). |
| `REMOTE_LIMIT` | Maximum listings per remote source per run (default 100). |
| `INGEST_INTERVAL` | Recurring ingestion interval (default `6h`). |
| `INGEST_BACKFILL_TARGET` | Startup backfill fires while active real listings are below this (default 5000). |
| `INGEST_TRIGGER_TOKEN` | Secret enabling `POST /ingest/run`; unset disables the trigger entirely. |
| `INGEST_TRIGGER_MIN_INTERVAL` | Minimum gap between on-demand triggers (default 1m). |
| `ALERT_INTERVAL` | Saved-filter email alert interval (default 6h; disabled without SMTP). |
| `CV_PARSER_URL` | Hosted résumé-parse API (unset → parsing returns 503). |
| `CV_ENCRYPTION_KEY` | 64 hex characters used to encrypt CV bytes before object storage. |
| `S3_ENDPOINT` | S3-compatible endpoint. Required with `DATABASE_URL`. |
| `S3_ACCESS_KEY` / `S3_SECRET_KEY` | S3-compatible storage credentials. |
| `S3_BUCKET` | CV object bucket. |
| `S3_USE_TLS` | Use TLS to reach object storage (default `true`). |
| `TRUSTED_PROXY_HOPS` | Number of trusted reverse-proxy hops (default `0`). |
| `ENFORCE_HTTPS` | Reject plain HTTP + set HSTS (default off for local). |
| `SMTP_ADDR` | STARTTLS-capable SMTP server as `host:port`; required for verification/reset email outside dev mode. |
| `SMTP_USERNAME` / `SMTP_PASSWORD` | Optional SMTP credentials. |
| `SMTP_FROM` | Sender address; required when `SMTP_ADDR` is set. |
| `AUTH_DEV_EXPOSE_TOKENS` | Return verification/reset tokens in API responses for local demos only. |
| `AUTH_DEV_ALLOW_IN_MEMORY_REGISTRATION` | Local browser E2E only: allow registration without the 5,000-real-listing gate when dev tokens are enabled. |

## Ingestion

With `DATABASE_URL` set, a River scheduler re-ingests the enabled sources every
`INGEST_INTERVAL` (default 6h) and sweeps stale listings hourly. On startup the
API backfills only when the active real-listing count is below
`INGEST_BACKFILL_TARGET` (default 5000), so a healthy deployment does not
re-fetch every source on every restart.

One ingestion pass can also be triggered on demand when `INGEST_TRIGGER_TOKEN`
is set:
```bash
curl -fsS -X POST http://localhost:8080/ingest/run \
  -H "Authorization: Bearer $INGEST_TRIGGER_TOKEN"
# 202 started · 401 bad token · 409 a run is already in progress · 503 trigger disabled
```

## CV parsing

`POST /cv/{id}/parse` is served by the client in `backend/internal/cv/hosted.go`,
which posts `{filename, content_type, document_base64}` to `CV_PARSER_URL` and
expects `{data:{name,email,phone,education[],work_history[],skills[]}}`. The
endpoint returns 503 until `CV_PARSER_URL` points at a running parser service.
Any engine needs a thin adapter that speaks this envelope.

The chosen engine (M5.5, `plan.md`) is `orasik/resume-parser` (MIT) driven by an
**OpenRouter** model, deployed on **AWS** as a separate service that holds
`OPENROUTER_API_KEY`. Set `CV_PARSER_URL` (HTTPS, or loopback) and
`CV_PARSER_API_KEY` on the API. Never log CV contents or PII.

## Verification gate

Every change must pass the deterministic gate before it is considered done:
```bash
./scripts/verify.sh
```
It auto-detects components and runs build/lint/typecheck/test for each. Exit 0 = green.
