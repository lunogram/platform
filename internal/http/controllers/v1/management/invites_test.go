package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/mailer"
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
	mail       *capturedMailer
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

	cfg := config.Node{PublicURL: "https://console.example.test", Mail: testMailConfig()}
	captured, dispatcher, renderer := testMailer(t, cfg)

	return inviteTestEnv{
		controller: NewInviteController(logger, mgmt, engine, mgmtDB, dispatcher, renderer),
		engine:     engine,
		mgmt:       mgmt,
		mail:       captured,
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

// newProjectAdmin creates an admin who holds project admin, which is what the
// create-invite handler checks before minting anything, and returns a
// constructor for requests carrying them as the RBAC actor.
func (env inviteTestEnv) newProjectAdmin(t *testing.T, email string) func(body string) *http.Request {
	t.Helper()
	ctx := context.Background()

	adminID, _ := env.newAdmin(t, email)
	require.NoError(t, env.mgmt.AddAdminToProject(ctx, env.projectID, adminID, rbac.ProjectAdmin))
	require.NoError(t, access.ProvisionProjectRole(ctx, env.engine, adminID, env.projectID, rbac.ProjectAdmin))

	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(), rbac.WithOrganizationID(env.orgID))
	return func(body string) *http.Request {
		return httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)).
			WithContext(rbac.WithActor(ctx, actor))
	}
}

// An invite nobody is told about is not much of an invite: creating one mails
// the address it was sent to.
func TestCreateProjectInviteMailsTheInvitee(t *testing.T) {
	t.Parallel()

	env := newInviteTestEnv(t)
	newReq := env.newProjectAdmin(t, "inviter@example.com")

	res := httptest.NewRecorder()
	env.controller.CreateProjectInvite(res, newReq(`{"email":"invitee@example.com","role":"editor"}`), env.projectID)
	require.Equal(t, http.StatusCreated, res.Code, res.Body.String())

	// The inviter's account carries no name, so the invitation names them by the
	// only other thing the invitee can recognise them by.
	message := env.mail.awaitSubject(t, "inviter@example.com invited you to Invite Project")
	assert.Equal(t, "invitee@example.com", message.To)
	assert.Equal(t, mailer.KindProjectInvite, message.Kind)
	assert.Contains(t, message.Text, "Invite Project")

	// One destination whatever the invitee's account state, because that state
	// can change between sending the mail and opening it. The page decides
	// whether they sign in or sign up; the address rides along so it can offer
	// to register the right one.
	assert.Equal(t, "https://console.example.test/invites?email=invitee%40example.com",
		message.ActionURL,
		"the link grants nothing on its own; the invite is claimed by proving the address")
}

// Mail is a notification, not the invite itself. A deployment that configures no
// channel still issues invites; they are found on the console's invites page.
func TestCreateProjectInviteWithoutAMailer(t *testing.T) {
	t.Parallel()

	env := newInviteTestEnv(t)
	env.controller.mail = nil
	env.controller.renderer = nil
	newReq := env.newProjectAdmin(t, "inviter@example.com")

	res := httptest.NewRecorder()
	env.controller.CreateProjectInvite(res, newReq(`{"email":"invitee@example.com","role":"editor"}`), env.projectID)
	require.Equal(t, http.StatusCreated, res.Code, res.Body.String())
	env.mail.awaitQuiet(t)
}

func (env inviteTestEnv) forceExpire(t *testing.T, inviteID uuid.UUID) {
	t.Helper()
	_, err := env.controller.db.ExecContext(context.Background(),
		`UPDATE project_invites SET expires_at = NOW() - INTERVAL '1 minute' WHERE id = $1`, inviteID)
	require.NoError(t, err)
}

// Confirming the address is no longer a precondition for accepting: an account
// may act on the invites addressed to it as soon as it exists. What still binds
// the invite is the address on the account, checked above this.
func TestAcceptProjectInviteWithoutAConfirmedAddress(t *testing.T) {
	t.Parallel()

	env := newInviteTestEnv(t)
	inviterID, _ := env.newAdmin(t, "inviter@example.com")
	inviteeID, newReq := env.newAdminWithVerifiedEmail(t, "invitee@example.com", false)

	invite := env.createInvite(t, inviterID, "invitee@example.com", "editor", time.Hour)

	res := httptest.NewRecorder()
	env.controller.AcceptProjectInvite(res, newReq(), invite.ID)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	admin, err := env.mgmt.GetProjectAdmin(context.Background(), env.projectID, inviteeID)
	require.NoError(t, err)
	require.NotNil(t, admin)
	assert.Equal(t, "editor", admin.Role)

	stored, err := env.mgmt.GetInviteByID(context.Background(), invite.ID)
	require.NoError(t, err)
	assert.NotNil(t, stored.AcceptedAt)
}

// The same reasoning applies to merely SEEING an invite: who invited you and to
// which project is the invited mailbox's business, not that of whoever managed
// to register the address.
// The invites addressed to an account are shown to it whether or not the
// address has been confirmed, because confirming is no longer part of the
// journey.
func TestListMyInvitesWithoutAConfirmedAddress(t *testing.T) {
	t.Parallel()

	env := newInviteTestEnv(t)
	inviterID, _ := env.newAdmin(t, "lister-inviter@example.com")
	_, newReq := env.newAdminWithVerifiedEmail(t, "lister-invitee@example.com", false)

	env.createInvite(t, inviterID, "lister-invitee@example.com", "editor", time.Hour)

	res := httptest.NewRecorder()
	env.controller.ListMyInvites(res, newReq())
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	assert.Contains(t, res.Body.String(), "lister-inviter@example.com")
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

	// Every project read is scoped to the organization the admin is currently
	// in, so an invitee who accepts from their own organization has to be moved
	// into the one that owns the project. Without this they are handed a project
	// id the very next request reports as not found.
	t.Run("activates the organization that owns the project", func(t *testing.T) {
		env := newInviteTestEnv(t)
		ctx := context.Background()
		inviterID, _ := env.newAdmin(t, "inviter@example.com")

		homeOrgID, err := env.mgmt.CreateOrganization(ctx, "Invitee's Own Org")
		require.NoError(t, err)

		adminID, err := env.mgmt.CreateAdmin(ctx, management.Admin{
			OrganizationID: homeOrgID,
			Email:          "outsider@example.com",
			Role:           "owner",
		})
		require.NoError(t, err)
		require.NoError(t, env.mgmt.AddMember(ctx, homeOrgID, adminID, rbac.OrganizationMember))

		invite := env.createInvite(t, inviterID, "outsider@example.com", oapi.CreateProjectInviteRoleEditor, time.Hour)

		actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(), rbac.WithOrganizationID(homeOrgID))
		req := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(rbac.WithActor(ctx, actor))

		rec := httptest.NewRecorder()
		env.controller.AcceptProjectInvite(rec, req, invite.ID)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		admin, err := env.mgmt.GetAdmin(ctx, adminID)
		require.NoError(t, err)
		require.NotNil(t, admin.ActiveOrganizationID)
		assert.Equal(t, env.orgID, *admin.ActiveOrganizationID,
			"accepting has to leave the invitee in the organization owning the project")
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

// A membership that is removed and then invited back has to end up holding the
// role of the NEW invite and nothing else. The soft-deleted row reads as absent,
// so the accept flow sees a fresh membership and has no old role to revoke —
// leaving the previous grant in place would make Postgres and authorization
// disagree, and the check resolves the tuple, not the row.
func TestAcceptProjectInviteReplacesAStaleRoleGrant(t *testing.T) {
	t.Parallel()

	env := newInviteTestEnv(t)
	ctx := context.Background()
	inviterID, _ := env.newAdmin(t, "inviter@example.com")
	adminID, newReq := env.newAdmin(t, "returning@example.com")

	// A first membership as editor, then removed the way the roster does it.
	require.NoError(t, env.mgmt.AddAdminToProject(ctx, env.projectID, adminID, rbac.ProjectEditor))
	require.NoError(t, access.ProvisionProjectRole(ctx, env.engine, adminID, env.projectID, rbac.ProjectEditor))
	require.NoError(t, env.mgmt.DeleteProjectAdmin(ctx, env.projectID, adminID))

	invite := env.createInvite(t, inviterID, "returning@example.com", oapi.CreateProjectInviteRoleSupport, time.Hour)

	rec := httptest.NewRecorder()
	env.controller.AcceptProjectInvite(rec, newReq(), invite.ID)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	member, err := env.mgmt.GetProjectAdmin(ctx, env.projectID, adminID)
	require.NoError(t, err)
	require.Equal(t, rbac.ProjectSupport, member.Role)

	// The editor grant let them write; the support role they now hold must not.
	allowed, err := env.engine.Check(ctx, "user:"+adminID.String(), "create",
		rbac.ProjectResourceScope("campaigns", env.projectID))
	require.NoError(t, err)
	assert.False(t, allowed, "the revoked editor grant must not survive the new invite")
}
