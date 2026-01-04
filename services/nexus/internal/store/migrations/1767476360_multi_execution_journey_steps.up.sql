-- Multi-Execution Journey Steps (FSM Semantics)
-- Enable steps to execute multiple times per journey entry

ALTER TABLE journey_user_state ADD COLUMN occurrence INTEGER NOT NULL DEFAULT 1;
DROP INDEX IF EXISTS idx_journey_user_journey_entry_id_external_step_id;

CREATE UNIQUE INDEX journey_user_state_unique ON journey_user_state (journey_entry_id, external_step_id, occurrence);
CREATE INDEX journey_user_state_latest ON journey_user_state (journey_entry_id, external_step_id, occurrence DESC);
