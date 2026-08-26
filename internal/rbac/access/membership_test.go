package access

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// directRole reports whether the admin holds role as a directly assigned tuple.
// engine.Check resolves through the role hierarchy (an owner tuple satisfies an
// admin or member check), so it cannot tell a stale tuple from an inherited one;
// only reading the stored tuple back shows which role was actually written.
func directRole(t *testing.T, engine *rbac.Engine, adminID, orgID uuid.UUID, role string) bool {
	t.Helper()

	tuple := OrganizationRoleTuples(adminID, orgID, role)[0]
	present, err := engine.HasTuple(context.Background(), tuple.User, tuple.Relation, tuple.Object)
	require.NoError(t, err)
	return present
}

func TestSyncOrganizationRoleGrantsRole(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()
	adminID, orgID := uuid.New(), uuid.New()

	require.NoError(t, SyncOrganizationRole(ctx, engine, adminID, orgID, rbac.OrganizationMember))

	allowed, err := engine.Check(ctx, "user:"+adminID.String(), rbac.OrganizationMember, rbac.OrganizationScope(orgID))
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestSyncOrganizationRoleIsIdempotentForSameRole(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()
	adminID, orgID := uuid.New(), uuid.New()

	require.NoError(t, SyncOrganizationRole(ctx, engine, adminID, orgID, rbac.OrganizationAdmin))

	// Re-granting the role the admin already holds must succeed: OpenFGA rejects
	// writing a tuple that already exists, which used to surface as a 500 when an
	// existing member was added to their organization again.
	require.NoError(t, SyncOrganizationRole(ctx, engine, adminID, orgID, rbac.OrganizationAdmin),
		"re-granting an unchanged role must be idempotent")

	assert.True(t, directRole(t, engine, adminID, orgID, rbac.OrganizationAdmin),
		"the role tuple must survive a repeated grant")

	allowed, err := engine.Check(ctx, "user:"+adminID.String(), string(rbac.Update), rbac.OrganizationScope(orgID))
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestSyncOrganizationRoleRevokesSupersededRoleOnDemotion(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()
	adminID, orgID := uuid.New(), uuid.New()
	user := "user:" + adminID.String()
	scope := rbac.OrganizationScope(orgID)

	require.NoError(t, SyncOrganizationRole(ctx, engine, adminID, orgID, rbac.OrganizationOwner))
	require.NoError(t, SyncOrganizationRole(ctx, engine, adminID, orgID, rbac.OrganizationMember))

	assert.True(t, directRole(t, engine, adminID, orgID, rbac.OrganizationMember))
	assert.False(t, directRole(t, engine, adminID, orgID, rbac.OrganizationOwner),
		"the superseded owner tuple must be removed, not left alongside the new role")

	allowed, err := engine.Check(ctx, user, string(rbac.Read), scope)
	require.NoError(t, err)
	assert.True(t, allowed, "a member can still read the organization")

	allowed, err = engine.Check(ctx, user, string(rbac.Delete), scope)
	require.NoError(t, err)
	assert.False(t, allowed, "a demoted owner must lose owner-only permissions")
}

func TestSyncOrganizationRoleGrantsSupersedingRoleOnPromotion(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()
	adminID, orgID := uuid.New(), uuid.New()

	require.NoError(t, SyncOrganizationRole(ctx, engine, adminID, orgID, rbac.OrganizationMember))
	require.NoError(t, SyncOrganizationRole(ctx, engine, adminID, orgID, rbac.OrganizationOwner))

	assert.True(t, directRole(t, engine, adminID, orgID, rbac.OrganizationOwner))
	assert.False(t, directRole(t, engine, adminID, orgID, rbac.OrganizationMember),
		"the superseded member tuple must be removed")

	allowed, err := engine.Check(ctx, "user:"+adminID.String(), string(rbac.Delete), rbac.OrganizationScope(orgID))
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestSyncOrganizationRoleRejectsUnknownRole(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()
	adminID, orgID := uuid.New(), uuid.New()

	require.NoError(t, SyncOrganizationRole(ctx, engine, adminID, orgID, rbac.OrganizationOwner))

	// A role that is not an organization role must be rejected before anything is
	// revoked, so a typo cannot strip an admin of the role they really hold.
	require.Error(t, SyncOrganizationRole(ctx, engine, adminID, orgID, rbac.ProjectEditor))

	assert.True(t, directRole(t, engine, adminID, orgID, rbac.OrganizationOwner),
		"a rejected role must leave the existing grant intact")
}

func TestSyncOrganizationRoleDoesNotLeakAcrossOrganizations(t *testing.T) {
	t.Parallel()

	engine := rbac.NewTestEngine(t)
	ctx := context.Background()
	adminID, orgA, orgB := uuid.New(), uuid.New(), uuid.New()

	require.NoError(t, SyncOrganizationRole(ctx, engine, adminID, orgA, rbac.OrganizationOwner))
	require.NoError(t, SyncOrganizationRole(ctx, engine, adminID, orgB, rbac.OrganizationMember))

	assert.True(t, directRole(t, engine, adminID, orgA, rbac.OrganizationOwner),
		"reconciling one organization must not revoke roles held in another")

	allowed, err := engine.Check(ctx, "user:"+adminID.String(), string(rbac.Delete), rbac.OrganizationScope(orgB))
	require.NoError(t, err)
	assert.False(t, allowed)
}
