UPDATE journey_version_steps SET data = '{}'::jsonb WHERE data IS NULL;
ALTER TABLE journey_version_steps ALTER COLUMN data SET DEFAULT '{}'::jsonb, ALTER COLUMN data SET NOT NULL;