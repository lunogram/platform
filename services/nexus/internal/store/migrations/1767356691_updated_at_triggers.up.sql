-- Create a function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Add triggers to tables with updated_at columns
CREATE TRIGGER update_journeys_updated_at
    BEFORE UPDATE ON journeys
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_journey_user_state_updated_at
    BEFORE UPDATE ON journey_user_state
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_journey_user_steps_updated_at
    BEFORE UPDATE ON journey_user_steps
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
