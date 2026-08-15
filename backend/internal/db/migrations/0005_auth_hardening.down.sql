DROP INDEX IF EXISTS users_email_normalized_unique;
ALTER TABLE email_verifications DROP COLUMN IF EXISTS expires_at;
