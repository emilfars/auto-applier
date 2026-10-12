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
