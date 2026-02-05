-- Re-add priority column to journey_version_step_children
ALTER TABLE journey_version_step_children ADD COLUMN priority INTEGER NOT NULL DEFAULT 0;
