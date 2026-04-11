-- Fix foreign keys on user_schedules and organization_schedules to cascade
-- deletes from users and organizations, matching the existing cascade
-- behaviour on child tables (user_scheduled_events, organization_scheduled_events).

-- user_schedules.user_id → users(id)
ALTER TABLE user_schedules
    DROP CONSTRAINT user_schedules_user_id_fkey,
    ADD CONSTRAINT user_schedules_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- organization_schedules.organization_id → organizations(id)
ALTER TABLE organization_schedules
    DROP CONSTRAINT organization_schedules_organization_id_fkey,
    ADD CONSTRAINT organization_schedules_organization_id_fkey
        FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;
