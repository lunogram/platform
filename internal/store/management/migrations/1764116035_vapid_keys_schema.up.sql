-- Migration: Create vapid_keys table
-- Purpose: Store VAPID key pairs for push notifications

CREATE TABLE IF NOT EXISTS vapid_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    name TEXT NOT NULL, -- e.g., "production", "staging", "client-A", etc.
    public_key TEXT NOT NULL,
    private_key TEXT NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ -- NULL = active, set = soft deleted
);

-- Unique names per active key (can't have two active keys with same name)
CREATE UNIQUE INDEX idx_vapid_keys_unique_name_active
    ON vapid_keys(name) 
    WHERE deleted_at IS NULL;