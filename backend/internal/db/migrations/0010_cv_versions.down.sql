DROP INDEX IF EXISTS cv_files_one_primary_per_user;

ALTER TABLE cv_files
    DROP COLUMN IF EXISTS is_primary,
    DROP COLUMN IF EXISTS label;
