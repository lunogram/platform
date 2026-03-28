-- Revert foreign keys on user_schedules and organization_schedules back to
-- their original definitions (no ON DELETE CASCADE).

-- user_schedules.user_id → users(id)
ALTER TABLE user_schedules
    DROP CONSTRAINT user_schedules_user_id_fkey,
    ADD CONSTRAINT user_schedules_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users(id);

-- organization_schedules.organization_id → organizations(id)
ALTER TABLE organization_schedules
    DROP CONSTRAINT organization_schedules_organization_id_fkey,
    ADD CONSTRAINT organization_schedules_organization_id_fkey
        FOREIGN KEY (organization_id) REFERENCES organizations(id);
