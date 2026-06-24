package management

import (
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func NewContainerStore(t *testing.T) *State {
	t.Helper()
	state, _ := newContainerStoreWithDB(t)
	return state
}

// newContainerStoreWithDB is like NewContainerStore but also returns the
// underlying *sqlx.DB so tests that need to set up edge states (e.g. forcing an
// invite's expiry) can run raw SQL.
func newContainerStoreWithDB(t *testing.T) (*State, store.DB) {
	t.Helper()

	uri := container.RunPostgreSQL(t)
	mgmtURI := container.CreateSchema(t, uri, "management")
	require.NoError(t, Migrate(mgmtURI))

	ctx := graceful.NewContext(t.Context())
	logger := zaptest.NewLogger(t)
	db, err := store.Connect(ctx, logger, mgmtURI)
	require.NoError(t, err)

	return NewState(db), db
}
