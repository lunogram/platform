-- Down migration for journey database

DROP TABLE IF EXISTS journey_user_state CASCADE;
DROP TABLE IF EXISTS journey_version_step_events CASCADE;
DROP TABLE IF EXISTS journey_version_step_children CASCADE;
DROP TABLE IF EXISTS journey_version_steps CASCADE;
DROP TABLE IF EXISTS journey_versions CASCADE;
DROP TABLE IF EXISTS journeys CASCADE;

DROP TYPE IF EXISTS journey_version_status;

DROP FUNCTION IF EXISTS set_updated_at();
DROP EXTENSION IF EXISTS "uuid-ossp";
