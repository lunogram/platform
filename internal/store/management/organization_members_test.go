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

// TestReconcileAdminOrganizations covers the cleanup that runs after a
// membership is removed: a dangling active_organization_id is cleared, and a
// home organization pointing at the removed org is re-pointed to a remaining
// membership.
func TestReconcileAdminOrganizations(t *testing.T) {
	t.Parallel()
	db := NewContainerStore(t)
	ctx := context.Background()

	t.Run("clears dangling active org and repoints home", func(t *testing.T) {
		homeOrg, err := db.CreateOrganization(ctx, "Home")
		require.NoError(t, err)
		secondOrg, err := db.CreateOrganization(ctx, "Second")
		require.NoError(t, err)

		adminID, err := db.CreateAdmin(ctx, Admin{
			OrganizationID: homeOrg,
			Email:          "reconcile@example.com",
			Role:           "owner",
		})
		require.NoError(t, err)
		require.NoError(t, db.AddMember(ctx, homeOrg, adminID, "owner"))
		require.NoError(t, db.AddMember(ctx, secondOrg, adminID, "member"))

		// Active org is the home org; now remove the home membership.
		require.NoError(t, db.SetActiveOrganization(ctx, adminID, homeOrg))
		require.NoError(t, db.RemoveMember(ctx, homeOrg, adminID))
		require.NoError(t, db.ReconcileAdminOrganizations(ctx, homeOrg, adminID))

		admin, err := db.GetAdmin(ctx, adminID)
		require.NoError(t, err)
		// active_organization_id pointed at the removed org → cleared.
		assert.Nil(t, admin.ActiveOrganizationID)
		// home organization_id pointed at the removed org → re-pointed to the
		// only remaining membership.
		assert.Equal(t, secondOrg, admin.OrganizationID)
	})

	t.Run("leaves home org alone when removed org was not home", func(t *testing.T) {
		homeOrg, err := db.CreateOrganization(ctx, "Home2")
		require.NoError(t, err)
		secondOrg, err := db.CreateOrganization(ctx, "Second2")
		require.NoError(t, err)

		adminID, err := db.CreateAdmin(ctx, Admin{
			OrganizationID: homeOrg,
			Email:          "reconcile2@example.com",
			Role:           "owner",
		})
		require.NoError(t, err)
		require.NoError(t, db.AddMember(ctx, homeOrg, adminID, "owner"))
		require.NoError(t, db.AddMember(ctx, secondOrg, adminID, "member"))

		// Active org is the second org; remove the second membership.
		require.NoError(t, db.SetActiveOrganization(ctx, adminID, secondOrg))
		require.NoError(t, db.RemoveMember(ctx, secondOrg, adminID))
		require.NoError(t, db.ReconcileAdminOrganizations(ctx, secondOrg, adminID))

		admin, err := db.GetAdmin(ctx, adminID)
		require.NoError(t, err)
		assert.Nil(t, admin.ActiveOrganizationID, "stale active org cleared")
		assert.Equal(t, homeOrg, admin.OrganizationID, "home org untouched")
	})

	t.Run("keeps home org when removed org was the only membership", func(t *testing.T) {
		homeOrg, err := db.CreateOrganization(ctx, "Solo")
		require.NoError(t, err)

		adminID, err := db.CreateAdmin(ctx, Admin{
			OrganizationID: homeOrg,
			Email:          "reconcile3@example.com",
			Role:           "owner",
		})
		require.NoError(t, err)
		require.NoError(t, db.AddMember(ctx, homeOrg, adminID, "owner"))

		require.NoError(t, db.RemoveMember(ctx, homeOrg, adminID))
		require.NoError(t, db.ReconcileAdminOrganizations(ctx, homeOrg, adminID))

		admin, err := db.GetAdmin(ctx, adminID)
		require.NoError(t, err)
		// No remaining membership to re-point to; home pointer stays (NOT NULL).
		assert.Equal(t, homeOrg, admin.OrganizationID)
		assert.Nil(t, admin.ActiveOrganizationID)
	})
}
