-- Single-use tokens for the two things a password account has to do out of
-- band: prove it owns the address it registered with, and recover from a
-- forgotten or compromised password.
--
-- Only the SHA-256 of the token is stored. The token itself exists in exactly
-- two places -- the recipient's inbox and the browser that follows the link --
-- so a database dump, a backup or a replica lag window never hands anybody a
-- working reset. SHA-256 rather than a password hash is deliberate: the input
-- is 32 bytes of CSPRNG output, which has nothing to stretch.
CREATE TABLE admin_action_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    admin_id UUID NOT NULL REFERENCES admins(id) ON DELETE CASCADE,
    -- A token is bound to what it may do. Without this a verification link --
    -- long-lived and handed out at registration -- would be redeemable as a
    -- password reset.
    purpose VARCHAR(32) NOT NULL
        CHECK (purpose IN ('email_verification', 'password_reset')),
    token_hash BYTEA NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- The hash is the lookup key, so it is unique across purposes: a collision
-- would let one token be redeemed as another.
CREATE UNIQUE INDEX admin_action_tokens_hash_unique ON admin_action_tokens (token_hash);

-- Supports both invalidating an admin's outstanding tokens of one purpose,
-- which a password change has to do, and clearing out the ones that can no
-- longer be redeemed as the next token of that purpose is issued. The second
-- reads spent rows, so the index cannot be partial on consumed_at IS NULL.
CREATE INDEX admin_action_tokens_admin_purpose_idx
    ON admin_action_tokens (admin_id, purpose);

CREATE TRIGGER set_updated_at_admin_action_tokens BEFORE UPDATE ON admin_action_tokens
    FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
