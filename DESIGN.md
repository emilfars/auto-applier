# DESIGN.md: Auto Applier

> **Owner-approved direction (2026-10-14).** The M8 run left an agent-derived
> "calm, plain" direction (ENERGY 1 / MOTION 1). The owner rejected it and asked
> for a livelier, modern UI; this file now records the UI the owner reviewed and
> approved. Palette, trust rules and accessibility floors are unchanged.

Every statement below cites its source in brackets.

## Design Read

Reading this as: a signed-in job-application workspace (job feed, CV and
profile, application tracker) for early-to-mid-career Indonesian job seekers on
phones and laptops, in a lively, modern navy-and-royal-blue visual language
with gradients, illustrations and purposeful motion, dial
ENERGY 3 / RHYTHM 3 / MOTION 3.

## Identity

- Auto Applier fills job application forms from one confirmed profile; the
  person always clicks Apply. [PRD §1]
- The visual identity continues the previous "Ofrim" site: deep navy surfaces
  with one royal-blue accent. [palette.md, intro]
- The name is set as text next to a brand tile: a rounded square in the brand
  gradient holding a paper-plane icon (`IconSend`). No separate logo file
  exists. [owner, 2026-10-14]

## Audience

- White-collar seekers in Indonesia with 0 to 8 years of experience, applying
  to 50 to 200 roles per cycle across several portals. [PRD §2]
- Dina, 22, fresh graduate: wants volume, pay transparency, little effort.
  Rizky, 29: wants precise filters and requirement-versus-CV gaps. Maya, 33:
  wants saved filters and alerts above a pay threshold, in minimal time.
  [PRD §4]
- Mobile-first market, Jabodetabek first, Bahasa Indonesia by default with
  English available, IDR by default. [PRD §5.6, §6; plan.md "Geo scope"]
- Feed must feel fast on 4G. [PRD §6]

## Personality

- **Trustworthy before clever.** The product handles a CV and personal data
  and must never present a guess as a fact: estimated pay is visibly different
  from stated pay, uncertain fields are never shown as filled. [PRD §5.4
  labeling rule, §5.5 APP-4]
- **The person is in control.** The system never submits. The interface says so
  where it matters (on Open & Fill and in the review panel), once, in plain
  words. [PRD §1 core decision]
- **Plain-spoken and brief.** Seekers repeat this flow dozens of times; copy
  states what happened and what to do next. [PRD §2; plan-revamp.md §7]

## Color

Tokens come from `design/brand/palette.md` and live only in
`web/src/styles.css`. Components never hold a color literal. [palette.md
"Rules"; ACCEPTANCE AC-WEB-2]

| Role | Token | Use | Source |
|---|---|---|---|
| Page | `--brand-bg` | page background | palette.md Core |
| Surface | `--brand-surface`, `--brand-surface-2` | cards; inputs and nested panels | palette.md Core |
| Text | `--brand-text`, `--brand-muted` | body; secondary | palette.md Core |
| Accent | `--brand-primary`, `--brand-primary-strong` | interactive elements only | palette.md Gradient: "primary blue reserved for interactive elements only" |
| Emphasis text | `--brand-accent-light` | links and small emphasis on navy | palette.md Core |
| State | `--brand-success`, `--brand-warning`, `--brand-danger` | filled / uncertain / error, stated pay, confirmations | palette.md Semantic |

Decisions, one line each:

- **One accent.** Royal blue marks things you can press, and nothing else. Why:
  palette.md reserves it for interactive elements.
- **Filled buttons use `--brand-primary-strong`.** Why: white on
  `--brand-primary` is 3.94:1 and fails WCAG AA at button text sizes; on
  `--brand-primary-strong` it is 5.83:1 (measured with `contrast-check.py`).
  palette.md lists that token for pressed and hero states, so it stays inside
  the brand.
- **State colors carry meaning and always come with a word.** Green = employer
  stated or confirmed, amber = estimate or uncertain, red = error. Why: PRD
  §5.4 and APP-4 make these distinctions trust-critical; color alone excludes
  color-blind users.
- **Light theme derives darker state and border tones from the same tokens**
  (with `color-mix`), because the palette's state colors fail as text on white
  (1.67:1 to 2.77:1). palette.md asks for light pairs to be verified during
  implementation.
- **Gradients and glows are decoration, never meaning.** `--brand-hero` (the
  palette.md navy gradient) backs page heroes and the sign-in promo panel;
  `--brand-gradient` (primary to accent-light) fills the brand tile and icon
  tiles; `--brand-glow-*` are soft radial glows behind heroes, stat tiles and
  the sidebar. Text never sits on a glow without meeting 4.5:1.
- **Company tiles use six fixed gradients** (`--avatar-0` … `--avatar-5`),
  chosen by a hash of the company name so a company keeps its color. Each pair
  is dark enough for white initials; the tiles are decorative (`aria-hidden`),
  the company name is always printed as text.

## Theme

Dark is the default and a light toggle works and persists. Why: palette.md
"Dark is the default theme (matches brand site)"; locked in M4.5 (plan.md
Milestone 4.5; plan-revamp.md §9 "Theme").

## Typography

- **Inter Variable, self-hosted** through `@fontsource-variable/inter`, with
  the system UI stack as fallback. Why: a modern, highly legible UI face that
  ships from our own origin (no third-party font request), and the browser
  only downloads the Latin subset (about 48 KB, `font-display: swap`), so text
  paints immediately on 4G. [PRD §6; 2026-10-14 UI rebuild]
- Sentence case everywhere. No uppercase labels with tracking. Why: plain-spoken
  personality; uppercase tracking is decoration here.
- Hero titles are large and fluid (`clamp(1.6rem, …, 2.4rem)`, tight
  tracking); below that three sizes do the work: section title, card title,
  body. Pay and status lines use weight, not size, to stand out.
- Numbers (pay, counts) use tabular figures so ranges line up in a scanned
  list. Why: Dina and Maya compare pay across cards [PRD §4].

## Dials

`Dial: ENERGY 3 / RHYTHM 3 / MOTION 3`

- **ENERGY 3.** Lively and encouraging: job search is stressful, and a UI that
  feels alive and rewarding (heroes, illustrations, a pop when the profile is
  confirmed) keeps people coming back. Energy never crowds the job list or the
  forms. [owner, 2026-10-14]
- **RHYTHM 3.** Every view opens with a gradient hero, then its own shape: the
  feed is a list with a sticky filter rail and a search bar overlapping the
  hero; the profile is section cards with a sticky confirmation aside and save
  bar; applications open with stat tiles; sign-in is a split card.
- **MOTION 3.** Purposeful CSS motion only, no animation library:
  - views fade up when shown; lists (job cards, CV rows, applications blocks)
    stagger in, 50 ms apart, capped at 8;
  - job cards lift 2 px on hover; alerts and Open & Fill notices fade in;
  - the completeness ring fills (900 ms); the confirmed check pops;
  - "New" badges (posted in the last 3 days) carry a pulsing dot; hero
    illustrations float gently.
  All of it sits inside `prefers-reduced-motion: no-preference`; with reduced
  motion everything renders in its final state. [plan-revamp.md §7 last line]

## Levers

- **Focal point per screen.** Feed: the job list (pay line first in each card).
  CV and profile: the confirm step. Applications: the tracker rows. Account:
  the sign-in form, or the signed-in identity.
- **Identity motif: the state marker.** A small square marker plus a word, in
  the three state colors, used wherever the product makes a trust statement:
  stated versus estimated pay, profile confirmed or not, CV primary, tracker
  status, and the extension's filled / uncertain / empty review. The same
  vocabulary appears on the web and on the employer's page, which is what makes
  the two feel like one product. Each marker names a real state.
- **Heroes.** Each view opens with `PageHeader`: gradient panel, two glows,
  title, one-line purpose and an illustration on wide screens. The feed hero
  leaves room for the search bar to overlap its bottom edge.
- **Illustrations.** Six inline SVGs in `Illustration.tsx` (`jobs`, `search`,
  `documents`, `tracker`, `secure`, `error`), drawn from theme tokens so they
  follow dark and light. Used in heroes, the sign-in promo and empty states.
  Always `aria-hidden`; the words next to them carry the meaning.
- **Surfaces.** Cards (16px radius) group each block; section cards on the
  profile, stat tiles on applications. Filter chips (Mantine `Pill`) show every
  active feed filter and remove it in one tap.
- **Elevation.** Shadows (`--brand-shadow-*`) mark the floating search bar,
  the sign-in card, hovered job cards, the brand tile and dialogs.
- **Radius.** Mantine `md` (8px) for inputs and buttons, `lg` (16px) for
  cards, 24px for heroes, 14px for company tiles. State markers stay square
  with a 2px radius.
- **Icons.** `@tabler/icons-react` where the glyph names the action or the
  thing (search, upload, external link, sun/moon, trash, stat tiles). No wand,
  sparkle or star.

## Information architecture

One page, four views switched by the URL hash, built from Mantine components
already installed. No router dependency. [plan-revamp.md §9 "Information
architecture"]

| View | Hash | Who | Content | Job of the screen |
|---|---|---|---|---|
| Jobs | `#feed` (default) | everyone | `JobFeed`, `JobCard` | Find a role worth applying to and start Open & Fill |
| CV and profile | `#profile` | signed in | `CvPanel`, `ProfilePanel` | Upload, review, confirm the data that gets filled |
| Applications | `#applications` | signed in | `EnhancementsPanel` (completeness, tracker, saved filters, answer snippets) | Track what was filled and keep reusable answers |
| Account | `#account` | everyone | `AuthPanel`; signed in: session and `AccountPanel` (export, delete) | Sign in or manage data rights |

- Navigation: a fixed left sidebar (brand, views, account, language and
  theme) on wide screens; a compact top bar plus a labeled bottom bar on
  phones, with the content padded so the bar never covers it. Each view opens
  with a page header (title and one-line purpose). Why: four
  destinations fit a bottom bar and stay one thumb tap away
  (antislop-layoutmobile, "Nav That Stays Desktop").
- The feed comes first for everyone. Why: PRD G3 and G4 make discovery the
  entry point, and a signed-out visitor can browse before registering.
- Signed-out visitors see Jobs and Account only; a nav item never points at a
  view that is not rendered (antislop R-24).

## Screen list (R4 order)

1. Tokens and theme
2. App shell and navigation
3. `AuthPanel`
4. `JobFeed` and `JobCard`
5. `CvPanel`
6. `ProfilePanel`
7. `EnhancementsPanel`
8. `AccountPanel`
9. Extension review UI (in-page summary; may not touch `guard*.ts`)

Each screen ships loading, empty, error and populated states in `id-ID` and
`en`, in both themes, at 360, 768 and 1280px. [plan-revamp.md §5 E-STATE]

## Out of scope for this direction

Photos, company logos (the feed has no logo data; that needs a backend
change), testimonials, marketing pages, and any new product feature.
