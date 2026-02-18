package management

import (
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func ptr[T any](v T) *T {
	return &v
}

func NewContainerStore(t *testing.T) *State {
	t.Helper()

	uri := container.RunPostgreSQL(t)
	mgmtURI := container.CreateSchema(t, uri, "management")
	require.NoError(t, Migrate(mgmtURI))

	ctx := graceful.NewContext(t.Context())
	logger := zaptest.NewLogger(t)
	db, err := store.Connect(ctx, logger, mgmtURI)
	require.NoError(t, err)

	return NewState(db)
}
