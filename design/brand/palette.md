# Brand palette — authoritative design tokens

> Sampled 2026-08-21 from the "Ofrim" previous static site screenshots in this
> folder (`ofrim-home-1.png`, `ofrim-home-2.png` — add the actual files here).
> Deep-navy dark theme with a royal-blue accent. This file is the single source
> of truth for M4.5; coding agents read this, not the images.

## Core

| Token | Value | Source / usage |
|---|---|---|
| `--brand-primary` | `#2E7CF6` | royal blue: primary buttons ("Review CV Sekarang"), active nav underline, links, icon accents |
| `--brand-primary-strong` | `#1E5FD0` | hover/pressed state, gradient end on hero panels |
| `--brand-accent-light` | `#5AA7FF` | light-blue highlights: stat numbers (25.000+, 9.900+), badge icons, small emphasis text |
| `--brand-bg-dark` | `#0B1830` | page background (deep navy) |
| `--brand-surface-dark` | `#13234A` | cards, testimonial panels, pricing boxes |
| `--brand-surface-2-dark` | `#1B2F5E` | raised surfaces: chips, inputs, nested panels, offer box |
| `--brand-border-dark` | `#24406F` | card borders, dividers on dark surfaces |
| `--brand-bg-light` | `#F4F7FC` | light theme page background (derived neutral) |
| `--brand-surface-light` | `#FFFFFF` | light theme cards |
| `--brand-text-on-dark` | `#FFFFFF` | headings/body on navy backgrounds |
| `--brand-text-on-light` | `#13223F` | headings/body on light backgrounds |
| `--brand-muted` | `#A9B8D8` | secondary text, captions, inactive nav |

## Semantic

| Token | Value | Usage |
|---|---|---|
| `--brand-success` | `#34D399` | confirmations, "filled" fill state, safety notice |
| `--brand-warning` | `#FBBF24` | "uncertain" review state |
| `--brand-danger` | `#F87171` | errors, destructive actions |

## Gradient

Hero and section headers use a subtle navy→blue radial/linear gradient:
`linear-gradient(180deg, #1B2F5E 0%, #0B1830 100%)` with the primary blue
reserved for interactive elements only.

## Rules (enforced by AC-WEB-2 / AC-WEB-4)

- No hardcoded hex/rgb outside the single token definition file in `web/src`.
- Body text contrast ≥ 4.5:1 against its background in both themes
  (`#FFFFFF` on `#0B1830` ≈ 15.9:1 ✅; `#A9B8D8` on `#0B1830` ≈ 8.4:1 ✅;
  verify light-theme pairs during implementation).
- Dark is the default theme (matches brand site); light variant uses the
  derived neutrals above.
