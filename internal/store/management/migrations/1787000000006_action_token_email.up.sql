-- A token was bound to an admin but not to the mailbox it was sent to, and
-- admins.email can be changed by an organization owner without proving the new
-- address.
--
-- That reopened the invite escalation this table was part of closing. Register
-- an address you control, keep the verification link, change the account's
-- address to an invited one, then follow the link: redemption marked the
-- identity verified and admitted the invite against the NEW address, so holding
-- a mailbox was no longer what earned membership in the organization that
-- invited it.
--
-- Binding the address into the token makes the link a statement about one
-- mailbox rather than about an account, which is what it always claimed to be.
ALTER TABLE admin_action_tokens ADD COLUMN email VARCHAR(255);

UPDATE admin_action_tokens t
SET email = lower(a.email)
FROM admins a
WHERE a.id = t.admin_id;

-- Any token whose admin has since been hard-deleted cannot name a mailbox and
-- can never be redeemed anyway.
DELETE FROM admin_action_tokens WHERE email IS NULL;

ALTER TABLE admin_action_tokens ALTER COLUMN email SET NOT NULL;
