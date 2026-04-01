-- Add time dependency tracking to rules for periodic list reconciliation.
-- When a rule contains rolling time periods (e.g. "in last 30 days"), the list
-- needs periodic recomputation because users can fall out of the time window
-- without any triggering event.
ALTER TABLE rules ADD COLUMN depends_on_time BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE rules ADD COLUMN recompute_interval INTERVAL;

-- Track when a list was last recomputed so the scheduler knows when the next
-- recomputation is due.
ALTER TABLE lists ADD COLUMN last_recomputed_at TIMESTAMPTZ;

-- Partial index on lists to quickly find reconciliation candidates.
-- Narrows the scan to only dynamic, non-deleted lists and includes
-- last_recomputed_at for the time check + version_id for the join.
CREATE INDEX idx_lists_reconciliation_candidates
    ON lists (last_recomputed_at, version_id)
    WHERE type = 'dynamic' AND deleted_at IS NULL;

-- Partial index on rules to quickly filter time-dependent rules.
-- The join comes from list_versions.rule_id = rules.id (PK), so this
-- helps with the WHERE filter after the join.
CREATE INDEX idx_rules_time_dependent
    ON rules (id)
    WHERE depends_on_time = TRUE AND recompute_interval IS NOT NULL;
