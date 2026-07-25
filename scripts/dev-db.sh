#!/usr/bin/env bash
# dev-db.sh — provision the local Postgres database for Auto Applier development.
#
# Uses a locally-running Postgres (e.g. `brew services start postgresql@18`).
# Creates the `autoapplier` database if it does not exist and prints the
# DATABASE_URL to export. Migrations are applied automatically by the API on
# startup when DATABASE_URL is set.
#
# Usage:
#   ./scripts/dev-db.sh            # create db + print DATABASE_URL
#   eval "$(./scripts/dev-db.sh --export)"   # create db + export into shell
set -euo pipefail

DB_NAME="${AUTOAPPLIER_DB:-autoapplier}"
DB_HOST="${PGHOST:-localhost}"
DB_PORT="${PGPORT:-5432}"
DB_USER="${PGUSER:-$(whoami)}"
URL="postgres://${DB_USER}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"

if ! command -v psql >/dev/null 2>&1; then
  echo "psql not found — install Postgres (e.g. brew install postgresql@18)" >&2
  exit 1
fi

if ! pg_isready -h "$DB_HOST" -p "$DB_PORT" >/dev/null 2>&1; then
  echo "Postgres is not accepting connections on ${DB_HOST}:${DB_PORT}." >&2
  echo "Start it first, e.g.: brew services start postgresql@18" >&2
  exit 1
fi

if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -lqt 2>/dev/null | cut -d '|' -f1 | grep -qw "$DB_NAME"; then
  : # database already exists
else
  createdb -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$DB_NAME"
  echo "created database ${DB_NAME}" >&2
fi

if [ "${1:-}" = "--export" ]; then
  echo "export DATABASE_URL=\"${URL}\""
else
  echo "$URL"
fi
