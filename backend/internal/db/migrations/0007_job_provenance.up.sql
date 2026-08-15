-- Distinguish generated seed rows from real Tier 1/2 listings.
ALTER TABLE jobs ADD COLUMN synthetic BOOLEAN NOT NULL DEFAULT FALSE;

-- Existing generator rows are the only known synthetic rows; all other rows stay real.
UPDATE jobs
SET synthetic = TRUE
WHERE source_url LIKE 'https://example.test/%';
