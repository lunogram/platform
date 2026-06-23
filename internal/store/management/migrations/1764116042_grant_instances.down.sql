ALTER TABLE auth_methods ADD COLUMN grant_constraints JSONB;
ALTER TABLE auth_method_grants DROP COLUMN instances;
