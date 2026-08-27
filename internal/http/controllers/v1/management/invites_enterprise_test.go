//go:build enterprise

package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

type inviteTestEnv struct {
	controller *InviteController
	engine     *rbac.Engine
	mgmt       *management.State
	orgID      uuid.UUID
	projectID  uuid.UUID
}

func newInviteTestEnv(t *testing.T) inviteTestEnv {
	t.Helper()
	ctx := context.Background()

	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	mgmt := management.NewState(mgmtDB)
	engine := rbac.NewTestEngine(t)
	logger := zaptest.NewLogger(t)

	orgID, err := mgmt.CreateOrganization(ctx, "Invite Org")
	require.NoError(t, err)

	projectID, err := mgmt.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Invite Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	// Provision the project so project-scoped RBAC checks resolve.
	require.NoError(t, access.ProvisionProject(ctx, engine, orgID, projectID))

	return inviteTestEnv{
		controller: NewInviteController(logger, mgmt, engine, mgmtDB),
		engine:     engine,
		mgmt:       mgmt,
		orgID:      orgID,
		projectID:  projectID,
	}
}

// newAdmin creates an admin whose address has been proved, in the env's org, and
// returns its id together with a constructor for requests whose context carries
// that admin as the RBAC actor.
func (env inviteTestEnv) newAdmin(t *testing.T, email string) (uuid.UUID, func() *http.Request) {
	t.Helper()
	return env.newAdminWithVerifiedEmail(t, email, true)
}

// newAdminWithVerifiedEmail creates an admin holding one identity, which either
// proves their address or does not. Invite handling turns on that distinction,
// so it has to be expressible.
func (env inviteTestEnv) newAdminWithVerifiedEmail(t *testing.T, email string, verified bool) (uuid.UUID, func() *http.Request) {
	t.Helper()
	adminID, err := env.mgmt.CreateAdmin(context.Background(), management.Admin{
		OrganizationID: env.orgID,
		Email:          email,
		Role:           "member",
	})
	require.NoError(t, err)

	_, err = env.mgmt.CreateAdminIdentity(context.Background(), management.AdminIdentity{
		AdminID:       adminID,
		Provider:      management.IdentityProviderClerk,
		Issuer:        "https://idp.test",
		Subject:       "user_" + adminID.String(),
		Email:         &email,
		EmailVerified: verified,
	})
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(), rbac.WithOrganizationID(env.orgID))
	ctx := rbac.WithActor(context.Background(), actor)
	newReq := func() *http.Request {
		return httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
	}
	return adminID, newReq
}

func (env inviteTestEnv) createInvite(t *testing.T, inviterID uuid.UUID, email string, role oapi.CreateProjectInviteRole, ttl time.Duration) *management.Invite {
	t.Helper()
	invite, err := env.mgmt.CreateProjectInvite(context.Background(), env.projectID, inviterID, email, nil, role, ttl)
	require.NoError(t, err)
	return invite
}

func (env inviteTestEnv) forceExpire(t *testing.T, inviteID uuid.UUID) {
	t.Helper()
	_, err := env.controller.db.ExecContext(context.Background(),
		`UPDATE project_invites SET expires_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, inviteID)
	require.NoError(t, err)
}

// An invite is addressed to a mailbox, so accepting one has to mean the account
// holds that mailbox.
//
// Matching admins.email against the invite used to be sufficient because every
// driver authenticated through an upstream that had already verified the
// address. Local passwords broke that assumption: the address on an account is
// whatever was typed into a registration form. Without this check, registering
// a guessed victim@example.com is enough to take the project role their invite
// grants.
func TestAcceptProjectInviteRequiresAVerifiedEmail(t *testing.T) {
	t.Parallel()

	env := newInviteTestEnv(t)
	inviterID, _ := env.newAdmin(t, "inviter@example.com")
	inviteeID, newReq := env.newAdminWithVerifiedEmail(t, "invitee@example.com", false)

	invite := env.createInvite(t, inviterID, "invitee@example.com", "editor", time.Hour)

	res := httptest.NewRecorder()
	env.controller.AcceptProjectInvite(res, newReq(), invite.ID)
	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())

	admin, err := env.mgmt.GetProjectAdmin(context.Background(), env.projectID, inviteeID)
	require.Error(t, err, "an unverified admin must not have been added to the project")
	require.Nil(t, admin)

	stored, err := env.mgmt.GetInviteByID(context.Background(), invite.ID)
	require.NoError(t, err)
	assert.Nil(t, stored.AcceptedAt, "the invite must still be pending")
}

// The same reasoning applies to merely SEEING an invite: who invited you and to
// which project is the invited mailbox's business, not that of whoever managed
// to register the address.
func TestListMyInvitesWithholdsFromUnverifiedAddresses(t *testing.T) {
	t.Parallel()

	env := newInviteTestEnv(t)
	inviterID, _ := env.newAdmin(t, "lister-inviter@example.com")
	_, newReq := env.newAdminWithVerifiedEmail(t, "lister-invitee@example.com", false)

	env.createInvite(t, inviterID, "lister-invitee@example.com", "editor", time.Hour)

	res := httptest.NewRecorder()
	env.controller.ListMyInvites(res, newReq())
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	assert.NotContains(t, res.Body.String(), "lister-inviter@example.com",
		"an unverified address must not be shown a stranger's invite")
}

func TestAcceptProjectInvite(t *testing.T) {
	t.Parallel()

	t.Run("accepts a pending invite and grants RBAC access", func(t *testing.T) {
		env := newInviteTestEnv(t)
		inviterID, _ := env.newAdmin(t, "inviter@example.com")
		adminID, newReq := env.newAdmin(t, "invitee@example.com")
		invite := env.createInvite(t, inviterID, "invitee@example.com", oapi.CreateProjectInviteRoleEditor, time.Hour)

		rec := httptest.NewRecorder()
		env.controller.AcceptProjectInvite(rec, newReq(), invite.ID)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		pa, err := env.mgmt.GetProjectAdmin(context.Background(), env.projectID, adminID)
		require.NoError(t, err)
		assert.Equal(t, "editor", pa.Role)

		allowed, err := env.engine.Check(context.Background(), "user:"+adminID.String(), "read", rbac.ProjectResourceScope("users", env.projectID))
		require.NoError(t, err)
		assert.True(t, allowed, "accepted invitee should have project access")
	})

	t.Run("rejects accept by the wrong email with 403", func(t *testing.T) {
		env := newInviteTestEnv(t)
		inviterID, _ := env.newAdmin(t, "inviter@example.com")
		invite := env.createInvite(t, inviterID, "intended@example.com", oapi.CreateProjectInviteRoleEditor, time.Hour)
		_, newReq := env.newAdmin(t, "someone-else@example.com")

		rec := httptest.NewRecorder()
		env.controller.AcceptProjectInvite(rec, newReq(), invite.ID)
		assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("rejects accept of a revoked invite with 4xx", func(t *testing.T) {
		env := newInviteTestEnv(t)
		inviterID, _ := env.newAdmin(t, "inviter@example.com")
		_, newReq := env.newAdmin(t, "revoked@example.com")
		invite := env.createInvite(t, inviterID, "revoked@example.com", oapi.CreateProjectInviteRoleEditor, time.Hour)
		_, err := env.mgmt.RevokeProjectInvite(context.Background(), invite.ID)
		require.NoError(t, err)

		rec := httptest.NewRecorder()
		env.controller.AcceptProjectInvite(rec, newReq(), invite.ID)
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("rejects accept of an expired invite with 4xx", func(t *testing.T) {
		env := newInviteTestEnv(t)
		inviterID, _ := env.newAdmin(t, "inviter@example.com")
		_, newReq := env.newAdmin(t, "expired@example.com")
		invite := env.createInvite(t, inviterID, "expired@example.com", oapi.CreateProjectInviteRoleEditor, time.Hour)
		env.forceExpire(t, invite.ID)

		rec := httptest.NewRecorder()
		env.controller.AcceptProjectInvite(rec, newReq(), invite.ID)
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("double accept is idempotent and reconciles RBAC tuples", func(t *testing.T) {
		env := newInviteTestEnv(t)
		inviterID, _ := env.newAdmin(t, "inviter@example.com")
		adminID, newReq := env.newAdmin(t, "twice@example.com")
		invite := env.createInvite(t, inviterID, "twice@example.com", oapi.CreateProjectInviteRoleEditor, time.Hour)

		rec1 := httptest.NewRecorder()
		env.controller.AcceptProjectInvite(rec1, newReq(), invite.ID)
		require.Equal(t, http.StatusOK, rec1.Code, rec1.Body.String())

		// Simulate the stranded-membership case: drop the RBAC tuples that the
		// first accept wrote, leaving Postgres membership without access.
		require.NoError(t, env.engine.DeleteTuples(context.Background(), access.ProjectRoleTuples(adminID, env.projectID, "editor")))
		allowed, err := env.engine.Check(context.Background(), "user:"+adminID.String(), "read", rbac.ProjectResourceScope("users", env.projectID))
		require.NoError(t, err)
		require.False(t, allowed)

		// Re-accept must succeed (idempotent) and repair the missing tuples.
		rec2 := httptest.NewRecorder()
		env.controller.AcceptProjectInvite(rec2, newReq(), invite.ID)
		require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())

		allowed, err = env.engine.Check(context.Background(), "user:"+adminID.String(), "read", rbac.ProjectResourceScope("users", env.projectID))
		require.NoError(t, err)
		assert.True(t, allowed, "re-accept should reconcile RBAC access")
	})

	t.Run("accept upgrades an existing lower project role", func(t *testing.T) {
		env := newInviteTestEnv(t)
		inviterID, _ := env.newAdmin(t, "inviter@example.com")
		adminID, newReq := env.newAdmin(t, "upgrade@example.com")

		// Pre-existing support membership.
		require.NoError(t, env.mgmt.AddAdminToProject(context.Background(), env.projectID, adminID, "support"))
		require.NoError(t, env.engine.WriteTuples(context.Background(), access.ProjectRoleTuples(adminID, env.projectID, "support")))

		invite := env.createInvite(t, inviterID, "upgrade@example.com", oapi.CreateProjectInviteRoleAdmin, time.Hour)

		rec := httptest.NewRecorder()
		env.controller.AcceptProjectInvite(rec, newReq(), invite.ID)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		pa, err := env.mgmt.GetProjectAdmin(context.Background(), env.projectID, adminID)
		require.NoError(t, err)
		assert.Equal(t, "admin", pa.Role, "role should be upgraded to admin")

		// Admin-level permission (delete inbox requires admin) should now resolve.
		allowed, err := env.engine.Check(context.Background(), "user:"+adminID.String(), "delete", rbac.ProjectResourceScope("inbox", env.projectID))
		require.NoError(t, err)
		assert.True(t, allowed, "upgraded admin should have admin-level access")
	})
}
