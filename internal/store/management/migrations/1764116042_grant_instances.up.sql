-- Migration: fold create-name constraints into the grant as instances
--
-- The create-name allow-list previously lived in a separate top-level column
-- auth_methods.grant_constraints (map<resource, string[]>), parallel to a
-- method's grants. Move the allow-list onto the grant itself: each grant row
-- gains an optional instances list (a JSON array of strings) scoping that grant
-- to those named instances. NULL means unrestricted. This makes the
-- constraint's dependency on a grant structural — a constraint cannot exist
-- without its grant — and removes the orphan-constraint class of bug.
--
-- Scope is create-only for now (only a create grant's instances are enforced),
-- but the per-grant shape generalizes.

ALTER TABLE auth_method_grants ADD COLUMN instances JSONB;
ALTER TABLE auth_methods DROP COLUMN grant_constraints;
