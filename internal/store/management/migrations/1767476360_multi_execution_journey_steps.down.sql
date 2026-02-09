DROP INDEX IF EXISTS journey_user_state_latest;
DROP INDEX IF EXISTS journey_user_state_unique;
CREATE UNIQUE INDEX IF NOT EXISTS idx_journey_user_journey_entry_id_external_step_id ON journey_user_state(journey_entry_id, external_step_id);
ALTER TABLE journey_user_state DROP COLUMN IF EXISTS occurrence;
