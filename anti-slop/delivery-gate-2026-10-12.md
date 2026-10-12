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
