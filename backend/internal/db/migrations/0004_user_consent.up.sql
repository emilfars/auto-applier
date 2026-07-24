-- Signup consent (UU PDP No. 27/2022, AC-NFR-PRIV): timestamp of when the user
-- consented to data processing at registration. NULL for accounts created
-- before consent capture (e.g. OAuth sign-in).
ALTER TABLE users ADD COLUMN consent_at TIMESTAMPTZ;
