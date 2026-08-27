-- Email becomes an identity-linking key: the console exchange links a verified
-- upstream identity onto an existing admin by address. That join must be
-- case-insensitive, so the uniqueness that backs it must be too. The old
-- UNIQUE (email) index let 'A@x.com' and 'a@x.com' coexist, and a
-- case-insensitive lookup across them would return whichever row Postgres
-- happened to hand back first.
--
-- Because the old index was case-SENSITIVE and covered every live row, every
-- possible collision here is case-only. That bounds the reconciliation below:
-- no group can fail to rank, and the index build cannot fail after the UPDATE.
ALTER TABLE admins ADD COLUMN email_conflict_at TIMESTAMPTZ;

-- Two admins sharing an address may be two real people, so this migration
-- neither deletes nor rewrites: it elects one canonical row per address and
-- quarantines the rest by stamping email_conflict_at. A quarantined admin keeps
-- its real email and keeps logging in through admin_identities; only the EMAIL
-- JOIN excludes it, and linking a new identity onto that address fails closed
-- rather than guessing which account was meant.
--
-- The ranking is deterministic so a repeated run (a restore, a rebuilt replica,
-- a re-applied migration) elects the same keeper every time: most live
-- organization memberships, then most live project memberships, then oldest,
-- then lowest id. The last two tiebreakers do not depend on physical row order
-- -- exactly the property the old index failed to provide.
WITH ranked AS (
    SELECT
        a.id,
        row_number() OVER (
            PARTITION BY lower(a.email)
            ORDER BY
                (SELECT count(*) FROM organization_members om
                  WHERE om.admin_id = a.id AND om.deleted_at IS NULL) DESC,
                (SELECT count(*) FROM project_admins pa
                  WHERE pa.admin_id = a.id AND pa.deleted_at IS NULL) DESC,
                a.created_at ASC,
                a.id ASC
        ) AS position
    FROM admins a
    WHERE a.deleted_at IS NULL
)
UPDATE admins
SET email_conflict_at = NOW()
WHERE id IN (SELECT id FROM ranked WHERE position > 1);

DROP INDEX admins_email_unique;

-- CREATE INDEX CONCURRENTLY is not usable: golang-migrate runs each file inside
-- an implicit transaction. A plain build takes a SHARE lock, and admins holds
-- one row per staff member, so the blocking window is negligible.
CREATE UNIQUE INDEX admins_email_lower_unique
    ON admins (lower(email))
    WHERE deleted_at IS NULL AND email_conflict_at IS NULL;
