ALTER TABLE admin_identities DROP CONSTRAINT IF EXISTS admin_identities_secret_is_local;
ALTER TABLE admin_identities DROP CONSTRAINT IF EXISTS admin_identities_provider_check;

UPDATE admin_identities
SET provider = 'password',
    issuer = 'urn:lunogram:password'
WHERE provider = 'basic'
AND secret_hash IS NOT NULL
AND deleted_at IS NULL;

ALTER TABLE admin_identities ADD CONSTRAINT admin_identities_provider_check
    CHECK (provider IN ('basic', 'password', 'clerk', 'oidc', 'saml'));
ALTER TABLE admin_identities ADD CONSTRAINT admin_identities_check
    CHECK ((provider = 'password') = (secret_hash IS NOT NULL));
