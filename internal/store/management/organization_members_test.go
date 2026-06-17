package management

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lunogram/platform/internal/store"
)

func TestOrganizationMembersStore(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	orgID, err := db.CreateOrganization(ctx, "Home Org")
	require.NoError(t, err)

	otherOrgID, err := db.CreateOrganization(ctx, "Other Org")
	require.NoError(t, err)

	adminID, err := db.CreateAdmin(ctx, Admin{
		OrganizationID: orgID,
		Email:          "member@example.com",
		Role:           "owner",
	})
	require.NoError(t, err)

	t.Run("CreateAdmin sets active organization to home org", func(t *testing.T) {
		admin, err := db.GetAdmin(ctx, adminID)
		require.NoError(t, err)
		require.NotNil(t, admin.ActiveOrganizationID)
		assert.Equal(t, orgID, *admin.ActiveOrganizationID)
	})

	t.Run("AddMember then IsMember/GetMember", func(t *testing.T) {
		require.NoError(t, db.AddMember(ctx, orgID, adminID, "owner"))

		ok, err := db.IsMember(ctx, orgID, adminID)
		require.NoError(t, err)
		assert.True(t, ok)

		member, err := db.GetMember(ctx, orgID, adminID)
		require.NoError(t, err)
		assert.Equal(t, "owner", member.Role)

		ok, err = db.IsMember(ctx, otherOrgID, adminID)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("AddMember is idempotent and updates role", func(t *testing.T) {
		require.NoError(t, db.AddMember(ctx, orgID, adminID, "admin"))
		member, err := db.GetMember(ctx, orgID, adminID)
		require.NoError(t, err)
		assert.Equal(t, "admin", member.Role)
		// restore
		require.NoError(t, db.AddMember(ctx, orgID, adminID, "owner"))
	})

	t.Run("ListAdmins returns members of the organization", func(t *testing.T) {
		admins, total, err := db.ListAdmins(ctx, orgID, store.Pagination{Limit: 20, Offset: 0}, "")
		require.NoError(t, err)
		assert.Equal(t, 1, total)
		require.Len(t, admins, 1)
		assert.Equal(t, adminID, admins[0].ID)

		// The admin is not a member of the other org, so it must not appear there.
		_, total, err = db.ListAdmins(ctx, otherOrgID, store.Pagination{Limit: 20, Offset: 0}, "")
		require.NoError(t, err)
		assert.Equal(t, 0, total)
	})

	t.Run("cross-org membership and ListOrganizationsForAdmin", func(t *testing.T) {
		require.NoError(t, db.AddMember(ctx, otherOrgID, adminID, "member"))

		orgs, err := db.ListOrganizationsForAdmin(ctx, adminID)
		require.NoError(t, err)
		require.Len(t, orgs, 2)
		// ordered by name ASC: "Home Org" then "Other Org"
		assert.Equal(t, "Home Org", orgs[0].Name)
		assert.Equal(t, "owner", orgs[0].Role)
		assert.Equal(t, "Other Org", orgs[1].Name)
		assert.Equal(t, "member", orgs[1].Role)
	})

	t.Run("SetActiveOrganization", func(t *testing.T) {
		require.NoError(t, db.SetActiveOrganization(ctx, adminID, otherOrgID))
		admin, err := db.GetAdmin(ctx, adminID)
		require.NoError(t, err)
		require.NotNil(t, admin.ActiveOrganizationID)
		assert.Equal(t, otherOrgID, *admin.ActiveOrganizationID)
	})

	t.Run("RemoveMember then revive", func(t *testing.T) {
		require.NoError(t, db.RemoveMember(ctx, otherOrgID, adminID))

		ok, err := db.IsMember(ctx, otherOrgID, adminID)
		require.NoError(t, err)
		assert.False(t, ok)

		// Reviving a removed membership works (ON CONFLICT clears deleted_at).
		require.NoError(t, db.AddMember(ctx, otherOrgID, adminID, "member"))
		ok, err = db.IsMember(ctx, otherOrgID, adminID)
		require.NoError(t, err)
		assert.True(t, ok)
	})
}
