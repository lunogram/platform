-- Recreate job_locks table
CREATE TABLE "job_locks" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "key" varchar(255),
    "owner" varchar(255),
    "expiration" timestamptz,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

CREATE UNIQUE INDEX job_locks_key_uniq ON public.job_locks USING btree (key);

-- Recreate entity_tags table
CREATE TABLE "entity_tags" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "tag_id" uuid NOT NULL,
    "entity" varchar(255),
    "entity_id" uuid NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Recreate resources table
CREATE TABLE "resources" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "type" varchar(255),
    "name" varchar(255),
    "value" jsonb,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);

-- Recreate notifications table
CREATE TABLE "notifications" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "user_id" uuid NOT NULL,
    "content_type" varchar(255),
    "content" jsonb,
    "read_at" timestamptz,
    "expires_at" timestamptz,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY ("id")
);
