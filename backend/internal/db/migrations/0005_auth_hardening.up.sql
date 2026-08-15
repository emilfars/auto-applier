CREATE UNIQUE INDEX users_email_normalized_unique ON users (lower(trim(email)));

ALTER TABLE email_verifications
    ADD COLUMN expires_at TIMESTAMPTZ NOT NULL DEFAULT (now() + interval '24 hours');

UPDATE email_verifications SET token = encode(digest(token, 'sha256'), 'hex');
UPDATE sessions SET token = encode(digest(token, 'sha256'), 'hex');
UPDATE password_resets SET token = encode(digest(token, 'sha256'), 'hex');
