ALTER TABLE admins ADD COLUMN external_id VARCHAR(255);

-- Best-effort restore: the legacy column held one subject per admin, so the
-- oldest live identity is the closest equivalent. Identities adopted since the
-- up migration have had their sentinel issuer rewritten to the real one, which
-- is why this does not filter on the sentinel.
UPDATE admins a
SET external_id = i.subject
FROM (
    SELECT DISTINCT ON (admin_id) admin_id, subject
    FROM admin_identities
    WHERE deleted_at IS NULL
    ORDER BY admin_id, created_at ASC, id ASC
) i
WHERE i.admin_id = a.id;

DROP TRIGGER IF EXISTS set_updated_at_admin_identities ON admin_identities;
DROP TABLE IF EXISTS admin_identities;
