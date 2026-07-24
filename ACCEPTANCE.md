# ACCEPTANCE.md — Machine-Verifiable Test Specifications

> The contract for "done." Every requirement below maps to an **automated check** that
> `./scripts/verify.sh` (and/or CI) runs. A feature is complete only when its acceptance
> tests exist, are wired into the gate, and pass.
>
> **Rule:** no requirement is "done" on prose alone. If it isn't machine-verified, it isn't done.

## How to read this file
- **ID** — PRD requirement (see `PRD.md`).
- **Acceptance criteria** — observable, binary outcome.
- **Verification** — the concrete automated test/command that proves it.
- **Test ID** — stable id used in test names (e.g. `AC-AUTH-1`) so results are traceable.
- **Status** — ⬜ not written · 🟡 written/failing · ✅ passing.

Test-type conventions:
- **unit** — Go `*_test.go` / TS `*.test.ts` (Vitest/Jest).
- **integration** — API + ephemeral Postgres (testcontainers/docker), HTTP-level.
- **e2e** — Playwright (web) / extension harness against fixture ATS pages.
- **static** — build/vet/gofmt/lint/typecheck (already enforced by `verify.sh`).

---

## 0. Global gates (always enforced by verify.sh)
| Test ID | Criteria | Verification | Status |
|---|---|---|---|
| AC-GATE-1 | Backend compiles | `go build ./...` in `/backend` | ✅ |
| AC-GATE-2 | Backend vet + format clean | `go vet ./...`, `gofmt -l` empty | ✅ |
| AC-GATE-3 | All Go tests pass | `go test ./...` | ✅ |
| AC-GATE-4 | TS components typecheck | `npm run typecheck` per package | ⬜ |
| AC-GATE-5 | TS components lint clean | `npm run lint` per package | ⬜ |
| AC-GATE-6 | TS unit tests pass | `npm run test` per package | ⬜ |
| AC-GATE-7 | All buildable components build | `npm run build` per package | ⬜ |
| AC-GATE-8 | `verify.sh` exits 0 | `./scripts/verify.sh; echo $?` == 0 | ✅ (0 components) |

## 0.1 Prime-directive guard (NON-NEGOTIABLE)
| Test ID | Criteria | Verification | Status |
|---|---|---|---|
| AC-SAFE-1 | Extension code never calls `form.submit()` / `.click()` on a submit control / dispatches a synthetic submit on external pages | static scan test in `/extension` + `/packages/fill-mappings` asserts no submit-triggering APIs in fill paths (grep-based unit test, allowlist-free) | ⬜ |
| AC-SAFE-2 | Fill result exposes per-field state `filled | uncertain | empty`; uncertain fields are never reported as filled | unit test on fill engine | ⬜ |
| AC-SAFE-3 | No backend endpoint performs application submission to a third-party portal | integration test: route table contains no submit-proxy; code scan test | ⬜ |

---

## 1. Accounts (M1)
| Test ID | ID | Criteria | Verification | Status |
|---|---|---|---|---|
| AC-AUTH-1 | AUTH-1 | Register with email+password creates unverified user; verification token issued | integration: POST /auth/register → 201, user row `verified=false`, token persisted | ✅ |
| AC-AUTH-1b | AUTH-1 | Unverified user cannot access gated routes | integration: gated GET → 403 pre-verify, 200 post-verify | ✅ |
| AC-AUTH-2 | AUTH-2 | Google OAuth callback creates/links account and issues session | integration with mocked OIDC provider | ✅ |
| AC-AUTH-3 | AUTH-3 | Password reset: request → token → set new password → old password rejected | integration flow | ✅ |
| AC-AUTH-4 | AUTH-4 | Login issues session token; logout invalidates it; token expiry enforced | integration | ✅ |
| AC-AUTH-4b | AUTH-4 | Auth endpoints rate-limited after N failures | integration: N+1th attempt → 429 | ✅ |

## 2. CV & Profile (M2)
| Test ID | ID | Criteria | Verification | Status |
|---|---|---|---|---|
| AC-CV-1 | CV-1 | Accept PDF/DOCX ≤5MB; reject other types and >5MB | integration: matrix of uploads → 201 / 415 / 413 | ✅ |
| AC-CV-1b | CV-1 | Stored CV object is encrypted at rest (ciphertext != plaintext bytes) | integration: read raw object store, assert not equal to source | ✅ |
| AC-CV-2 | CV-2 | Parse populates contact/education/work/skills for a fixture CV with ≥90% field accuracy | unit/integration against labeled fixtures (id + en) | ✅ |
| AC-CV-3 | CV-3 | All parsed fields are editable and persist | integration: PATCH profile → GET reflects change | ✅ |
| AC-CV-4 | CV-4 | Added-info fields (expected salary, notice period, work auth, relocation, preferred locations, employment type) persist and validate | integration | ✅ |
| AC-CV-5 | CV — | **Confirm-before-apply gate:** profile has `confirmed=false` until user confirms; fill flow refuses to arm while unconfirmed | integration + extension unit: arming blocked when `confirmed=false` | ✅ |

## 3. Ingestion & Feed (M3)
| Test ID | ID | Criteria | Verification | Status |
|---|---|---|---|---|
| AC-SCR-1 | SCR-1 | Scheduler runs a source scraper and persists normalized jobs | integration against a local fixture HTML server | ✅ |
| AC-SCR-1b | SCR-1 | **Circuit breaker:** one failing source does not fail the run or empty the feed | integration: inject source error → other sources still ingest, feed non-empty | ✅ |
| AC-SCR-2 | SCR-2 | Normalized job has title, company, location, stated_salary, requirements, seniority, employment_type, posted_at, source_url | unit on normalizer + schema assertion | ✅ |
| AC-SCR-2b | SCR-2 | Only Tier 1+2 sources present; no login-walled source configured | unit: source registry excludes Tier 3 | ✅ |
| AC-SCR-3 | SCR-3 | Duplicate listings across sources collapse to one canonical job | unit on dedup key + integration | ✅ |
| AC-SCR-4 | SCR-4 | Listing removed/expired at source is marked stale within 48h window logic | unit on staleness rule with clock injection | ✅ |
| AC-FEED-1 | FEED-1 | Feed returns paginated cards; pay shows **stated only**, labeled; no estimated values at MVP | integration + unit: response contains no `estimated` pay field | ⬜ |
| AC-FEED-2 | FEED-2 | Filters (pay, location incl. remote, skills, YoE, employment type, posted date, source) return correct subset | integration: seeded dataset → filtered counts match expected | ⬜ |
| AC-FEED-2b | FEED-2 | Filter query p95 < 500ms on 50k-listing seed | perf test (k6/Go bench) asserting p95 threshold | ⬜ |
| AC-FEED-3 | FEED-3 | Free-text search matches on title + company | integration | ⬜ |
| AC-FEED-1p | NFR | Feed p95 < 2s on simulated 4G profile | perf test (may run in CI perf stage, not per-commit) | ⬜ |
| AC-SEED-1 | plan | Seed job present that loads ≥5,000 listings before signup opens | integration: seeded count ≥ 5000 | ⬜ |

## 4. Extension Autofill (M4 — core value)
| Test ID | ID | Criteria | Verification | Status |
|---|---|---|---|---|
| AC-MAP-1 | APP-3 | Each supported portal map (Jobstreet, Glints, Kalibrr, Greenhouse, Workable, Lever, generic) has a `version` and passing map tests | unit per map in `/packages/fill-mappings` | ⬜ |
| AC-APP-1 | APP-1 | On a fixture ATS page, common fields (name, contact, education, work history, expected salary, notice period, links) fill correctly | e2e against saved fixture DOMs | ⬜ |
| AC-APP-1b | APP-1 | Fill coverage ≥80% of mappable fields on each supported fixture | e2e metric assertion | ⬜ |
| AC-APP-2 | APP-2 | CV file auto-attaches on file-upload fields | e2e on fixture with file input | ⬜ |
| AC-APP-4 | APP-4 | Filled fields marked `filled`, uncertain marked `uncertain`; **no submit is ever triggered** | e2e asserts DOM highlight states + no submit event fired (ties to AC-SAFE-1) | ⬜ |
| AC-APP-5 | APP-5 | "Open & Fill" from feed arms the extension on the source page | e2e | ⬜ |
| AC-APP-TEL | plan | Fill-correction events are emitted when user overrides a filled value | unit on telemetry emitter | ⬜ |

## 5. Post-MVP specs (write when milestone starts)
| Test ID | ID | Criteria | Verification | Status |
|---|---|---|---|---|
| AC-SCR-5 | SCR-5 | Estimated salary labeled distinctly from stated; never shown as stated | unit + e2e on rendering | ⬜ |
| AC-FEED-4 | FEED-4 | Match score computed from requirement overlap vs CV | unit | ⬜ |
| AC-FEED-5 | FEED-5 | Saved filters persist; alert fires on new matching listing | integration | ⬜ |
| AC-APP-6 | APP-6 | Tracker records "form filled" and user-confirmed "submitted"; statuses transition validly | integration | ⬜ |
| AC-AUTH-5 | AUTH-5 | Account deletion removes PII; data export returns complete user data (UU PDP) | integration | ⬜ |
| AC-MOB-4 | MOB-4 | Android WebView consumes the **same** `fill-mappings` package (no forked copy) | build/dep test: android references shared package version | ⬜ |

## 6. Non-functional acceptance
| Test ID | Criteria | Verification | Status |
|---|---|---|---|
| AC-NFR-SEC | CV/PII encrypted at rest; HTTPS enforced | integration (AC-CV-1b) + config test rejecting plain HTTP | ⬜ |
| AC-NFR-PRIV | Consent captured at signup; deletion + export available | integration (AC-AUTH-5) + signup consent test | ⬜ |
| AC-NFR-I18N | UI strings resolve for `id-ID` and `en`; currency defaults to IDR | unit: no missing-key in either locale bundle | ✅ |
| AC-NFR-SCALE | System handles seed of 50k listings without feed regression | perf stage | ⬜ |

---

## Traceability & enforcement
- Every `AC-*` test name embeds its Test ID so CI output maps 1:1 to this file.
- `scripts/verify.sh` is the local gate; CI runs the same plus perf/e2e stages that are too slow per-commit.
- When you implement a requirement: (1) write its `AC-*` test, (2) make it pass, (3) flip Status here to ✅, (4) update `PROGRESS.md`.
- Adding a new requirement means adding a row here **first** (spec before code).
