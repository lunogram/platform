-- Distinguish the role an event dependency plays for an entrance step.
-- 'enter' (the default) means the event triggers enrollment into the journey.
-- 'exit'  means the event completes the user's active runs from that entrance
--         (used by list triggers to exit a user when they leave the list).
ALTER TABLE journey_version_step_events
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'enter'
    CONSTRAINT journey_version_step_events_kind_check CHECK (kind IN ('enter', 'exit'));

-- An entrance may now register the same event for both an enter and an exit
-- role across different entrances, so widen the primary key to include kind.
ALTER TABLE journey_version_step_events
    DROP CONSTRAINT journey_version_step_events_pkey;

ALTER TABLE journey_version_step_events
    ADD PRIMARY KEY (version_id, external_id, event_id, kind);
