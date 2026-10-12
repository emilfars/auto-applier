# Delivery Gate reports (M8 R4)

antislop active: during (session override). One section per R4 loop. Direction: `DESIGN.md` (agent-authored, dials ENERGY 1 / RHYTHM 2 / MOTION 1). Evidence comes from `scripts/ui-matrix.mjs` (real Chrome, production build, in-memory API), `scripts/browser-e2e.mjs`, `web/src/tokens.test.ts` and the component tests. Rules about content this app does not have (R-17, R-18, R-22, R-28, R-36 statistics, testimonials, illustrations, FAQ, trust claims) are PASS for every screen because none exists; they are listed once here and not repeated.

## App shell and navigation

Design Read: the frame of a four-view workspace for Indonesian job seekers, calm navy, dial 1/2/1.

| Item | Status | Evidence |
|---|---|---|
| R-02 em dash | PASS | `app.tagline` rewritten in both locales; no em dash in shell strings |
| R-03 mobile | PASS | 360px: header fits title, language control and theme toggle; bottom bar replaces the header nav; AppShell pads the content by the bar height; matrix cells for `auth`/`account` report 0 page overflow, 0 clipped |
| R-23 assets | PASS | product name set as text; no logo, icon or image created |
| R-24, R-26 navigation | PASS | `App.test.tsx` "offers only real destinations to a visitor": signed-out links are exactly feed and account; signed-in adds profile and applications, each a rendered `section` |
| R-25 contrast | PASS | nav text is `--brand-muted` (8.87:1 on `--brand-bg`) or `--brand-text`; current marker is `--brand-link` (7.08:1); `tokens.test.ts` |
| R-32 keyboard | PASS | nav items are `<a href>`; language control is a radio group; theme toggle is a button with an accessible name; focus ring `--brand-link` |
| R-34 themes | PASS | matrix captures both themes for every cell, 0 off-brand backgrounds |
| R-35 run and click | PASS | viewport screenshots at 360 and 1280; hash navigation exercised by `ui-matrix.mjs` for every screen and by `App.test.tsx` "switches view on hash change" |
| R-37 direction | PASS | `DESIGN.md`, labeled agent-authored |
| R-04 icons | PASS | briefcase = jobs, CV file = CV and profile, checklist = applications, person = account; each names its destination |
| R-06 type | PASS | system stack, reason in `DESIGN.md` |
| R-10, R-12, R-13 | PASS | no blur, glow or shadow in the shell |
| Liveliness | PASS | focal point is the view content; the shell carries one accent (the 2px current-view marker) |
| C-2 dead controls | PASS | the burger and its empty navbar are gone; every control has a destination or toggles state |
| R-05 structure | PASS | feed first, four views shaped by their job (`DESIGN.md` information architecture) |
| Tap targets | PASS | nav items min-height 44px (64px in the bottom bar), theme toggle 44px, language segments 44x44 |

## AuthPanel (sign in, sign up, verify, reset)

Design Read: a short account form for a returning or new job seeker, calm navy, dial 1/2/1. Focal point: the one filled button.

| Item | Status | Evidence |
|---|---|---|
| R-02 em dash | PASS | no em dash in any `auth.*`, `validation.*` or `error.*` string (`grep` on both locale files for these keys) |
| R-03 mobile | PASS | matrix `auth` cells at 360/768/1280: 0 page overflow, 0 clipped (after excluding screen-reader-only text); buttons and inputs are 44px tall on touch (`styles.css` touch-target rule) |
| R-25 contrast | PASS | filled button white on `--brand-primary-strong` 5.83:1; error and notice text is `--brand-text` on a 15% state tint; `tokens.test.ts` |
| R-26 controls | PASS | "Send reset token" used to do nothing with an empty email; it now validates and shows the email error (`AuthPanel.flows.test.tsx` "asks for a valid email before requesting a reset") |
| R-27 states | PASS | loading (skeleton with hidden "Loading" text), error (role=alert, icon, sentence with a next step), populated; empty is not applicable to a form. Matrix: 36 captured, 12 N/A |
| R-32 keyboard | PASS | native form submit on Enter; mode switch buttons expose `aria-pressed`; reset toggle exposes `aria-expanded` |
| R-34 themes | PASS | light and dark captured for every cell, 0 off-brand |
| R-35 run and click | PASS | `browser-e2e.mjs` in real Chrome: Sign up tab -> fills email and password -> consent -> Create account -> token prefilled -> Verify -> Sign in -> signed-in state, 0 console errors. Matrix error cell: wrong password -> alert. Unit: double submit blocked, unverified sign-in opens the verify step, network failure message |
| R-37 direction | PASS | `DESIGN.md` |
| R-04 icons | PASS | alert circle on errors, check circle on confirmations: each marks the state it sits on, so status is not color-only |
| R-15, R-16 copy | PASS | actions say what they do: "Masuk", "Buat akun", "Verifikasi", "Kirim token reset" |
| C-2, C-4 | PASS | every control acts; input survives an error (`AuthPanel.flows.test.tsx` checks the email is kept) |
| Liveliness | PASS | one filled button per state of the form; the mode switch and reset link are quiet |
| Copy in both locales | PASS | validation and status-specific errors come from `validation.*`, `auth.error.*`, `error.*` in `id.json` and `en.json` (key parity test) |

Known limit: an unmapped 4xx still shows the server's English message (it names the specific field); listed in `REVAMP_REPORT.md`.

## JobFeed and JobCard

Design Read: a scannable job list with a filter rail for seekers comparing pay and requirements, calm navy, dial 1/2/1. Focal point: the list; inside a card, the pay line.

| Item | Status | Evidence |
|---|---|---|
| R-02 em dash | PASS | `grep -c "—"` on both locale files returns 0 (seven strings per locale rewritten) |
| R-03 mobile | PASS | matrix `feed`: 48/48 cells captured at 360/768/1280, 0 page overflow, 0 clipped. At 360px search and location stay visible, the other eight filters sit behind a labeled "Filter lain" toggle with an active count, and the first card starts inside the first screen (viewport screenshot). Card buttons are 36px on desktop and 44px on touch |
| R-17, R-38 data | PASS | cards show only API fields; pay is the employer's label or an estimate that says so; an undisclosed salary says "Gaji tidak dicantumkan" |
| R-25 contrast | PASS | pay amount is `--brand-text`; markers pair a state color with words; `tokens.test.ts` |
| R-26 controls | PASS | the "Cari" button that changed nothing is removed (filters apply as you type, Enter still submits); every remaining button acts: Open & Fill, View & apply (real href, new tab), Mark applied, Hide, More filters, Clear filters, Try again, pagination |
| R-27 states | PASS | loading (text plus skeleton cards, `aria-busy`), empty (says no job matches the filters and offers Clear filters), error (what failed, what to check, Try again), populated; matrix captures all four in both locales and themes; `JobFeed.states.test.tsx` covers retry |
| R-32 keyboard | PASS | filter form is a labeled `form`; toggle exposes `aria-expanded`/`aria-controls`; notices are `role="status"` |
| R-34 themes | PASS | light and dark captured, 0 off-brand |
| R-35 run and click | PASS | `browser-e2e.mjs`: search narrows to zero results and back, Open & Fill arms the extension, 0 console errors. Unit: every filter reaches the query in both locales, clear resets, pagination, dismiss, mark applied, all five Open & Fill notices |
| R-01, R-12, R-13 | PASS | no gradient, glow or shadow; cards are a flat surface with a hairline border |
| R-04 icons | PASS | search, map pin, sliders (filters), external link, alert: each names its control. The magic wand is gone |
| R-06 type | PASS | sentence case; the uppercase tracked "Auto Applier" eyebrow is gone |
| R-09 badges | PASS | requirement tags are plain small chips (real labels from the listing); source, employment type and seniority moved into one muted text line |
| R-14 cards | PASS | cards are identical on purpose: rows of one entity the user compares (PRD FEED-1) |
| R-15, R-16 copy | PASS | "Buka & Isi", "Lihat & lamar", "Tandai dilamar", "Sembunyikan"; the Prime Directive sentence appears once above the list instead of on all 24 cards |
| R-29 palette | PASS | navy surfaces, one blue accent on the primary action, state green/amber only on markers |
| Liveliness | PASS | identity motif in use: square state marker plus words for stated pay (green) versus estimate (amber), and for the Open & Fill notices |
| C-3 | PASS | filter rail and list only; no section exists to fill space |

Kept and deferred: the source filter still lists the synthetic seed's source IDs (PROGRESS D10).

## CvPanel

Design Read: an upload row and a short list of stored CVs, the first step of the profile view, dial 1/2/1. Focal point: the upload control until a CV exists, then the "Uraikan ke profil" action.

| Item | Status | Evidence |
|---|---|---|
| R-02 em dash | PASS | `cv.reviewNote` rewritten; locale files contain no em dash |
| R-03 mobile | PASS | matrix `cv`: 48/48 captured at 360/768/1280, 0 overflow, 0 clipped; long filenames truncate with an ellipsis by design; buttons 44px on touch |
| R-23, R-38 | PASS | the file placeholder states the real limits (PDF or DOCX, 5 MB, from `PRD.md` CV-1); no sample files or names |
| R-26 controls | PASS | Upload is disabled until a file is chosen; Parse, Use as primary and Try again all act (`CvPanel.test.tsx`) |
| R-27 states | PASS | loading (text plus skeleton), empty (says no CV yet and what to do), error (what failed, why, Try again), populated; upload errors name size (413) and type (415); parsing unavailable (503) keeps its own sentence. Matrix captures all four states |
| R-32, R-34 | PASS | native file input behind Mantine `FileInput`; both themes captured, 0 off-brand |
| R-35 run and click | PASS | `browser-e2e.mjs` uploads a real PDF through this panel and sees it listed; unit tests click parse, primary, retry, upload |
| R-04 icons | PASS | upload arrow on Upload, alert and check circles on messages; wand and star removed |
| R-09 badges | PASS | "Utama" is a state marker for a real state (the primary CV), not a pill |
| Liveliness | PASS | state marker motif reused; rows are the only surface |
| C-2, C-4 | PASS | no dead control; works in both themes, three widths, four states |

## ProfilePanel

Design Read: a long sectioned form whose job is "review what will be filled into applications, then confirm", dial 1/2/1. Focal point: the confirm step, the only filled button on the screen.

| Item | Status | Evidence |
|---|---|---|
| R-02 em dash | PASS | `profile.confirmNote` and `profile.confirmed` rewritten; locale files contain no em dash |
| R-03 mobile | PASS | matrix `profile`: 48/48 captured at 360/768/1280, 0 overflow, 0 clipped; fields stack to one column below the `md` breakpoint and row fields below `sm`; remove buttons are 44px |
| R-23, R-38 | PASS | no sample data in fields; placeholders removed; row labels say what goes in each field, date fields state the format |
| R-26 controls | PASS | Save, Confirm, Add education, Add experience, Remove row, Try again all act (`ProfilePanel.test.tsx`) |
| R-27 states | PASS | loading (text plus skeleton), load error (what failed, why, Try again), empty rows ("Belum ada pendidikan. Tambahkan setidaknya satu sebelum mengonfirmasi."), populated; a fresh account is the matrix "empty" cell |
| R-32 keyboard | PASS | every field has a visible label; rows are `role="group"` with a name; remove buttons have an accessible name with the row number |
| R-34 themes | PASS | both themes captured, 0 off-brand |
| R-35 run and click | PASS | `browser-e2e.mjs` fills all 21 fields including an education row and a work row through the UI, saves, confirms, and the confirmed data reaches the ATS fixture; unit tests cover add, remove, validation, load retry and a rejected save |
| R-36 copy, audit #9, #10 | PASS | raw JSON editors replaced by labeled rows; "Required", "Invalid email" and the employment types are localized |
| R-12 shadow, R-14 | PASS | no shadows; only repeated rows get a surface |
| Liveliness | PASS | section headings give the form a rhythm of five named groups; the state marker shows unconfirmed (amber) or confirmed (green) next to the one filled button |
| C-4 | PASS | input survives a validation error (`ProfilePanel.test.tsx` "keeps what was typed") |

Known limit: `work_authorization` is still free text and the server's 4xx validation messages are English; both need a product decision or a contract change and are listed in `REVAMP_REPORT.md`.
