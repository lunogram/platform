package container

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// RunPostgreSQL runs a PostgreSQL container for testing and returns the connection string
// to a unique database created specifically for this test.
func RunPostgreSQL(t *testing.T) (uri string) {
	t.Helper()
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:18.1-alpine",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("password"),
		postgres.BasicWaitStrategies(),
		testcontainers.WithReuseByName("testcontainer-postgresql"),
		// NOTE: increase max_connections to support parallel test execution
		// Default is 100, we increase to 500 for parallel tests
		testcontainers.WithCmdArgs("-c", "max_connections=500"),
	)
	require.NoError(t, err)

	adminURI, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	admin, err := sql.Open("pgx", adminURI)
	require.NoError(t, err)
	defer admin.Close()

	database := fmt.Sprintf("app_%d", time.Now().UnixNano())
	_, err = admin.Exec(fmt.Sprintf("CREATE DATABASE %s", database))
	require.NoError(t, err)

	return strings.Replace(adminURI, "/postgres?", "/"+database+"?", 1)
}

// AddSearchPath adds a search_path parameter to a PostgreSQL connection URI.
// This allows isolating tables into separate schemas within the same database.
func AddSearchPath(uri, schema string) string {
	if strings.Contains(uri, "?") {
		return uri + "&search_path=" + schema
	}
	return uri + "?search_path=" + schema
}

// CreateSchema creates a schema in the database if it doesn't exist.
func CreateSchema(t *testing.T, uri, schema string) {
	t.Helper()

	db, err := sql.Open("pgx", uri)
	require.NoError(t, err)
	defer db.Close()

	_, err = db.Exec(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema))
	require.NoError(t, err)
}
