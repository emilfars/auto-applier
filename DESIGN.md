# DESIGN.md: Auto Applier

> **Agent-authored direction (antislop option 2).** The kickoff prompt for M8
> left the design brief as an unfilled placeholder, and nobody was available to
> ask, so this direction is derived only from `PRD.md`, `plan.md` and
> `design/brand/palette.md` (`plan-revamp.md` section 9). Agent-written style
> leans toward default taste, so expect a calm, plain result. The owner should
> replace any line here that does not match the product they want.
>
> antislop R-37 would label an unasked direction "draft without direction" with
> dials 1/1/1. The project rule (`plan-revamp.md` section 9) allows a derived
> direction with dials capped at ENERGY 2 / RHYTHM 2 / MOTION 1; where the two
> disagree the project rule holds (`plan-revamp.md` section 3).

Every statement below cites its source in brackets.

## Design Read

Reading this as: a signed-in job-application workspace (job feed, CV and
profile, application tracker) for early-to-mid-career Indonesian job seekers on
phones and laptops, in a calm, plain-spoken navy visual language, dial
ENERGY 1 / RHYTHM 2 / MOTION 1.

## Identity

- Auto Applier fills job application forms from one confirmed profile; the
  person always clicks Apply. [PRD §1]
- The visual identity continues the previous "Ofrim" site: deep navy surfaces
  with one royal-blue accent. [palette.md, intro]
- The name is set as text. No logo or icon asset exists and none is created in
  this run. [plan-revamp.md §4, antislop R-23]

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
- **No gradient, glow, glass or texture.** palette.md describes a hero
  gradient for the marketing site; this app has no hero, so the gradient has
  nothing to separate.

## Theme

Dark is the default and a light toggle works and persists. Why: palette.md
"Dark is the default theme (matches brand site)"; locked in M4.5 (plan.md
Milestone 4.5; plan-revamp.md §9 "Theme").

## Typography

- **System UI font stack** (`system-ui, -apple-system, "Segoe UI", Roboto,
  sans-serif`), not a loaded web font. Why: the feed must reach p95 under 2s on
  4G [PRD §6], and this run may not add font assets [plan-revamp.md §4]. The
  previous theme named Inter but never loaded it, so this states what users
  already get.
- Sentence case everywhere. No uppercase labels with tracking. Why: plain-spoken
  personality; uppercase tracking is decoration here.
- Three sizes do the work: screen title, card title, body. Pay and status lines
  use weight, not size, to stand out. Why: one focal point per screen.
- Numbers (pay, counts) use tabular figures so ranges line up in a scanned
  list. Why: Dina and Maya compare pay across cards [PRD §4].

## Dials

`Dial: ENERGY 1 / RHYTHM 2 / MOTION 1`

- **ENERGY 1.** A tool used many times a week by people under job-search
  stress; it should get out of the way. [PRD §2]
- **RHYTHM 2.** Each screen has its own shape because its job differs: the feed
  is a scannable list with a filter rail, the profile is a long sectioned form,
  the tracker is rows with a status control, account is two short blocks. Within
  a screen the rhythm is uniform.
- **MOTION 1.** Hover and focus states only. Skeletons do not shimmer when the
  system asks for reduced motion. [plan-revamp.md §7 last line]

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
- **Whitespace as structure.** Sections are separated by space and a hairline,
  not by a card around every block. Only repeated entities (job cards, CV rows,
  tracker rows) get a surface.
- **Elevation.** Flat by default. A shadow appears on things that float above
  the page: the mobile filter drawer, the delete dialog, notifications.
- **Radius.** One radius (Mantine `md`, 8px) for inputs, buttons and cards.
  State markers are square with a 2px radius. Nothing is pill-shaped.
- **Icons.** `@tabler/icons-react`, already installed, only where the glyph
  names the action (search, upload, external link, sun/moon, trash). No wand,
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

- Navigation: a header row of the views on wide screens; a labeled bottom bar
  on phones, with the content padded so the bar never covers it. Why: four
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

Logos, illustrations, photos, marketing sections, statistics, testimonials,
and any new product feature. [plan-revamp.md §1 non-goals, §4 UI-specific]
