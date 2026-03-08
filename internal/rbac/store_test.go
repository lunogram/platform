package rbac_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewEnginePostgres verifies that NewEngine can initialize against a real
// PostgreSQL database, run OpenFGA migrations, write the authorization model,
// and perform basic tuple/check operations.
//
// This is an integration test that catches configuration issues such as
// MaxTypesPerAuthorizationModel defaulting to 0 when using the Postgres
// datastore (as opposed to the in-memory backend used in unit tests).
func TestNewEnginePostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	uri := container.RunPostgreSQL(t)

	// Run the OpenFGA migrations so the required tables exist.
	err := rbac.Migrate(rbac.Config{PostgresURI: uri})
	require.NoError(t, err, "OpenFGA migrations should succeed")

	ctx := context.Background()

	// This is the call that previously failed with:
	//   "The number of type definitions in an authorization model exceeds the allowed limit of 0"
	engine, err := rbac.NewEngine(ctx, rbac.Config{PostgresURI: uri})
	require.NoError(t, err, "NewEngine should initialize without error")
	defer engine.Close()

	// Verify the engine is fully functional by writing a tuple and checking it.
	orgID := uuid.New()
	obj := "organization:" + orgID.String()

	err = engine.WriteTuple(ctx, "user:test-user", "owner", obj)
	require.NoError(t, err, "WriteTuple should succeed")

	allowed, err := engine.Check(ctx, "user:test-user", "owner", obj)
	require.NoError(t, err, "Check should succeed")
	assert.True(t, allowed, "owner tuple should grant access")

	// Verify computed permissions work through the role hierarchy.
	allowed, err = engine.Check(ctx, "user:test-user", "read", obj)
	require.NoError(t, err)
	assert.True(t, allowed, "owner should inherit read via admin → member → read")

	// Verify a user without any tuples is denied.
	allowed, err = engine.Check(ctx, "user:other-user", "read", obj)
	require.NoError(t, err)
	assert.False(t, allowed, "user without tuples should be denied")
}

// TestNewEnginePostgresIdempotent verifies that calling NewEngine twice
// against the same database reuses the existing store and model rather than
// failing or creating duplicates.
func TestNewEnginePostgresIdempotent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	uri := container.RunPostgreSQL(t)

	err := rbac.Migrate(rbac.Config{PostgresURI: uri})
	require.NoError(t, err)

	ctx := context.Background()

	engine1, err := rbac.NewEngine(ctx, rbac.Config{PostgresURI: uri})
	require.NoError(t, err, "first NewEngine call should succeed")
	defer engine1.Close()

	// Write a tuple using the first engine.
	orgID := uuid.New()
	obj := "organization:" + orgID.String()
	require.NoError(t, engine1.WriteTuple(ctx, "user:u1", "member", obj))

	engine2, err := rbac.NewEngine(ctx, rbac.Config{PostgresURI: uri})
	require.NoError(t, err, "second NewEngine call should succeed")
	defer engine2.Close()

	// The second engine should see tuples written by the first.
	allowed, err := engine2.Check(ctx, "user:u1", "read", obj)
	require.NoError(t, err)
	assert.True(t, allowed, "second engine should see tuples from the first")
}
