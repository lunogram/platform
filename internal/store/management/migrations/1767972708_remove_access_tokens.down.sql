-- Recreate access_tokens table
CREATE TABLE "access_tokens" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "admin_id" uuid NOT NULL,
    "token" text NOT NULL,
    "revoked" bool DEFAULT false,
    "expires_at" timestamptz,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "ip" varchar(255),
    "user_agent" varchar(255),
    PRIMARY KEY ("id")
);

ALTER TABLE "access_tokens" ADD FOREIGN KEY ("admin_id") REFERENCES "admins"("id") ON DELETE CASCADE;

-- Recreate indexes
CREATE INDEX access_tokens_token_idx ON public.access_tokens USING btree (token);
CREATE INDEX access_tokens_admin_id_idx ON public.access_tokens USING btree (admin_id);

-- Recreate trigger
CREATE TRIGGER set_updated_at_access_tokens BEFORE UPDATE ON access_tokens FOR EACH ROW EXECUTE PROCEDURE set_updated_at();
