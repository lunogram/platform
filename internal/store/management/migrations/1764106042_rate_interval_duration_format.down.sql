-- Restore rate_limit and rate_interval to nullable columns with original defaults.
ALTER TABLE providers
    ALTER COLUMN rate_limit DROP NOT NULL,
    ALTER COLUMN rate_limit DROP DEFAULT,
    ALTER COLUMN rate_interval DROP NOT NULL,
    ALTER COLUMN rate_interval SET DEFAULT 'second';

-- Convert duration strings back to the old-style 'second' value.
UPDATE providers SET rate_interval = 'second' WHERE rate_interval = '1s';
