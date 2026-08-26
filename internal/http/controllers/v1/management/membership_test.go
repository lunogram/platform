package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rbac/access"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// createAdminRequest posts the given email and role to CreateAdmin as the
// organization owner in actorCtx, and returns the recorded response.
func createAdminRequest(t *testing.T, controller *AdminsController, actorCtx context.Context, email string, role oapi.OrganizationRole) *httptest.ResponseRecorder {
	t.Helper()

	body, err := json.Marshal(oapi.CreateAdmin{Email: email, Role: role})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/admins", bytes.NewReader(body))
	controller.CreateAdmin(res, req.WithContext(actorCtx))
	return res
}

// heldOrganizationRoles returns the organization roles the admin holds as
// directly assigned tuples. The role hierarchy makes engine.Check useless here:
// an owner tuple satisfies a member check, so only reading the stored tuples
// back shows which roles are really present.
func heldOrganizationRoles(t *testing.T, engine *rbac.Engine, adminID, orgID uuid.UUID) []string {
	t.Helper()

	var held []string
	for _, role := range access.OrganizationRoles() {
		present, err := engine.HasTuple(t.Context(), "user:"+adminID.String(), role, rbac.OrganizationScope(orgID))
		require.NoError(t, err)
		if present {
			held = append(held, role)
		}
	}
	return held
}

func TestCreateAdminIsIdempotentForAnUnchangedRole(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)
	state := management.NewState(mgmt)

	orgID, err := state.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	callerID, err := state.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "owner@example.com",
		Role:           rbac.OrganizationOwner,
	})
	require.NoError(t, err)
	require.NoError(t, state.AddMember(ctx, orgID, callerID, rbac.OrganizationOwner))

	// The invitee is already a registered admin, so CreateAdmin takes the
	// "email already belongs to a registered admin" branch and only grants an
	// organization membership.
	inviteeID, err := state.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "invitee@example.com",
		Role:           rbac.OrganizationMember,
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, callerID.String(), rbac.WithOrganizationID(orgID))
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, rbac.OrganizationOwner, "")
	controller := NewAdminsController(logger, mgmt, engine)

	res := createAdminRequest(t, controller, actorCtx, "invitee@example.com", oapi.OrganizationRoleMember)
	require.Equal(t, 201, res.Code, res.Body.String())

	// Granting the same membership again must succeed. The membership upsert is
	// idempotent, so before the tuple write became idempotent too this failed on
	// OpenFGA's "cannot write a tuple which already exists" and returned a 500
	// even though the database half had committed.
	res = createAdminRequest(t, controller, actorCtx, "invitee@example.com", oapi.OrganizationRoleMember)
	require.Equal(t, 201, res.Code, res.Body.String())

	member, err := state.GetMember(ctx, orgID, inviteeID)
	require.NoError(t, err)
	assert.Equal(t, rbac.OrganizationMember, member.Role)

	assert.Equal(t, []string{rbac.OrganizationMember}, heldOrganizationRoles(t, engine, inviteeID, orgID))
}

func TestCreateAdminReplacesTheTupleOfASupersededRole(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)
	state := management.NewState(mgmt)

	orgID, err := state.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	callerID, err := state.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "owner@example.com",
		Role:           rbac.OrganizationOwner,
	})
	require.NoError(t, err)
	require.NoError(t, state.AddMember(ctx, orgID, callerID, rbac.OrganizationOwner))

	inviteeID, err := state.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "invitee@example.com",
		Role:           rbac.OrganizationMember,
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, callerID.String(), rbac.WithOrganizationID(orgID))
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, rbac.OrganizationOwner, "")
	controller := NewAdminsController(logger, mgmt, engine)

	res := createAdminRequest(t, controller, actorCtx, "invitee@example.com", oapi.OrganizationRoleOwner)
	require.Equal(t, 201, res.Code, res.Body.String())

	// AddMember updates the role column in place, so the demotion has to take the
	// owner tuple with it; leaving it behind would keep the admin an owner in
	// OpenFGA while the database says member.
	res = createAdminRequest(t, controller, actorCtx, "invitee@example.com", oapi.OrganizationRoleMember)
	require.Equal(t, 201, res.Code, res.Body.String())

	member, err := state.GetMember(ctx, orgID, inviteeID)
	require.NoError(t, err)
	assert.Equal(t, rbac.OrganizationMember, member.Role)

	assert.Equal(t, []string{rbac.OrganizationMember}, heldOrganizationRoles(t, engine, inviteeID, orgID))

	scope := rbac.OrganizationScope(orgID)
	allowed, err := engine.Check(ctx, "user:"+inviteeID.String(), string(rbac.Delete), scope)
	require.NoError(t, err)
	assert.False(t, allowed, "a demoted owner must lose owner-only permissions")

	allowed, err = engine.Check(ctx, "user:"+inviteeID.String(), string(rbac.Read), scope)
	require.NoError(t, err)
	assert.True(t, allowed, "the admin is still a member and can read the organization")
}
