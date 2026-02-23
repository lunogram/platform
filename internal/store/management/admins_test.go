package management

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminsStore(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	t.Run("creates admin", func(t *testing.T) {
		adminID, err := db.CreateAdmin(ctx, Admin{
			OrganizationID: orgID,
			ExternalID:     ptr("ext-123"),
			Email:          "admin@example.com",
			FirstName:      ptr("Test"),
			LastName:       ptr("Admin"),
		})
		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, adminID)

		admin, err := db.GetAdmin(ctx, adminID)
		require.NoError(t, err)
		assert.Equal(t, "admin@example.com", admin.Email)
		assert.Equal(t, "Test", *admin.FirstName)
		assert.Equal(t, "Admin", *admin.LastName)
	})

	t.Run("finds admin by email", func(t *testing.T) {
		_, err := db.CreateAdmin(ctx, Admin{
			OrganizationID: orgID,
			Email:          "findme@example.com",
		})
		require.NoError(t, err)

		admin, err := db.GetAdminByEmail(ctx, "findme@example.com")
		require.NoError(t, err)
		assert.Equal(t, "findme@example.com", admin.Email)
	})
}
