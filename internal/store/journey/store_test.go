package journey

import (
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func NewContainerStore(t *testing.T) (*State, *sqlx.DB) {
	t.Helper()

	uri := container.RunPostgreSQL(t)
	journeyURI := container.CreateSchema(t, uri, "journey")

	require.NoError(t, Migrate(journeyURI))

	ctx := graceful.NewContext(t.Context())
	logger := zaptest.NewLogger(t)

	journeyDB, err := store.Connect(ctx, logger, journeyURI)
	require.NoError(t, err)

	return NewState(journeyDB), journeyDB
}
