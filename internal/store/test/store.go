package test

import (
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// RunPostgreSQL creates a PostgreSQL test container with all three schemas
// (management, users, journey) and returns the database connections.
func RunPostgreSQL(t *testing.T) (mgmt, usrs, jrny *sqlx.DB) {
	t.Helper()

	uri := container.RunPostgreSQL(t)
	mgmtURI := container.CreateSchema(t, uri, "management")
	usersURI := container.CreateSchema(t, uri, "users")
	journeyURI := container.CreateSchema(t, uri, "journey")

	require.NoError(t, management.Migrate(mgmtURI))
	require.NoError(t, subjects.Migrate(usersURI))
	require.NoError(t, journey.Migrate(journeyURI))

	ctx := graceful.NewContext(t.Context())
	logger := zaptest.NewLogger(t)

	mgmt, err := store.Connect(ctx, logger, mgmtURI)
	require.NoError(t, err)

	usrs, err = store.Connect(ctx, logger, usersURI)
	require.NoError(t, err)

	jrny, err = store.Connect(ctx, logger, journeyURI)
	require.NoError(t, err)

	return mgmt, usrs, jrny
}
