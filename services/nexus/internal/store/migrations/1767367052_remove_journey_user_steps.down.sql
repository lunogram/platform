-- Recreate journey_user_steps table
CREATE TABLE journey_user_steps (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    journey_id UUID NOT NULL REFERENCES journeys(id) ON DELETE CASCADE,
    step_id UUID NOT NULL REFERENCES journey_version_steps(id) ON DELETE RESTRICT,
    entrance_id UUID REFERENCES journey_user_steps(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    external_id TEXT NOT NULL,
    entered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    exited_at TIMESTAMPTZ,
    delay_until TIMESTAMPTZ,
    data JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_journey_user_steps_user_id ON journey_user_steps(user_id);
CREATE INDEX idx_journey_user_steps_journey_id ON journey_user_steps(journey_id);
CREATE INDEX idx_journey_user_steps_step_id ON journey_user_steps(step_id);
CREATE INDEX idx_journey_user_steps_entrance_id ON journey_user_steps(entrance_id);
CREATE INDEX idx_journey_user_steps_external_id ON journey_user_steps(external_id);
CREATE INDEX idx_journey_user_steps_entered_at ON journey_user_steps(entered_at);

CREATE TRIGGER update_journey_user_steps_updated_at
    BEFORE UPDATE ON journey_user_steps
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
