package container

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// RunPostgreSQL runs a PostgreSQL container and returns the database URI.
func RunPostgreSQL(t *testing.T) string {
	t.Helper()
	return createTestDatabase(t)
}

// CreateSchema creates the given schema and returns the URI with search_path set.
func CreateSchema(t *testing.T, uri, schema string) string {
	t.Helper()

	db, err := sql.Open("pgx", uri)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema))
	require.NoError(t, err)

	return addSearchPath(uri, schema)
}

// containerName scopes the reused container to the current test session.
// Every package within a single "go test" invocation shares one container, while
// independent runs — concurrent worktrees, a second run on the same machine — each
// get their own. A name shared across sessions is unsafe because reuse registers
// the container with the reusing session's reaper, so the first session to finish
// tears the container down while the others are still connected to it.
func containerName() string {
	id := testcontainers.SessionID()
	return "testcontainer-postgresql-" + id[:min(len(id), 12)]
}

// createTestDatabase creates a new test database and returns its URI.
func createTestDatabase(t *testing.T) string {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:18.1-alpine",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("password"),
		postgres.BasicWaitStrategies(),
		testcontainers.WithReuseByName(containerName()),
		// NOTE: increase max_connections to support parallel test execution
		// Default is 100, we increase to 1000 for parallel tests
		testcontainers.WithCmdArgs("-c", "max_connections=1000"),
	)
	require.NoError(t, err)

	adminURI, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	admin, err := sql.Open("pgx", adminURI)
	require.NoError(t, err)
	admin.SetMaxOpenConns(2)
	admin.SetMaxIdleConns(1)
	defer admin.Close()

	database := "app_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = admin.Exec(fmt.Sprintf("CREATE DATABASE %s", database))
	require.NoError(t, err)

	return strings.Replace(adminURI, "/postgres?", "/"+database+"?", 1)
}

// addSearchPath adds a search_path parameter to a PostgreSQL connection URI.
// It includes the public schema so extensions like uuid-ossp are visible.
func addSearchPath(uri, schema string) string {
	searchPath := schema + ",public"
	if strings.Contains(uri, "?") {
		return uri + "&search_path=" + searchPath
	}
	return uri + "?search_path=" + searchPath
}
