# Auto Applier

Semi-automated, **human-in-the-loop** job application assistant. Upload your CV
once; the browser fills every application for you — **you always click Apply**.
The system never submits. See `PRD.md` and `plan.md`.

## Repository layout
```
/backend                 Go API + (later) scrapers + ingestion workers
/web                     React + TypeScript web app (Vite)
/packages/fill-mappings  Shared, versioned per-portal fill maps (later)
/extension               Chrome MV3 extension (later)
/scripts/verify.sh       Deterministic verification gate
```

## Local development

Prerequisites: Go 1.26+, Node 20+, Docker.

Run the full stack (Postgres + API + web):
```bash
docker compose up --build
# api:  http://localhost:8080/healthz
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

Environment variables (all optional for local dev):
| Var | Purpose |
|---|---|
| `DATABASE_URL` | Postgres DSN. Unset → in-memory repos. |
| `FEED_SEED_COUNT` | Synthetic demo listings seeded on boot (default 200; 0 disables). |
| `CAREERJET_AFFID` | Free Careerjet affiliate id enabling the Tier-1 source (unset → source disabled). |
| `CV_PARSER_URL` | Hosted résumé-parse API (unset → parsing returns 503). |
| `ENFORCE_HTTPS` | Reject plain HTTP + set HSTS (default off for local). |
| `SMTP_ADDR` | STARTTLS-capable SMTP server as `host:port`; required for verification/reset email outside dev mode. |
| `SMTP_USERNAME` / `SMTP_PASSWORD` | Optional SMTP credentials. |
| `SMTP_FROM` | Sender address; required when `SMTP_ADDR` is set. |
| `AUTH_DEV_EXPOSE_TOKENS` | Return verification/reset tokens in API responses for local demos only. |

## Verification gate

Every change must pass the deterministic gate before it is considered done:
```bash
./scripts/verify.sh
```
It auto-detects components and runs build/lint/typecheck/test for each. Exit 0 = green.
