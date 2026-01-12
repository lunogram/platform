package store

import (
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/pkg/container"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func NewContainerStore(t *testing.T) *State {
	t.Helper()

	logger := zaptest.NewLogger(t)

	ctx := graceful.NewContext(t.Context())
	config := Config{
		URI: container.RunPostgreSQL(t),
	}

	err := Migrate(config)
	require.NoError(t, err)

	db, err := New(ctx, logger, config)
	require.NoError(t, err)

	return NewState(db)
}

func ptr[T any](v T) *T {
	return &v
}
