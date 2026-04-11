DROP INDEX IF EXISTS idx_rules_time_dependent;
DROP INDEX IF EXISTS idx_lists_reconciliation_candidates;

ALTER TABLE lists DROP COLUMN IF EXISTS last_recomputed_at;
ALTER TABLE rules DROP COLUMN IF EXISTS recompute_interval;
ALTER TABLE rules DROP COLUMN IF EXISTS depends_on_time;
