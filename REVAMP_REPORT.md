# REVAMP_REPORT.md: M8 Revamp, for owner review

Branch `revamp/m8`, 36 commits on top of `main` (bc80b09). Nothing was pushed, merged or opened as a PR. Phase boundaries are the local tags `m8-R0-done` … `m8-R6-done`. Run date: 2026-10-12, unattended.

**Verdict in one paragraph.** The app now runs correctly for a signed-in user (it did not before: every signed-in page went blank once the M5 endpoints were reachable), the web UI and the extension's review panel are rebuilt on the brand tokens with every state in both locales, and all blocking evals are green except three that could not be made green inside this run's limits: E-FUNC (AUTH-2 has no production implementation, a locked deferral), the 50k-listing p95 part of E-PERF (603–656 ms locally against 500 ms, unchanged code, pre-existing), and E-COV for a few files where the coverage tool cannot measure. Read sections 4 and 5 before merging.

## 1. What changed, per phase

| Phase | Result |
|---|---|
| **R0 Baseline and audit** | Preflight passed (Docker, Postgres test container on 127.0.0.1:5442, Chrome for Testing). Full gate green on `main`. Baselines captured. Findings F0–F8 (functional) and S1–S7 (simplification) classified. Added `scripts/changed-coverage.mjs` (E-COV) and `scripts/ui-matrix.mjs` (E-STATE). `verify.sh` was not touched (protected), so coverage is a separate script. |
| **R1 Functional fixes** | **F0** signed-in app rendered a blank page: Go encodes empty lists as `null`, the web client read `.length` on them. Fixed in `web/src/api/m5.ts`. **F1** `web/nginx.conf` did not proxy `/saved-filters`, `/applications`, `/snippets`, `/jobs`, so every M5 feature was dead in the compose/production stack; fixed, with a test that fails when a client prefix is missing. **F2** `apiFetch` threw `SyntaxError` on non-JSON error bodies. **F3** the browser E2E now filters the feed, proxies the M5 routes and fails on any console error or warning (that assertion immediately caught a `/favicon.ico` 404). |
| **R2 Simplification** | Deleted six unreachable ingest constructors, the test-only in-memory `ArmingStore` and its handlers (the AC-APP-5 flow test now runs against the production `SessionArmingStore`), merged two copied page-bridge promises, extracted a repeated removal sequence, unexported private symbols. |
| **R3 Design direction** | `anti-slop/audit-001-2026-10-12.md` (20 findings) and `DESIGN.md`. The kickoff brief was an unfilled placeholder, so the direction is agent-authored from `PRD.md`, `plan.md` and `palette.md`, and says so. Dials ENERGY 1 / RHYTHM 2 / MOTION 1. |
| **R4 UI revamp** | Brand palette actually applied (it was not: the Mantine resolver lost on CSS specificity and the app rendered Mantine grey). Four hash-switched views with the feed first; labeled bottom nav on phones. Every screen has loading, empty, error and populated states in `id-ID` and `en`. Education and work history are form rows instead of raw JSON. Localized validation and errors through one shared helper. Extension review panel is a legend that matches the field outlines. Lighthouse 99 / 100. |
| **R5 Hardening** | Error boundary; expired session signs the UI out; failed sign-out is reported; double-save guard; gzip limited to static assets. |
| **R6 Review and close** | Evals re-run from a clean checkout (`git worktree`, `npm ci`). `ponytail-review` on the cumulative diff: two findings fixed, four reported below. |

## 2. Evals against baseline

Final numbers are from the clean checkout at the last code commit.

### Per-loop (blocking)

| Eval | Result | Evidence |
|---|---|---|
| E-GATE | ✅ every loop | `./scripts/verify.sh`: ran=21, skipped=1 (android, not present, M6 non-goal), failed=0. Baseline identical. |
| E-SAFE | ✅ every loop | Static guards in the gate; `browser-e2e.mjs`: `submitted=false`, fixture submit listener never fired. `backend/internal/safety`, `extension/src/guard*.ts`, `safety.test.ts`: 0 diff lines vs `main`. |
| E-COV | ✅ with a caveat | Lowest loop 91.4%, most 100%. Caveat in section 5: the tool cannot measure four files. |
| E-SCOPE | ✅ | R4-4 exceeded 400 changed lines before commit and was split into two gated commits; R4-6 and R4-7 were cut in two up front for the same reason. |
| E-FROZEN | ✅ | 41 routes identical to baseline; `backend/internal/db`, `auth`, `cv`, `storage`, `packages/fill-mappings/src/maps`, `extension/manifest.json`, `palette.md`, `verify.sh`, workflows, compose files: no diff. Extension message type declarations unchanged; only test-only handlers were deleted. |

### Per-phase

| Eval | Baseline | Final | Status |
|---|---|---|---|
| E-FUNC (R1) | n/a | every PRD ID maps to an `AC-*` test (section 3); AUTH-2 mock-only | ❌ partial |
| E-JOURNEY (R1, R6) | E2E registered through `fetch`, no filter step, no console check | real form: sign up → verify → sign in → upload CV → fill profile incl. rows → confirm → filter feed → Open & Fill → review states → no submit → export → delete; 0 console errors or warnings | ✅ |
| E-SIMPLE (R2) | backend 12,373 · extension/src 1,792 · fill-mappings 642 · `deadcode -test` 6 · clones 22 | at R2 close: 12,342 · 1,667 · 642 · 0 · 20 | ✅ |
| E-BEHAVE (R2) | Go 238 · ext 82 · web 105 · maps 70 | Go 238 (0 lost) · ext 76 (7 removed with `ArmingStore`, listed in PROGRESS R2-3; 1 added) · web 212 · maps 70 | ✅ |
| E-SLOP (R4) | no direction, no gate | `anti-slop/delivery-gate-2026-10-12.md`: 9 sections, 0 FAIL | ✅ |
| E-A11Y (R4) | Lighthouse accessibility 90; white on primary 3.94:1 | `tokens.test.ts` checks every text/state/button/border/focus pair in both themes; Lighthouse 100; keyboard run 8 steps | ✅ (keyboard run excludes file choice) |
| E-STATE (R4) | 288 cells: 132 captured, 120 missing | 288 cells: 252 captured, 36 not applicable, 0 missing, 0 page overflow, 0 clipped, 0 off-brand, 0 console errors | ✅ |
| E-PERF (R4, R5) | Lighthouse performance 70 (measured uncompressed); JS 468,321 B; p95 687 ms (2026-09-20 log) | Lighthouse performance 99, accessibility 100, best-practices 96 (3 runs; FCP 1.7 s, LCP 2.0 s, CLS 0); initial JS 354,099 B (−24.4%), total JS 471,563 B (+0.7%); p95 652 ms | 🟡 Lighthouse and bundle ✅, p95 ❌ |
| E-PROD (R5) | not measured as a checklist at R0; known failing then: error boundary, expired session, double submit, missing states | 12 of 12 PASS, evidence in the PROGRESS R5 close line | ✅ |

Other numbers: Go statement coverage 69.9% → 70.0%; extension 74.9% → 76.5%; fill-mappings 98.6% → 98.6%; web reported 79.7% → 96.3% but the two web figures are **not comparable** (the baseline run collapsed most files to one line; see section 5). `web/src` non-test LOC grew 3,036 → 3,545: states, localization and the row editor are new code; R2's size target excluded web components by plan. Extension `content-entry.js` 22,221 → 24,426 B (+9.9%).

### Loop health

| Phase | Loops | First-pass gate | Reverted | Stop events | Review findings |
|---|---|---|---|---|---|
| R0 | 4 | 4/4 | 0 | 0 | 0 |
| R1 | 4 | 4/4 | 0 | 0 | 1 |
| R2 | 6 | 6/6 | 0 | 0 | 2 |
| R3 | 1 | 1/1 | 0 | 0 | 0 |
| R4 | 14 | 14/14 | 0 | 0 | 12 |
| R5 | 4 | 4/4 | 0 | 0 | 2 |
| R6 | 1 | 1/1 | 0 | 0 | 6 |

No loop was reverted and no halt condition tripped. "First-pass" counts the full gate; several loops needed a second targeted run (unit tests, E2E, matrix) before the gate was started.

## 3. E-FUNC traceability

"Production path" means the test drives the shipped code and fakes only the third party at the network edge.

| PRD ID | AC test | Proven on |
|---|---|---|
| AUTH-1, 3, 4 | AC-AUTH-1/1b/3/4/4b | Production path: HTTP handlers on memory and Postgres; SMTP faked at the socket. Sign-up, verify and sign-in also run through the real form in Chrome. |
| **AUTH-2 (P0)** | AC-AUTH-2 | **Mock only.** No production `OIDCProvider` exists. Deferred (D1). |
| AUTH-5 | AC-AUTH-5/5b | Production path, real Chrome download and deletion. |
| CV-1, 3, 4 | AC-CV-1/1b/3/4/5 | Production path incl. S3 (MinIO) in the Docker smoke and the browser E2E. |
| CV-2 | AC-CV-2/2b | Production path against a mock OpenRouter; live accuracy not run (D2). |
| CV-5, 6 | AC-CV-5, AC-CV-6 | API and web tests; primary CV and completeness render in the matrix. |
| SCR-1..4 | AC-SCR-1..4, 7..14 | Production path against fixture servers and Postgres. No live ingestion was run (off limits). |
| SCR-5, 6 · FEED-4, 5, 6 · APP-6, 7 | section 6 rows | API tests on the memory store; **the web side is proven against mocked API calls plus a real-browser render**, not a full journey (a tracker status change was not driven in a browser). |
| FEED-1..3 | AC-FEED-1/2/3 | Postgres integration; filter narrowing in real Chrome. |
| FEED-2b | AC-FEED-2b | ❌ locally: 652 ms vs 500 ms (D12). |
| APP-1, 2, 4, 5 | AC-APP-1/1b/2/4/5 | Native Chrome MV3 run on the Lever fixture. |
| APP-3 | AC-MAP-1 | Saved fixture DOMs per portal. Live portals are not verified (login walls, ToS). |
| MOB-1..4 | AC-MOB-4 | Not built (M6, non-goal). |

## 4. Deferred (M8)

| ID | Item | Why deferred |
|---|---|---|
| D1 | AUTH-2 Google OAuth has no production provider | Locked decision; `backend/internal/auth` is protected. |
| D2 | CV-2 live parser accuracy | Needs a real `OPENROUTER_API_KEY`. |
| D3 | AC-FEED-1p (4G p95), AC-NFR-SCALE | Held by locked decision. |
| D4 | `ingest.HTMLSource` (232 lines) is reached only by tests | Deleting it removes the AC-SCR-1 fixture proof; product decision. |
| D5 | Source filter cannot list per-board ATS sources | Needs a new route or field (frozen). |
| D6 | Dead code in `auth` and `storage`; their `writeJSON` copies | Protected paths. |
| D7 | `discover.NewDiscoverer` / `Validate` unreachable from the CLI | Test seam of a tool this run may not execute. |
| D8 | E-COV not wired into `verify.sh` | `verify.sh` is protected; run `node scripts/changed-coverage.mjs <base>`. |
| D9 | Empty Go slices encode as JSON `null` | Frozen JSON shape; the web client normalizes. |
| D10 | Feed source filter lists the synthetic seed's sources (`glints`, `jobstreet`, `kemnaker`, `company-career`), not real ones | Product decision: which data set the filter serves, with no route that lists sources. |
| D11 | One shared `writeJSON` helper | About 200 call-site edits for roughly 20 lines saved. |
| D12 | 50k-listing feed p95 above 500 ms locally | Pre-existing; needs a search index (schema is frozen) or a quieter machine. `backend/internal/feed` is byte-identical to `main`. |

## 5. Not verified, and known limits

- **M5 through the real nginx container.** The proxy fix is proven by a config test and by the E2E's own proxy, and gzip was checked against the real `nginx:1.27-alpine` image, but no M5 request was sent through the compose stack's nginx.
- **E-COV tool limits.** vitest's v8 lcov collapses `ErrorBoundary.tsx`, `main.tsx`, `api/account.ts` and `api/download.ts` to one line record, so changed lines in those files are not really measured (R5-1 reported 1/1). The R0 web coverage baseline (79.7%, 436/547 statements) used a much smaller denominator than the final run (2237/2322) and should be ignored. Go coverage uses `-coverpkg=./...` and whole coverage blocks, so a changed line inside a partly executed block counts as covered.
- **Extension review panel was never screenshotted.** Its DOM and counts are asserted in real Chrome and in happy-dom; its look on a real employer page is unreviewed by eye. Its language follows the browser because the frozen message contract carries no locale.
- **Keyboard run** covers 8 steps but not choosing a CV file (OS dialog) or confirming a complete profile.
- **Screenshot matrix** checks render, page overflow, clipped text, brand background and console errors. It does not judge visual quality; I looked at about a dozen of the 252 screenshots.
- **`scripts/ui-matrix.mjs` and `scripts/keyboard-journey.mjs` need `playwright-core` from outside the repo** (`PW_CORE=/dir`), so they are not in the gate.
- **Lighthouse** ran on the signed-out feed only, mobile preset, against a local server that gzips like the new nginx config. The baseline of 70 was measured before gzip and is not like-for-like.
- **Live sources, production data, AWS, real secrets:** untouched and unverified, by the plan's limits.
- **Server validation messages** for unmapped 4xx responses are still English.
- **`work_authorization`** is still free text.

## 6. Review findings left for the owner (`ponytail-review`, cumulative diff)

Fixed in R6-1: the Open & Fill gate refused a confirmed user until their profile had loaded; seven unused translation keys.

1. **`web/src/styles.css` is no longer tokens only.** It now holds five small rule groups (navigation, state marker, boot shell, focus ring, touch targets). AC-WEB-1's wording says "token definitions only". The rules use tokens exclusively; either accept the wording drift or move them to a second CSS file.
2. **Load-error text keeps the language it failed in.** `CvPanel`, `ProfilePanel` and `EnhancementsPanel` store the translated message, so switching language while an error is shown leaves the old language until retry.
3. **`profile.confirm` does not run client validation** before saving and confirming (pre-existing; the server rejects incomplete profiles).
4. **Design direction is mine, not yours.** `DESIGN.md` is derived, calm and conservative. If you have a brief, the tokens and the state-marker motif are the two places to change.

## 7. How to review

```
git log --oneline main..revamp/m8
git diff m8-R3-done m8-R4-done -- web/src        # the UI rebuild
TEST_DATABASE_URL=postgres://… ./scripts/verify.sh
node scripts/ui-matrix.mjs --serve               # prints a local URL to click through
```
