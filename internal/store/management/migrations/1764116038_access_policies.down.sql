-- Reverse the access_policies generalization. Non-API-key policies (if any) are
-- dropped because the original project_api_keys table only modelled API keys.

DROP VIEW IF EXISTS project_api_keys;

DELETE FROM access_policies WHERE type <> 'api_key';

ALTER TABLE access_policies DROP COLUMN IF EXISTS session_config;
ALTER TABLE access_policies DROP COLUMN IF EXISTS issuer_config;
ALTER TABLE access_policies DROP COLUMN IF EXISTS grants;
ALTER TABLE access_policies DROP COLUMN IF EXISTS type;

ALTER TABLE access_policies ALTER COLUMN value SET NOT NULL;

ALTER TABLE access_policies RENAME TO project_api_keys;
