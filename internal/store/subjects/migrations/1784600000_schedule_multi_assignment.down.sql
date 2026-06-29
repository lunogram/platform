-- Restore the single-assignment-per-name constraint. Note: this will fail if any
-- subject already holds multiple assignments for the same schedule definition;
-- such duplicates must be reconciled before rolling back.

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_schedules_user_schedule_unique
    ON user_schedules (user_id, schedule_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_organization_schedules_org_schedule_unique
    ON organization_schedules (organization_id, schedule_id);
