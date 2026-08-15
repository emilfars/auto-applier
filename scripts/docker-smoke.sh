#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

if ! command -v docker >/dev/null 2>&1; then
  printf 'Docker smoke failed: Docker CLI is not installed\n' >&2
  exit 1
fi
if ! command -v curl >/dev/null 2>&1; then
  printf 'Docker smoke failed: curl is not installed\n' >&2
  exit 1
fi
if ! command -v go >/dev/null 2>&1; then
  printf 'Docker smoke failed: Go is not installed\n' >&2
  exit 1
fi
if ! docker info >/dev/null 2>&1; then
  printf 'Docker smoke failed: Docker daemon is unavailable\n' >&2
  exit 1
fi

export COMPOSE_PROJECT_NAME="auto-applier-smoke-$$"
export COMPOSE_DB_PORT=15432
export COMPOSE_S3_PORT=19000
export COMPOSE_WEB_PORT=15173
export CV_ENCRYPTION_KEY=000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
export S3_ACCESS_KEY=autoapplier
export S3_SECRET_KEY=autoapplier-dev-secret
export PUBLIC_URL="http://127.0.0.1:${COMPOSE_WEB_PORT}"
export PROXY_HEADER='X-Forwarded-Proto: https'

COMPOSE=(docker compose -f "${ROOT_DIR}/docker-compose.yml" -f "${ROOT_DIR}/docker-compose.production.yml")
SMOKE_DIR="${ROOT_DIR}/.docker-smoke.$$"
mkdir -p "${SMOKE_DIR}"

cleanup() {
  status=$?
  "${COMPOSE[@]}" down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf "${SMOKE_DIR}"
  exit "${status}"
}
trap cleanup EXIT

fail() {
  printf 'Docker smoke failed: %s\n' "$1" >&2
  exit 1
}

wait_for_health() {
  local body
  for _ in $(seq 1 60); do
    body="$(curl -fsS --max-time 3 -H "${PROXY_HEADER}" "${PUBLIC_URL}/healthz" 2>/dev/null || true)"
    case "${body}" in
      *'"status":"ok"'*|*'"status": "ok"'*) return 0 ;;
    esac
    sleep 2
  done
  fail "API health did not become ready through the nginx public path"
}

"${COMPOSE[@]}" up --build --detach >/dev/null || fail "compose stack failed to start"
wait_for_health

# Registration is intentionally closed until real listings are seeded. These
# fixtures are synthetic smoke data only and can never satisfy that gate.
"${COMPOSE[@]}" exec -T db psql -v ON_ERROR_STOP=1 -U autoapplier -d autoapplier -Atqc \
  "INSERT INTO jobs (source, source_url, dedup_key, title, company, location, remote, salary_currency, requirements, posted_at, synthetic)
   SELECT 'smoke', 'https://smoke.invalid/job/' || g, 'smoke-' || g, 'Smoke Role', 'Smoke Company', 'Jakarta', false, 'IDR', '[]'::jsonb, now(), true
   FROM generate_series(1, 5000) AS g;" >/dev/null ||
  fail "smoke listings could not be seeded"

SYNTHETIC_COUNT="$("${COMPOSE[@]}" exec -T db psql -v ON_ERROR_STOP=1 -U autoapplier -d autoapplier -Atqc \
  "SELECT count(*) FROM jobs WHERE stale = false AND synthetic = true" | tr -d '\r\n')" ||
  fail "smoke fixture count could not be checked"
[ "${SYNTHETIC_COUNT}" = "5000" ] || fail "smoke fixtures were not all marked synthetic"
REAL_COUNT="$("${COMPOSE[@]}" exec -T db psql -v ON_ERROR_STOP=1 -U autoapplier -d autoapplier -Atqc \
  "SELECT count(*) FROM jobs WHERE stale = false AND synthetic = false" | tr -d '\r\n')" ||
  fail "real listing count could not be checked"
[ "${REAL_COUNT}" = "0" ] || fail "synthetic fixtures contributed to the real listing count"

EMAIL="smoke-$(date +%s)-$$@example.invalid"
SESSION_TOKEN="smoke-session-$(date +%s)-$$"
MARKER='auto-applier-smoke-marker'
PLAINTEXT="$(printf '%%PDF-1.7\n%s\n' "${MARKER}")"
REGISTER_STATUS="$(curl -sS --max-time 10 -o "${SMOKE_DIR}/register.json" -w '%{http_code}' \
  -H "${PROXY_HEADER}" -H 'Content-Type: application/json' \
  -d "{\"email\":\"${EMAIL}\",\"password\":\"smoke-password-123\",\"consent\":true}" \
  "${PUBLIC_URL}/auth/register")" ||
  fail "registration gate request failed through the nginx public path"
[ "${REGISTER_STATUS}" = "503" ] || fail "registration opened with synthetic fixtures"

# Provision only a verified smoke account and an already-authenticated session
# directly in the disposable Postgres database. This does not add a production
# auth bypass or expose development tokens.
SMOKE_USER_ID="$("${COMPOSE[@]}" exec -T db psql -v ON_ERROR_STOP=1 -U autoapplier -d autoapplier -Atqc \
  "INSERT INTO users (email, password_hash, verified, consent_at)
   VALUES ('${EMAIL}', NULL, true, now()) RETURNING id::text" |
  tr -d '\r\n')" || fail "smoke user could not be provisioned"
[ -n "${SMOKE_USER_ID}" ] || fail "smoke user provisioning returned no id"
"${COMPOSE[@]}" exec -T db psql -v ON_ERROR_STOP=1 -U autoapplier -d autoapplier -Atqc \
  "INSERT INTO sessions (token, user_id, expires_at)
   VALUES (encode(digest('${SESSION_TOKEN}', 'sha256'), 'hex'), '${SMOKE_USER_ID}', now() + interval '1 hour')" >/dev/null ||
  fail "smoke session could not be provisioned"

PAYLOAD_BYTES="$(printf '%s' "${PLAINTEXT}" | wc -c | tr -d ' ')"
printf '%s' "${PLAINTEXT}" |
  curl -fsS --max-time 10 -H "${PROXY_HEADER}" -H "Cookie: session=${SESSION_TOKEN}" \
    -F 'file=@-;filename=smoke.pdf;type=application/pdf' "${PUBLIC_URL}/cv" \
    >"${SMOKE_DIR}/upload.json" ||
  fail "CV upload failed through the nginx public path"
grep -Fq "\"filename\":\"smoke.pdf\"" "${SMOKE_DIR}/upload.json" ||
  fail "CV upload response did not contain the expected metadata"
grep -Fq "\"size_bytes\":${PAYLOAD_BYTES}" "${SMOKE_DIR}/upload.json" ||
  fail "CV upload response contained an unexpected size"

curl -fsS --max-time 10 -H "${PROXY_HEADER}" -H "Cookie: session=${SESSION_TOKEN}" \
  "${PUBLIC_URL}/cv" >"${SMOKE_DIR}/cv-before.json" ||
  fail "CV listing failed through the nginx public path"
grep -Fq '"filename":"smoke.pdf"' "${SMOKE_DIR}/cv-before.json" ||
  fail "uploaded CV was not listed before API restart"

"${COMPOSE[@]}" restart api >/dev/null || fail "API restart failed"
wait_for_health
curl -fsS --max-time 10 -H "${PROXY_HEADER}" -H "Cookie: session=${SESSION_TOKEN}" \
  "${PUBLIC_URL}/cv" >"${SMOKE_DIR}/cv-after.json" ||
  fail "CV listing failed after API restart"
grep -Fq '"filename":"smoke.pdf"' "${SMOKE_DIR}/cv-after.json" ||
  fail "uploaded CV metadata did not persist across API restart"

OBJECT_KEY="$("${COMPOSE[@]}" exec -T db psql -v ON_ERROR_STOP=1 -U autoapplier -d autoapplier -Atqc \
  "SELECT object_key FROM cv_files WHERE filename = 'smoke.pdf' ORDER BY created_at DESC LIMIT 1" |
  tr -d '\r\n')" || fail "uploaded object key could not be read"
case "${OBJECT_KEY}" in
  cv/*) ;;
  *) fail "uploaded object key was invalid" ;;
esac

(
  cd "${ROOT_DIR}/backend"
  TEST_S3_ENDPOINT="127.0.0.1:${COMPOSE_S3_PORT}" \
  TEST_S3_ACCESS_KEY="${S3_ACCESS_KEY}" \
  TEST_S3_SECRET_KEY="${S3_SECRET_KEY}" \
  TEST_S3_BUCKET=cv-files \
  TEST_S3_OBJECT_KEY="${OBJECT_KEY}" \
  TEST_S3_PLAINTEXT="${PLAINTEXT}" \
  TEST_S3_ENCRYPTION_KEY="${CV_ENCRYPTION_KEY}" \
  go test -count=1 -run '^TestS3StorePersistsEncryptedBytes_AC_NFR_SEC$' ./internal/storage
) >/dev/null || fail "S3 object did not persist as encrypted ciphertext"

printf 'Docker smoke passed: nginx health, API CV flow, persistent encrypted S3 object\n'
