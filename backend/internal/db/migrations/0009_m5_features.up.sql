CREATE TABLE saved_filters (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    query           JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_alerted_at TIMESTAMPTZ,
    UNIQUE (user_id, name)
);

CREATE TABLE job_user_states (
    user_id         UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    job_key         TEXT NOT NULL,
    dismissed       BOOLEAN NOT NULL DEFAULT FALSE,
    already_applied BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, job_key)
);

CREATE TABLE answer_snippets (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    body       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, name)
);

CREATE INDEX idx_saved_filters_user ON saved_filters (user_id);
CREATE INDEX idx_snippets_user ON answer_snippets (user_id);
