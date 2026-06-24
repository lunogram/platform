-- Migration: scope trusted issuers to their project.
--
-- A trusted_issuer auth method is resolved at authentication time by the JWT
-- `iss`. Resolving by `iss` alone (with no project context) lets a token whose
-- self-asserted issuer collides with another project's registration authenticate
-- against the wrong project. The client API now carries the project in the URL,
-- so resolution becomes (project_id, issuer)-scoped and the issuer is unique per
-- project among active methods.
--
-- The wrinkle: project_id and deleted_at live on the parent auth_methods row,
-- while issuer lives on the child auth_method_trusted_issuers row. We denormalize
-- an immutable project_id onto the child (backfilled from the parent) and add a
-- plain UNIQUE(project_id, issuer). Soft-deleting a method now hard-deletes its
-- trusted-issuer child row (see DeleteAuthMethod), so a deleted issuer is freed
-- and the simple unique constraint does not block re-registration.

ALTER TABLE auth_method_trusted_issuers
    ADD COLUMN project_id UUID REFERENCES projects(id) ON DELETE CASCADE;

-- Backfill the project from the parent auth method.
UPDATE auth_method_trusted_issuers t
SET project_id = m.project_id
FROM auth_methods m
WHERE m.id = t.auth_method_id;

ALTER TABLE auth_method_trusted_issuers
    ALTER COLUMN project_id SET NOT NULL;

-- A given issuer may be registered at most once per project. (Re-registration
-- after delete works because the child row is hard-deleted with its parent.)
ALTER TABLE auth_method_trusted_issuers
    ADD CONSTRAINT auth_method_trusted_issuers_project_issuer_uniq UNIQUE (project_id, issuer);
