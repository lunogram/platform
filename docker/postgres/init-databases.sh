#!/bin/bash
set -e

# Create the three databases for the split store architecture
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE management;
    CREATE DATABASE subjects;
    CREATE DATABASE journey;
    CREATE DATABASE rbac;

    -- Grant privileges
    GRANT ALL PRIVILEGES ON DATABASE management TO $POSTGRES_USER;
    GRANT ALL PRIVILEGES ON DATABASE subjects TO $POSTGRES_USER;
    GRANT ALL PRIVILEGES ON DATABASE journey TO $POSTGRES_USER;
    GRANT ALL PRIVILEGES ON DATABASE rbac TO $POSTGRES_USER;
EOSQL

echo "Created databases: management, subjects, journey"
