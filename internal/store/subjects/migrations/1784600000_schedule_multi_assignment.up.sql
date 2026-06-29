-- Allow multiple schedule assignments with the same name per subject.
-- Previously a (user_id, schedule_id) / (organization_id, schedule_id) pair was
-- unique, so a subject could hold at most one assignment per named schedule and a
-- second submit overwrote the first. Assignments are now addressed by their own
-- primary key (id): submitting an existing id upserts that row, while omitting it
-- creates a new instance. The non-unique lookup indexes (idx_user_schedules_user,
-- idx_organization_schedules_org) are retained for query performance.

DROP INDEX IF EXISTS idx_user_schedules_user_schedule_unique;
DROP INDEX IF EXISTS idx_organization_schedules_org_schedule_unique;
