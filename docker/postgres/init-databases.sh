#!/bin/bash
set -e

# Create the three databases for the split store architecture
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE management;
    CREATE DATABASE users;
    CREATE DATABASE journey;
    
    -- Grant privileges
    GRANT ALL PRIVILEGES ON DATABASE management TO $POSTGRES_USER;
    GRANT ALL PRIVILEGES ON DATABASE users TO $POSTGRES_USER;
    GRANT ALL PRIVILEGES ON DATABASE journey TO $POSTGRES_USER;
EOSQL

echo "Created databases: management, users, journey"
