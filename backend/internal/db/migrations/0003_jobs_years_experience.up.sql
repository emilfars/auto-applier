-- Feed YoE filter (FEED-2): minimum years of experience a listing requires,
-- parsed from the listing text. NULL when unspecified.
ALTER TABLE jobs ADD COLUMN years_experience INT;
