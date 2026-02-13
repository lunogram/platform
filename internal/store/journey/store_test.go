package journey

import (
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/users"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var (
	testDB   *store.Connections
	testOnce bool
)

func NewContainerStore(t *testing.T) *State {
	t.Helper()

	logger := zaptest.NewLogger(t)

	ctx := graceful.NewContext(t.Context())
	uri := container.RunPostgreSQL(t)

	err := management.Migrate(uri)
	require.NoError(t, err)

	err = users.Migrate(uri)
	require.NoError(t, err)

	err = Migrate(uri)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: uri,
		UsersURI:      uri,
		JourneyURI:    uri,
	})
	require.NoError(t, err)

	testDB = db
	testOnce = true

	return NewState(db.Journey)
}

func ManagementStore(t *testing.T) *management.State {
	t.Helper()
	if !testOnce {
		t.Fatal("must call NewContainerStore before ManagementStore")
	}
	return management.NewState(testDB.Management)
}

func DB(t *testing.T) *sqlx.DB {
	t.Helper()
	if !testOnce {
		t.Fatal("must call NewContainerStore before DB")
	}
	return testDB.Journey
}
