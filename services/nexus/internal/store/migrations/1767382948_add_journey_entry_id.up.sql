-- Add journey_entry_id to track unique journey entries per user
-- This allows users to enter the same journey multiple times

-- Drop the old unique constraint that prevented multiple entries
ALTER TABLE journey_user_state DROP CONSTRAINT IF EXISTS journey_user_state_journey_id_user_id_key;

-- Drop old columns first
ALTER TABLE journey_user_state DROP COLUMN IF EXISTS external_id;
ALTER TABLE journey_user_state DROP COLUMN IF EXISTS type;
ALTER TABLE journey_user_state DROP COLUMN IF EXISTS status;

-- Add new columns
ALTER TABLE journey_user_state ADD COLUMN IF NOT EXISTS journey_entry_id UUID NOT NULL DEFAULT uuid_generate_v4();
ALTER TABLE journey_user_state ADD COLUMN IF NOT EXISTS journey_id UUID NOT NULL REFERENCES journeys(id) ON DELETE CASCADE;
ALTER TABLE journey_user_state ADD COLUMN IF NOT EXISTS external_step_id TEXT NOT NULL DEFAULT '';
ALTER TABLE journey_user_state ALTER COLUMN data SET NOT NULL;
ALTER TABLE journey_user_state ALTER COLUMN data SET DEFAULT '{}'::jsonb;


-- Create indexes
CREATE INDEX IF NOT EXISTS idx_journey_user_state_entry_id ON journey_user_state(journey_entry_id);
CREATE INDEX IF NOT EXISTS idx_journey_user_state_external_step_id ON journey_user_state(external_step_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_journey_user_journey_entry_id_external_step_id ON journey_user_state(journey_entry_id, external_step_id);
CREATE INDEX IF NOT EXISTS idx_journey_user_state_journey_id ON journey_user_state(journey_id);