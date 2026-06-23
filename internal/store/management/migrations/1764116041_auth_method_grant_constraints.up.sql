-- Migration: per-grant create constraints on auth methods
--
-- Narrows an auth method's create grants to specific named instances (e.g. the
-- event names an "events:create" grant may emit), keyed by resource:
--
--   {"events": ["purchase", "signup"]}
--
-- A resource present with a non-empty list is restricted to those names; an
-- absent resource is unrestricted. NULL (the default) means no constraints.
-- This generalizes the earlier project-level client event allow-list into a
-- per-auth-method capability that travels with the credential.

ALTER TABLE auth_methods ADD COLUMN grant_constraints JSONB;
