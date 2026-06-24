-- Reverse 1764116042_trusted_issuer_project_scope: drop the per-project unique
-- constraint and the denormalized project_id column from the child row.

ALTER TABLE auth_method_trusted_issuers
    DROP CONSTRAINT IF EXISTS auth_method_trusted_issuers_project_issuer_uniq;

ALTER TABLE auth_method_trusted_issuers
    DROP COLUMN IF EXISTS project_id;
