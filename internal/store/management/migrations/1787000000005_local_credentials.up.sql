-- The 'basic' and 'password' providers become one.
--
-- They were split because the environment credential's secret lived in
-- configuration and a per-admin one lived in secret_hash. That put the same kind
-- of secret behind two verifiers, and only one of them had the login throttle,
-- the constant-time comparison against a dummy hash, and the re-hash on login.
-- The environment credential is now seeded into a row like any other, so there
-- is one provider, one issuer and one code path.
--
-- Existing deployments cross over without doing anything. A 'basic' row is
-- already what a local identity looks like apart from the missing hash, and the
-- seed fills that in on the next boot; a deployment that never had one gets its
-- admin found by address and completed the same way.

-- Rows written by the short-lived 'password' provider, which only ever existed
-- on an unreleased branch. Their subject is already the admin id, so only the
-- issuer and the provider move.
UPDATE admin_identities
SET provider = 'basic',
    issuer = 'urn:lunogram:basic'
WHERE provider = 'password'
AND deleted_at IS NULL;

-- A 'basic' row used to be keyed on the configured address. The subject is now
-- the admin's own id, so that a credential does not follow an address when the
-- address changes.
UPDATE admin_identities
SET subject = admin_id::text
WHERE provider = 'basic'
AND issuer = 'urn:lunogram:basic'
AND subject <> admin_id::text
AND deleted_at IS NULL;

ALTER TABLE admin_identities DROP CONSTRAINT IF EXISTS admin_identities_provider_check;
ALTER TABLE admin_identities ADD CONSTRAINT admin_identities_provider_check
    CHECK (provider IN ('basic', 'clerk', 'oidc', 'saml'));

-- The old pair of CHECKs said a password row must carry a secret and a basic row
-- must not. Neither half survives: 'basic' is now the provider that may carry
-- one, and it may also be waiting for one -- an admin who holds an invite but
-- has not set a password yet, and the seeded account between the migration and
-- the boot that fills it. What still has to hold is the half that matters: a
-- federated identity can never carry a local secret.
ALTER TABLE admin_identities DROP CONSTRAINT IF EXISTS admin_identities_check;
ALTER TABLE admin_identities DROP CONSTRAINT IF EXISTS admin_identities_check1;
ALTER TABLE admin_identities ADD CONSTRAINT admin_identities_secret_is_local
    CHECK (secret_hash IS NULL OR provider = 'basic');
