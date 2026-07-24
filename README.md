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

## Verification gate

Every change must pass the deterministic gate before it is considered done:
```bash
./scripts/verify.sh
```
It auto-detects components and runs build/lint/typecheck/test for each. Exit 0 = green.
