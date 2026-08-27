-- An admin may hold several upstream identities (a Clerk user today, an
-- enterprise SSO subject tomorrow), so identity moves out of the single
-- admins.external_id scalar into its own table.
--
-- The identity key is (issuer, subject), NOT (provider, subject): Phase 2 adds
-- per-organization SSO connections whose subject spaces collide across
-- customers, and the issuer is what disambiguates them. That makes Phase 2 a
-- purely additive ADD COLUMN connection_id with no index change. provider stays
-- descriptive.
CREATE TABLE admin_identities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    admin_id UUID NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    -- 'basic' is the statically configured AUTH_BASIC_EMAIL credential, whose
    -- secret lives in configuration rather than here; it is therefore distinct
    -- from 'password', which the CHECK below binds to a stored secret_hash.
    provider VARCHAR(32) NOT NULL
        CHECK (provider IN ('basic', 'password', 'clerk', 'oidc', 'saml')),
    issuer VARCHAR(512) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    email VARCHAR(255),
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    -- Ships now and unused, so adding password auth in Phase 1 needs no
    -- migration. The CHECK binds it to exactly the password provider, so a
    -- federated identity can never carry a local secret and vice versa.
    secret_hash TEXT,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CHECK ((provider = 'password') = (secret_hash IS NOT NULL))
);

CREATE UNIQUE INDEX admin_identities_issuer_subject_unique
    ON admin_identities (issuer, subject) WHERE deleted_at IS NULL;
CREATE INDEX admin_identities_admin_id_idx ON admin_identities (admin_id);
CREATE INDEX admin_identities_email_idx ON admin_identities (lower(email));

CREATE TRIGGER set_updated_at_admin_identities BEFORE UPDATE ON admin_identities
    FOR EACH ROW EXECUTE PROCEDURE set_updated_at();

-- Every existing external_id becomes an identity under a sentinel issuer. The
-- exchange adopts such a row on the owner's next login, rewriting it in place to
-- the real issuer; until then it is what keeps already-provisioned admins
-- reachable. Only the Clerk provider ever wrote external_id, hence 'clerk'.
--
-- DISTINCT ON guards the unique index: nothing ever enforced that two admins
-- could not carry the same external_id. Should a customer database hold such a
-- pair, the deterministic pick (oldest, then lowest id) gets the legacy row and
-- the other admin simply logs in through the normal resolution path.
INSERT INTO admin_identities (admin_id, provider, issuer, subject, email)
SELECT DISTINCT ON (external_id)
    id, 'clerk', 'urn:lunogram:legacy-external-id', external_id, email
FROM admins
WHERE deleted_at IS NULL
AND external_id IS NOT NULL
AND external_id <> ''
ORDER BY external_id, created_at ASC, id ASC;

-- Dropped rather than deprecated: a column that still exists gets dual-written
-- by the next person to touch CreateAdmin, and the two-sources-of-truth
-- divergence this refactor removes comes straight back. Nothing structural
-- depended on it (nullable, no unique index, no foreign key) and the data now
-- lives in admin_identities, so the drop is reversible.
ALTER TABLE admins DROP COLUMN external_id;
