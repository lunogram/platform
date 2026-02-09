-- Remove triggers
DROP TRIGGER IF EXISTS update_journey_user_steps_updated_at ON journey_user_steps;
DROP TRIGGER IF EXISTS update_journey_user_state_updated_at ON journey_user_state;
DROP TRIGGER IF EXISTS update_journeys_updated_at ON journeys;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();
