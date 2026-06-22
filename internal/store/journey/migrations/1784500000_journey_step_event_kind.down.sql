ALTER TABLE journey_version_step_events
    DROP CONSTRAINT journey_version_step_events_pkey;

DELETE FROM journey_version_step_events WHERE kind <> 'enter';

ALTER TABLE journey_version_step_events
    DROP COLUMN kind;

ALTER TABLE journey_version_step_events
    ADD PRIMARY KEY (version_id, external_id, event_id);
