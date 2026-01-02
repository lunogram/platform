-- Revert: Remove journey_entry_id, current_step_id and restore original constraint

-- Drop the partial unique index if it exists
DROP INDEX IF EXISTS journey_user_state_active_unique;

-- Drop the current_step_id index
DROP INDEX IF EXISTS idx_journey_user_state_current_step_id;

-- Drop the entry_id index
DROP INDEX IF EXISTS idx_journey_user_state_entry_id;

-- Remove the current_step_id column
ALTER TABLE journey_user_state 
DROP COLUMN IF EXISTS current_step_id;

-- Remove the journey_entry_id column
ALTER TABLE journey_user_state 
DROP COLUMN IF EXISTS journey_entry_id;

-- Restore the original unique constraint
-- Note: This may fail if there are duplicate (journey_id, user_id) rows
ALTER TABLE journey_user_state
ADD CONSTRAINT journey_user_state_journey_id_user_id_key UNIQUE (journey_id, user_id);
