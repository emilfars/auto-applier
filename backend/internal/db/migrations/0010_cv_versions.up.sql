ALTER TABLE cv_files
    ADD COLUMN label TEXT NOT NULL DEFAULT '',
    ADD COLUMN is_primary BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE cv_files SET label = filename WHERE label = '';

UPDATE cv_files AS f
SET is_primary = TRUE
WHERE f.id = (
    SELECT candidate.id
    FROM cv_files AS candidate
    WHERE candidate.user_id = f.user_id
    ORDER BY candidate.created_at DESC, candidate.id DESC
    LIMIT 1
);

CREATE UNIQUE INDEX cv_files_one_primary_per_user
    ON cv_files (user_id) WHERE is_primary;
