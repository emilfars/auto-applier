# PRD: Auto Applier (Compact)

**Version:** 0.1 (Draft) · **Owner:** Hanif · **Status:** For review
> Condensed from the full PRD. See `plan.md` for the build sequence.

## 1. Overview
Web platform + browser extension that removes the repetitive grind of job hunting. Users upload/maintain a CV + profile, browse a continuously-scraped, filterable feed of jobs, and when they open a job's application page the extension autofills common fields. **The final Apply/Submit click is always the user's.**

**One-liner:** Upload your CV once; the browser fills every application for you, you just click apply.

**Core decision (non-negotiable):** semi-automated, **human-in-the-loop**. The system never submits. Keeps quality high, avoids spam/blacklisting, sidesteps ToS issues around automated submission. No headless/background submission, ever, in any phase.

## 2. Problem
Seekers waste 10–20 min/application re-typing the same data across portals (Jobstreet, Glints, Kalibrr, LinkedIn, company ATS). Active seekers apply to 50–200 roles/cycle. Friction → low application volume, application fatigue, poor targeting (can't compare pay/requirements in one view). **Who:** early-to-mid-career white-collar seekers in Indonesia (0–8 YoE) across multiple portals.

## 3. Goals & Non-Goals
**Goals (MVP)**
- **G1** Register / log in / manage profile — signup completion > 70%.
- **G2** Upload, parse, edit CV in-app — upload success > 95%.
- **G3** Feed refreshed from scraped sources — > 5,000 active listings, freshness < 48h.
- **G4** Filterable discovery (pay, requirements, location) — filter usage/session > 60%.
- **G5** Extension autofill; user clicks apply — fill coverage > 80% of fields on supported ATS, median time-to-apply < 90s.

**Non-Goals:** autonomous submission (permanent); employer-side product; CV writing / AI rewrite (Phase 2); interview scheduling/tracking beyond application status.

## 4. Personas
- **P1 Dina** (22, fresh grad, Depok): high volume, salary transparency, low-effort applications.
- **P2 Rizky** (29, 5 YoE switcher, Jakarta): precise filters, requirement-vs-CV gap visibility.
- **P3 Maya** (33, passive, Tangerang): saved filters, alerts, roles above a pay threshold, minimal time.

## 5. Functional Requirements

### 5.1 Auth & Accounts
| ID | Requirement | Pri |
|---|---|---|
| AUTH-1 | Email + password registration w/ email verification | P0 |
| AUTH-2 | Google OAuth login | P0 |
| AUTH-3 | Password reset flow | P0 |
| AUTH-4 | Session mgmt, secure token handling, logout | P0 |
| AUTH-5 | Account deletion + data export (UU PDP No. 27/2022) | P1 |

### 5.2 CV & Profile
| ID | Requirement | Pri |
|---|---|---|
| CV-1 | Upload CV (PDF, DOCX; max 5MB) | P0 |
| CV-2 | Auto-parse → contact, education, work history, skills | P0 |
| CV-3 | In-app editing of all parsed fields | P0 |
| CV-4 | Added-info form: expected salary, notice period, work authorization, relocation, preferred locations, employment type | P0 |
| CV-5 | Multiple CV versions per user | P1 |
| CV-6 | Completeness score + prompts | P1 |
| CV-7 | Regenerate downloadable CV (PDF) from profile | P2 |

**Parsing:** target ≥ 90% field-level accuracy (Indonesian + English). **Fallback:** user always reviews + confirms parsed data before first apply (this keeps data quality high).

### 5.3 Scraper & Ingestion
| ID | Requirement | Pri |
|---|---|---|
| SCR-1 | Scheduled scraping of target sources | P0 |
| SCR-2 | Normalization: title, company, location, salary (stated/estimated), requirements, seniority, employment type, posting date, source URL | P0 |
| SCR-3 | Cross-source deduplication | P0 |
| SCR-4 | Staleness: mark expired/removed within 48h | P0 |
| SCR-5 | Salary estimation model (title × location × seniority) | P1 |
| SCR-6 | Requirement extraction into structured tags | P1 |

**Source strategy (decided):** **Tier 1 (APIs/partner feeds) + Tier 2 (public boards, no login walls).** Tier 3 login-walled (LinkedIn, some Jobstreet views) **excluded from MVP** — high legal/ToS risk.

### 5.4 Job Feed & Filtering
| ID | Requirement | Pri |
|---|---|---|
| FEED-1 | Infinite-scroll card list: title, company, location, pay (stated vs estimated clearly labeled), posted date, source | P0 |
| FEED-2 | Filters: pay range, location (city + remote), skills, YoE, employment type, posting date, source | P0 |
| FEED-3 | Free-text search on title + company | P0 |
| FEED-4 | Match score per job vs CV (requirements overlap) | P1 |
| FEED-5 | Saved filters + email alerts on new matches | P1 |
| FEED-6 | Hide/dismiss jobs; "already applied" state | P1 |

**Labeling rule (trust-critical):** estimated pay must be visually distinct from employer-stated pay (e.g., "~Rp 8–11 jt (estimated)"). Never present estimates as fact. *MVP launches stated-salary only.*

### 5.5 Apply Flow — Browser Extension (user clicks apply)
| ID | Requirement | Pri |
|---|---|---|
| APP-1 | Chrome extension detects supported forms + autofills common fields (name, contact, education, work history, expected salary, notice period, links) | P0 |
| APP-2 | CV file auto-attach on upload fields | P0 |
| APP-3 | Field-mapping coverage: Jobstreet, Glints, Kalibrr, Greenhouse, Workable, Lever, common career forms | P0 |
| APP-4 | Visual fill-review: filled highlighted; unfilled/uncertain flagged. **Extension never triggers submit** | P0 |
| APP-5 | "Open & Fill" from feed: opens source page with extension armed | P0 |
| APP-6 | Application tracker: logs "form filled", user confirms "submitted"; statuses applied/viewed/rejected/interview | P1 |
| APP-7 | Cover-note/answer snippets w/ variable substitution (name, role, company) | P1 |
| APP-8 | Unknown-form fallback: generic heuristic fill (label matching), low-confidence flagged | P2 |

**Hard rule:** extension autofills only. Submission is always a human click.

### 5.6 Mobile Apply (fast-follow)
Chrome mobile has no extensions → desktop extension doesn't port. Path: native app (**Android first**) with in-app WebView injecting the same fill logic.
| ID | Requirement | Pri |
|---|---|---|
| MOB-1 | Android app: feed + filters, parity with web | P0 (mobile) |
| MOB-2 | In-app WebView loads application page w/ fill scripts; same review highlighting; user taps apply | P0 (mobile) |
| MOB-3 | CV attach from app storage on mobile upload fields | P0 (mobile) |
| MOB-4 | Shared fill-mapping layer: one codebase serves extension + mobile | P0 (mobile) |
| MOB-5 | iOS via Safari Web Extension or iOS app + WebView | P1 |
| MOB-6 | Android Autofill Service integration | P2 |

**Sequencing:** ship desktop extension first to validate mappings, then Android within 1–2 quarters (mobile-first market → not optional for scale, but don't double surface area before the fill engine is proven).

## 6. Non-Functional Requirements
- **Performance:** feed < 2s p95 on 4G; filter response < 500ms.
- **Scale (MVP):** 10k users, 50k listings, 1k concurrent.
- **Security:** CV/PII encrypted at rest; HTTPS only; auth rate limiting.
- **Privacy:** UU PDP compliance — consent capture, data deletion, no selling CV data without explicit opt-in.
- **Scraper resilience:** per-source circuit breakers; a source failure must not take down the feed.
- **Localization:** Bahasa Indonesia + English; IDR default.

## 7. Success Metrics (first 90 days)
5,000 registered users · CV→confirmed-profile > 60% · weekly active applier > 25% of WAU · ≥ 5 applications/active user/week · median listing age at apply < 5 days · "got interview via platform" tracked from day 1 (north-star input).

## 8. Phase 2 (indicative)
AI CV tailoring · AI screening-answer drafts · batch queue (fill→review→click in sequence) · Firefox/Edge + expanded ATS coverage · monetization (freemium: e.g. 10 fills/wk free, premium for unlimited + AI + priority alerts).

## 9. Risks & Mitigations
| Risk | Sev | Mitigation |
|---|---|---|
| Scraping violates ToS (LinkedIn/Jobstreet); legal + IP blocks | High | Tier sources; prefer APIs/partnerships; exclude login-walled at MVP; consult counsel before launch |
| Salary estimates wrong → trust collapse | High | Label clearly + confidence range; MVP is stated-salary only |
| CV parser poor on Indonesian formats | Med | Mandatory user review; parser feedback loop |
| Extension fills wrong data into wrong field | High | Per-field confidence flags; visual review before submit; per-ATS mapping tests; fill-correction telemetry |
| ATS DOM changes break mappings silently | Med | Mapping version per site; fill-failure telemetry; fast update pipeline |
| Scraper breaks silently on layout change | Med | Monitoring + per-source freshness SLA alerts |
| Low listing volume → empty-feed churn | Med | Seed ≥ 5k listings before signup; Jabodetabek-first |

## 10. Decisions (resolved open questions)
- **Source list:** Tier 1 + Tier 2 only; Tier 3 excluded from MVP.
- **Auto-apply scope:** extension autofills; user always clicks apply. No autonomous submission.
- **Salary estimation:** launch stated-salary only; model post-MVP.
- **Geo scope:** Jabodetabek-first.
- **Monetization:** free at MVP.
- **Employer side:** out of scope (seeker-side only).

## 11. MVP Scope Summary
**In:** auth · CV upload + parse + edit · added-info form · scraper (Tier 1–2) · normalized filterable feed with labeled (stated-only) pay · Chrome extension autofill (top ATS/portals) with user-clicked submission · basic tracker.
**Fast-follow (1–2 quarters):** Android app with in-app WebView fill (§5.6).
**Out:** autonomous submission (permanent) · AI rewriting · batch queue · employer features · monetization.
