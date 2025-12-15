package store

import (
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/pkg/container"
	"github.com/stretchr/testify/require"
)

func NewContainerStore(t *testing.T) *Stores {
	t.Helper()

	ctx := graceful.NewContext(t.Context())
	config := Config{
		URI: container.RunPostgreSQL(t),
	}

	err := Migrate(config)
	require.NoError(t, err)

	db, err := Connect(ctx, config)
	require.NoError(t, err)

	return NewStores(db)
}

func ptr[T any](v T) *T {
	return &v
}
