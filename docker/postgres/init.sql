-- Creates the role and databases the platform connects to. Safe to run on every
-- start: anything that already exists is skipped. Postgres has no CREATE
-- DATABASE IF NOT EXISTS and refuses CREATE DATABASE inside a DO block, so each
-- query below selects the statements still needed and psql's \gexec runs them.
--
-- docker-compose.yml runs it when the volume is first initialised; devenv runs
-- it on every `devenv up`.

SELECT 'CREATE ROLE postgres SUPERUSER LOGIN PASSWORD ''postgrespw'''
WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'postgres')\gexec

SELECT format('CREATE DATABASE %I OWNER postgres', name)
FROM unnest(ARRAY['management', 'subjects', 'journey', 'rbac']) AS name
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = name)\gexec
