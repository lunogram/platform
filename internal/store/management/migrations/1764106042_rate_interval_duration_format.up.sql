-- Align rate_interval default with Go time.Duration format used throughout the codebase.
-- The original migration used 'second' as the default, but all Go code and provider
-- manifests use Go duration strings (e.g. '1s', '1m', '1h').

-- Backfill any existing NULL values before adding NOT NULL constraints.
UPDATE providers SET rate_limit = 0 WHERE rate_limit IS NULL;
UPDATE providers SET rate_interval = '1s' WHERE rate_interval IS NULL OR rate_interval = 'second';

-- Set defaults and add NOT NULL constraints.
ALTER TABLE providers
    ALTER COLUMN rate_limit SET DEFAULT 0,
    ALTER COLUMN rate_limit SET NOT NULL,
    ALTER COLUMN rate_interval SET DEFAULT '1s',
    ALTER COLUMN rate_interval SET NOT NULL;
