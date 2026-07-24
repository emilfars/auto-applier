-- 0001_init.up.sql — Auto Applier M0 foundational schema.
--
-- Scope decisions baked in:
--   * Jabodetabek-first job listings.
--   * MVP salary is EMPLOYER-STATED ONLY (salary_stated_*); no estimated columns.
--   * Confirm-before-apply gate: profiles.confirmed must be true before the fill
--     flow may be armed (see AC-CV-5).
--   * CV files are encrypted at rest; only object-store references + metadata live
--     in Postgres, never CV contents/PII blobs.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT,
    verified      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE profiles (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    full_name          TEXT,
    phone              TEXT,
    -- structured, user-reviewed CV data
    education          JSONB NOT NULL DEFAULT '[]'::jsonb,
    work_history       JSONB NOT NULL DEFAULT '[]'::jsonb,
    skills             JSONB NOT NULL DEFAULT '[]'::jsonb,
    -- added-info form (CV-4)
    expected_salary    BIGINT,
    notice_period_days INTEGER,
    work_authorization TEXT,
    open_to_relocation BOOLEAN NOT NULL DEFAULT FALSE,
    preferred_locations JSONB NOT NULL DEFAULT '[]'::jsonb,
    employment_type    TEXT,
    -- confirm-before-apply gate (CV-5 / AC-CV-5)
    confirmed          BOOLEAN NOT NULL DEFAULT FALSE,
    confirmed_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE cv_files (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    object_key    TEXT NOT NULL,       -- reference into encrypted object storage
    filename      TEXT NOT NULL,
    content_type  TEXT NOT NULL,       -- application/pdf | application/vnd...docx
    size_bytes    BIGINT NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 5242880),
    encrypted     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE jobs (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source             TEXT NOT NULL,   -- Tier 1/2 source id (no login-walled sources)
    source_url         TEXT NOT NULL,
    dedup_key          TEXT NOT NULL,   -- cross-source dedup (SCR-3)
    title              TEXT NOT NULL,
    company            TEXT NOT NULL,
    location           TEXT,
    remote             BOOLEAN NOT NULL DEFAULT FALSE,
    -- stated salary only at MVP; currency defaults to IDR
    salary_stated_min  BIGINT,
    salary_stated_max  BIGINT,
    salary_currency    TEXT NOT NULL DEFAULT 'IDR',
    seniority          TEXT,
    employment_type    TEXT,
    requirements       JSONB NOT NULL DEFAULT '[]'::jsonb,
    posted_at          TIMESTAMPTZ,
    stale              BOOLEAN NOT NULL DEFAULT FALSE,  -- staleness within 48h (SCR-4)
    stale_at           TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (dedup_key)
);

CREATE INDEX idx_jobs_posted_at ON jobs (posted_at DESC);
CREATE INDEX idx_jobs_location ON jobs (location);
CREATE INDEX idx_jobs_stale ON jobs (stale);

CREATE TABLE applications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    job_id      UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    -- tracker states; 'submitted' is only ever set by explicit user confirmation.
    -- The system NEVER auto-submits an application to a third-party portal.
    status      TEXT NOT NULL DEFAULT 'form_filled'
                CHECK (status IN ('form_filled', 'submitted', 'viewed', 'rejected', 'interview')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, job_id)
);
