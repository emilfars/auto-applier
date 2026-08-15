#!/usr/bin/env bash
#
# verify.sh — The Deterministic Feedback Gate for Auto Applier.
#
# Detects which components exist and runs their checks. Components that do not
# exist yet are skipped (not failed), so this is safe to run from day one and
# grows automatically as the repo fills in.
#
# Exit 0 = all present components passed. Non-zero = at least one check failed.
#
# Usage:
#   ./scripts/verify.sh          # run all detected component checks
#   VERBOSE=1 ./scripts/verify.sh
#   CHROME_BIN=/path/to/chrome ./scripts/verify.sh
#
set -euo pipefail

# --- locate repo root (script lives in /scripts) ---
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

# --- pretty output ---
if [ -t 1 ]; then
  C_RESET=$'\033[0m'; C_RED=$'\033[31m'; C_GREEN=$'\033[32m'
  C_YELLOW=$'\033[33m'; C_BLUE=$'\033[34m'; C_BOLD=$'\033[1m'
else
  C_RESET=""; C_RED=""; C_GREEN=""; C_YELLOW=""; C_BLUE=""; C_BOLD=""
fi

FAILED=0
RAN=0
SKIPPED=0
declare -a RESULTS
VERIFY_OUT="${ROOT_DIR}/.verify-out.$$"
trap 'rm -f "${VERIFY_OUT}"' EXIT

section() { printf "\n%s==> %s%s\n" "${C_BOLD}${C_BLUE}" "$1" "${C_RESET}"; }
pass()    { RAN=$((RAN+1));     RESULTS+=("${C_GREEN}PASS${C_RESET}  $1"); printf "  %sPASS%s %s\n" "${C_GREEN}" "${C_RESET}" "$1"; }
skip()    { SKIPPED=$((SKIPPED+1)); RESULTS+=("${C_YELLOW}SKIP${C_RESET}  $1"); printf "  %sSKIP%s %s\n" "${C_YELLOW}" "${C_RESET}" "$1"; }
fail()    { RAN=$((RAN+1)); FAILED=1; RESULTS+=("${C_RED}FAIL${C_RESET}  $1"); printf "  %sFAIL%s %s\n" "${C_RED}" "${C_RESET}" "$1"; }

have() { command -v "$1" >/dev/null 2>&1; }

browser_bin() {
  if [ -n "${CHROME_BIN:-}" ]; then
    [ -x "${CHROME_BIN}" ]
    return
  fi
  local candidate
  for candidate in \
    "/Applications/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing" \
    "${ROOT_DIR}/.cft/chrome-linux64/chrome" \
    "${ROOT_DIR}/.cft/chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing" \
    "${ROOT_DIR}/.cft/chrome-mac-x64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing" \
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
    "/Applications/Chromium.app/Contents/MacOS/Chromium" \
    "/usr/bin/google-chrome" \
    "/usr/bin/google-chrome-stable" \
    "/usr/bin/chromium" \
    "/usr/bin/chromium-browser"; do
    [ -x "${candidate}" ] && return 0
  done
  for candidate in google-chrome google-chrome-stable chromium chromium-browser; do
    have "${candidate}" && return 0
  done
  return 1
}

# run <label> <cmd...>: run a command, record pass/fail (never aborts the script).
run() {
  local label="$1"; shift
  if [ "${VERBOSE:-0}" = "1" ]; then
    if "$@"; then pass "${label}"; else fail "${label}"; fi
  else
    if "$@" >"${VERIFY_OUT}" 2>&1; then
      pass "${label}"
    else
      fail "${label}"
      sed 's/^/      | /' "${VERIFY_OUT}" || true
    fi
  fi
}

# pick the right node package-manager run command for a dir
node_pm() {
  local dir="$1"
  if [ -f "${dir}/pnpm-lock.yaml" ] && have pnpm; then echo "pnpm"; 
  elif [ -f "${dir}/yarn.lock" ] && have yarn; then echo "yarn";
  else echo "npm"; fi
}

# has_script <dir> <script>: true if package.json defines the npm script.
has_script() {
  local dir="$1" name="$2"
  [ -f "${dir}/package.json" ] || return 1
  node -e "process.exit(((require('./${dir}/package.json').scripts)||{})['${name}']?0:1)" 2>/dev/null
}

# run an npm script if it exists, else skip
node_script() {
  local dir="$1" name="$2" label="$3"
  if has_script "${dir}" "${name}"; then
    local pm; pm="$(node_pm "${dir}")"
    run "${label}" bash -c "cd '${dir}' && ${pm} run ${name} --if-present"
  else
    skip "${label} (no '${name}' script)"
  fi
}

check_node_component() {
  local dir="$1" title="$2"
  section "${title}  (${dir})"
  if [ ! -d "${dir}" ]; then skip "${title} — directory not present"; return; fi
  if [ ! -f "${dir}/package.json" ]; then skip "${title} — no package.json"; return; fi
  if ! have node; then fail "${title} — node not installed but ${dir}/package.json exists"; return; fi
  if [ ! -d "${dir}/node_modules" ]; then
    fail "${title} — dependencies not installed (run npm ci in ${dir})"
    return
  fi
  node_script "${dir}" lint       "${title}: lint"
  node_script "${dir}" typecheck  "${title}: typecheck"
  node_script "${dir}" test       "${title}: test"
  node_script "${dir}" build      "${title}: build"
}

# ---------------------------------------------------------------------------
# Backend (Go)
# ---------------------------------------------------------------------------
section "Backend (Go)  (backend/)"
if [ -d backend ] && [ -f backend/go.mod ]; then
  if have go; then
    section "M3 Postgres verification"
    if [ -z "${TEST_DATABASE_URL:-}" ]; then
      fail "M3: TEST_DATABASE_URL is required for Postgres-backed ingest/feed and 50k HTTP p95 checks"
    else
      run "M3: ingest/feed Postgres integration and 50k HTTP p95" \
        bash -c "cd backend && go test -count=1 -p 1 ./internal/ingest ./internal/feed"
    fi

    run "backend: go build"  bash -c "cd backend && go build ./..."
    run "backend: go vet"    bash -c "cd backend && go vet ./..."
    if [ -n "$(cd backend && gofmt -l . 2>/dev/null)" ]; then
      fail "backend: gofmt (unformatted files below)"
      (cd backend && gofmt -l . | sed 's/^/      | /')
    else
      pass "backend: gofmt"
    fi
    if [ -n "${TEST_DATABASE_URL:-}" ]; then
      run "backend: go test (serialized, M3 packages run above)" bash -c \
        "set -euo pipefail; cd backend; packages=\$(go list ./... | grep -Ev '/internal/(ingest|feed)\$'); test -n \"\${packages}\"; go test -count=1 -p 1 \${packages}"
    else
      run "backend: go test (serialized)" bash -c "cd backend && go test -p 1 ./..."
    fi
  else
    fail "backend: Go not installed but backend/go.mod exists"
  fi
else
  skip "backend — not present (no backend/go.mod)"
fi

# ---------------------------------------------------------------------------
# Deployment
# ---------------------------------------------------------------------------
section "Deployment"
if have docker; then
  run "deployment: local compose config" docker compose config --quiet
  run "deployment: production compose config" env \
    CV_ENCRYPTION_KEY=000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f \
    S3_ACCESS_KEY=verify S3_SECRET_KEY=verify-secret \
    docker compose -f docker-compose.yml -f docker-compose.production.yml config --quiet
  run "deployment: Docker/API/S3 smoke" "${ROOT_DIR}/scripts/docker-smoke.sh"
else
  fail "deployment: Docker CLI not installed"
fi

# ---------------------------------------------------------------------------
# Node/TS components
# ---------------------------------------------------------------------------
check_node_component "packages/fill-mappings" "Fill mappings"
check_node_component "web"                     "Web app"
check_node_component "extension"               "Chrome extension"

# ---------------------------------------------------------------------------
# Browser E2E (required when the harness exists)
# ---------------------------------------------------------------------------
section "Browser E2E (Chrome MV3)"
if [ -f scripts/browser-e2e.mjs ]; then
  if ! have node; then
    fail "browser E2E — node is required"
  elif ! browser_bin; then
    fail "browser E2E — Chrome/Chromium not found; install Chrome for Testing or set CHROME_BIN"
  else
    run "browser E2E: real Chrome MV3 Open & Fill" node scripts/browser-e2e.mjs
  fi
else
  skip "browser E2E — scripts/browser-e2e.mjs not present"
fi

# ---------------------------------------------------------------------------
# Android (fast-follow) — only if a Gradle project exists
# ---------------------------------------------------------------------------
section "Android  (android/)"
if [ -d android ] && { [ -f android/gradlew ] || [ -f android/build.gradle ] || [ -f android/build.gradle.kts ]; }; then
  if [ -f android/gradlew ]; then
    run "android: gradle build" bash -c "cd android && ./gradlew --no-daemon assembleDebug -q"
  else
    skip "android — no gradlew wrapper yet"
  fi
else
  skip "android — not present"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
section "Summary"
for line in "${RESULTS[@]:-}"; do
  [ -n "${line}" ] && printf "  %s\n" "${line}"
done
printf "\n  ran=%d  skipped=%d  failed=%s\n" "${RAN}" "${SKIPPED}" "$([ ${FAILED} -eq 0 ] && echo 0 || echo yes)"

if [ "${RAN}" -eq 0 ]; then
  printf "\n%s⚠  No components to verify yet — nothing built.%s\n" "${C_YELLOW}" "${C_RESET}"
  printf "   This is expected before implementation starts; the gate is green.\n"
fi

if [ ${FAILED} -ne 0 ]; then
  printf "\n%sx VERIFY FAILED - fix the checks above before marking work done.%s\n" "${C_RED}${C_BOLD}" "${C_RESET}"
  exit 1
fi

printf "\n%s✓ VERIFY PASSED%s\n" "${C_GREEN}${C_BOLD}" "${C_RESET}"
exit 0
