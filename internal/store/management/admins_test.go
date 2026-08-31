package management

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
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
			Email:          "admin@example.com",
			FirstName:      ptr.To("Test"),
			LastName:       ptr.To("Admin"),
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

// Removing a project member is a soft delete, so the row survives every read
// that filters it out — and an INSERT still collides with it. Re-adding somebody
// you removed has to revive that row rather than fail on a duplicate key.
func TestAddAdminToProjectRevivesARemovedMembership(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Rejoin Organization")
	require.NoError(t, err)

	adminID, err := db.CreateAdmin(ctx, Admin{OrganizationID: orgID, Email: "rejoin@example.com"})
	require.NoError(t, err)

	projectID, err := db.CreateProject(ctx, Project{
		OrganizationID: &orgID, Name: "Rejoin Project", Timezone: "UTC", Locale: "en",
	})
	require.NoError(t, err)

	require.NoError(t, db.AddAdminToProject(ctx, projectID, adminID, "editor"))
	require.NoError(t, db.DeleteProjectAdmin(ctx, projectID, adminID))

	_, err = db.GetProjectAdmin(ctx, projectID, adminID)
	require.Error(t, err, "a removed member must not read back")

	require.NoError(t, db.AddAdminToProject(ctx, projectID, adminID, "admin"),
		"re-adding a removed member must not collide with the row left behind")

	member, err := db.GetProjectAdmin(ctx, projectID, adminID)
	require.NoError(t, err)
	assert.Equal(t, "admin", member.Role, "the role of the new grant wins")
}
