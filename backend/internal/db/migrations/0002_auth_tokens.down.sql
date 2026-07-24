-- 0002_auth_tokens.down.sql — revert auth token & session persistence.

DROP TABLE IF EXISTS password_resets;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS email_verifications;
