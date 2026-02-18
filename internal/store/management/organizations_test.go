package management

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrganizationsStore(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	t.Run("creates organization", func(t *testing.T) {
		orgID, err := db.CreateOrganization(ctx, "Test Organization")
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, orgID)

		org, err := db.GetOrganization(ctx, orgID)
		require.NoError(t, err)
		assert.Equal(t, "Test Organization", org.Name)
	})
}
