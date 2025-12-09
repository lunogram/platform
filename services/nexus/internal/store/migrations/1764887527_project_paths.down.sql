DROP TRIGGER IF EXISTS set_updated_at_user_schemas ON user_schemas;
DROP TABLE IF EXISTS user_schemas;

DROP TRIGGER IF EXISTS set_updated_at_event_schemas ON event_schemas;
DROP TABLE IF EXISTS event_schemas;

DROP TRIGGER IF EXISTS set_updated_at_events ON events;
DROP TABLE IF EXISTS events;

DROP TYPE IF EXISTS data_type;

CREATE TABLE "public"."project_rule_paths" (
    "id" uuid NOT NULL DEFAULT uuid_generate_v4(),
    "project_id" uuid NOT NULL,
    "path" varchar(255) NOT NULL,
    "name" varchar(255),
    "type" varchar(50) NOT NULL,
    "created_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated_at" timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "data_type" varchar(255) DEFAULT 'string'::character varying,
    "visibility" "public"."project_rule_paths_visibility" NOT NULL DEFAULT 'public'::project_rule_paths_visibility,
    CONSTRAINT "project_rule_paths_project_id_fkey" FOREIGN KEY ("project_id") REFERENCES "public"."projects"("id") ON DELETE CASCADE,
    PRIMARY KEY ("id")
);

CREATE INDEX project_rule_paths_project_id_idx ON public.project_rule_paths USING btree (project_id);