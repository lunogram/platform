package v1

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rbac/access"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestGetProfileWithInternalAdmin(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	orgsStore := management.NewOrganizationsStore(mgmt)
	orgID, err := orgsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	adminsStore := management.NewAdminsStore(mgmt)
	adminID, err := adminsStore.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "admin@example.com",
		Role:           "member",
	})
	require.NoError(t, err)

	type test struct {
		actor   *rbac.Actor
		orgRole string
		code    int
	}

	tests := map[string]test{
		"success with UUID subject": {
			actor: rbac.NewActor(rbac.ActorAdmin, adminID.String(),
				rbac.WithOrganizationID(orgID),
			),
			orgRole: "member",
			code:    200,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			engine, actorCtx := rbac.TestSetup(t, ctx, tt.actor, tt.orgRole, "")
			admins := NewAdminsController(logger, mgmt, engine)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/admin/profile", nil)

			req = req.WithContext(actorCtx)

			admins.GetProfile(res, req)

			require.Equal(t, tt.code, res.Code, res.Body.String())
		})
	}
}

func TestGetProfileWithExternalAdmin(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	orgsStore := management.NewOrganizationsStore(mgmt)
	orgID, err := orgsStore.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	externalID := "user_2abc123def"
	adminsStore := management.NewAdminsStore(mgmt)
	adminID, err := adminsStore.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		ExternalID:     &externalID,
		Email:          "external@example.com",
		Role:           "owner",
	})
	require.NoError(t, err)

	type test struct {
		actor   *rbac.Actor
		orgRole string
		code    int
	}

	tests := map[string]test{
		"success with external ID": {
			actor: rbac.NewActor(rbac.ActorAdmin, externalID,
				rbac.WithOrganizationID(orgID),
			),
			orgRole: "owner",
			code:    200,
		},
		"fallback to UUID when external ID not found": {
			actor: rbac.NewActor(rbac.ActorAdmin, adminID.String(),
				rbac.WithOrganizationID(orgID),
			),
			orgRole: "owner",
			code:    200,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			engine, actorCtx := rbac.TestSetup(t, ctx, tt.actor, tt.orgRole, "")
			admins := NewAdminsController(logger, mgmt, engine)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/admin/profile", nil)

			req = req.WithContext(actorCtx)

			admins.GetProfile(res, req)

			require.Equal(t, tt.code, res.Code, res.Body.String())
		})
	}
}

func TestGetProfileErrors(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	type test struct {
		setup func(t *testing.T, ctx context.Context) (*rbac.Engine, context.Context)
		code  int
	}

	tests := map[string]test{
		"no session in context": {
			setup: func(t *testing.T, ctx context.Context) (*rbac.Engine, context.Context) {
				return rbac.NewTestEngine(t), ctx
			},
			code: 401,
		},
		"empty subject": {
			setup: func(t *testing.T, ctx context.Context) (*rbac.Engine, context.Context) {
				actor := &rbac.Actor{ID: ""}
				return rbac.NewTestEngine(t), rbac.WithActor(ctx, actor)
			},
			code: 401,
		},
		"admin not found": {
			setup: func(t *testing.T, ctx context.Context) (*rbac.Engine, context.Context) {
				actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
					rbac.WithOrganizationID(uuid.New()),
				)
				return rbac.TestSetup(t, ctx, actor, "owner", "")
			},
			code: 404,
		},
		"invalid UUID format": {
			setup: func(t *testing.T, ctx context.Context) (*rbac.Engine, context.Context) {
				actor := &rbac.Actor{ID: "not-a-valid-uuid"}
				return rbac.NewTestEngine(t), rbac.WithActor(ctx, actor)
			},
			code: 401,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/admin/profile", nil)

			engine, ctx := tt.setup(t, req.Context())
			req = req.WithContext(ctx)

			admins := NewAdminsController(logger, mgmt, engine)

			admins.GetProfile(res, req)

			require.Equal(t, tt.code, res.Code, res.Body.String())
		})
	}
}

// TestSetActiveOrganizationRejectsNonMember is the IDOR guard: an admin must
// not be able to activate an organization they are not a member of, even though
// they hold a valid session.
func TestSetActiveOrganizationRejectsNonMember(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)
	state := management.NewState(mgmt)

	homeOrg, err := state.CreateOrganization(ctx, "Home Org")
	require.NoError(t, err)
	foreignOrg, err := state.CreateOrganization(ctx, "Foreign Org")
	require.NoError(t, err)

	adminID, err := state.CreateAdmin(ctx, management.Admin{
		OrganizationID: homeOrg,
		Email:          "idor@example.com",
		Role:           "owner",
	})
	require.NoError(t, err)
	require.NoError(t, state.AddMember(ctx, homeOrg, adminID, "owner"))
	// Deliberately NOT a member of foreignOrg.

	actor := rbac.NewActor(rbac.ActorAdmin, adminID.String(), rbac.WithOrganizationID(homeOrg))
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "")
	controller := NewAdminsController(logger, mgmt, engine)

	body := oapi.SetActiveOrganization{OrganizationId: foreignOrg}
	bb, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/active-organization", bytes.NewReader(bb))
	req = req.WithContext(actorCtx)

	controller.SetActiveOrganization(res, req)

	require.Equal(t, 403, res.Code, res.Body.String())

	// The active organization must be unchanged after the rejected switch.
	admin, err := state.GetAdmin(ctx, adminID)
	require.NoError(t, err)
	require.NotNil(t, admin.ActiveOrganizationID)
	require.Equal(t, homeOrg, *admin.ActiveOrganizationID)
}

// TestCreateAdminDoesNotMutateCrossOrgGlobalRecord verifies that adding an
// existing admin (whose home organization is a DIFFERENT org) to the caller's
// organization is purely a membership grant: it must NOT overwrite that admin's
// global email/name/role. Doing so would be a cross-organization privilege
// escalation.
func TestCreateAdminDoesNotMutateCrossOrgGlobalRecord(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)
	state := management.NewState(mgmt)

	orgA, err := state.CreateOrganization(ctx, "Org A")
	require.NoError(t, err)
	orgB, err := state.CreateOrganization(ctx, "Org B")
	require.NoError(t, err)

	// The caller is an owner of Org A.
	callerID, err := state.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgA,
		Email:          "owner-a@example.com",
		Role:           "owner",
	})
	require.NoError(t, err)
	require.NoError(t, state.AddMember(ctx, orgA, callerID, "owner"))

	// The victim belongs to Org B with an "owner" global role and a known name.
	victimFirst := "Victim"
	victimLast := "Original"
	victimID, err := state.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgB,
		Email:          "victim@example.com",
		FirstName:      &victimFirst,
		LastName:       &victimLast,
		Role:           "owner",
	})
	require.NoError(t, err)
	require.NoError(t, state.AddMember(ctx, orgB, victimID, "owner"))

	actor := rbac.NewActor(rbac.ActorAdmin, callerID.String(), rbac.WithOrganizationID(orgA))
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "")
	controller := NewAdminsController(logger, mgmt, engine)

	// Org A's owner adds the victim by email, requesting role "member" with an
	// attempt to rewrite their name. None of these global fields may change.
	attackFirst := "Hacked"
	attackLast := "Name"
	body := oapi.CreateAdmin{
		Email:     "victim@example.com",
		FirstName: &attackFirst,
		LastName:  &attackLast,
		Role:      oapi.OrganizationRoleMember,
	}
	bb, err := json.Marshal(body)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/admin/admins", bytes.NewReader(bb))
	req = req.WithContext(actorCtx)

	controller.CreateAdmin(res, req)
	require.Equal(t, 201, res.Code, res.Body.String())

	// The victim's GLOBAL record is untouched.
	victim, err := state.GetAdmin(ctx, victimID)
	require.NoError(t, err)
	require.Equal(t, orgB, victim.OrganizationID, "home org must not change")
	require.Equal(t, "owner", victim.Role, "global role must not be downgraded")
	require.NotNil(t, victim.FirstName)
	require.Equal(t, "Victim", *victim.FirstName, "global first name must not be rewritten")
	require.NotNil(t, victim.LastName)
	require.Equal(t, "Original", *victim.LastName, "global last name must not be rewritten")

	// But they ARE now a member of Org A with the requested org-scoped role.
	member, err := state.GetMember(ctx, orgA, victimID)
	require.NoError(t, err)
	require.Equal(t, "member", member.Role)

	// And Org B membership/role is preserved.
	bMember, err := state.GetMember(ctx, orgB, victimID)
	require.NoError(t, err)
	require.Equal(t, "owner", bMember.Role)
}

type projectAdminEnv struct {
	controller *AdminsController
	engine     *rbac.Engine
	state      *management.State
	orgID      uuid.UUID
	projectID  uuid.UUID
}

func newProjectAdminEnv(t *testing.T) projectAdminEnv {
	t.Helper()
	ctx := context.Background()

	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	state := management.NewState(mgmtDB)
	engine := rbac.NewTestEngine(t)

	orgID, err := state.CreateOrganization(ctx, "Members Org")
	require.NoError(t, err)

	projectID, err := state.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Members Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)
	require.NoError(t, access.ProvisionProject(ctx, engine, orgID, projectID))

	return projectAdminEnv{
		controller: NewAdminsController(zaptest.NewLogger(t), mgmtDB, engine),
		engine:     engine,
		state:      state,
		orgID:      orgID,
		projectID:  projectID,
	}
}

// newAdmin creates an admin, records their membership of the env organization
// and writes the matching organization role tuple.
func (env projectAdminEnv) newAdmin(t *testing.T, email, orgRole string) uuid.UUID {
	t.Helper()
	ctx := context.Background()

	adminID, err := env.state.CreateAdmin(ctx, management.Admin{
		OrganizationID: env.orgID,
		Email:          email,
		Role:           orgRole,
	})
	require.NoError(t, err)
	require.NoError(t, env.state.AddMember(ctx, env.orgID, adminID, orgRole))
	require.NoError(t, env.engine.WriteTuples(ctx, access.OrganizationRoleTuples(adminID, env.orgID, orgRole)))

	return adminID
}

// grantProjectRole records a project_admins row and its RBAC tuple, the pair
// that a correctly provisioned project member always has.
func (env projectAdminEnv) grantProjectRole(t *testing.T, adminID uuid.UUID, role string) {
	t.Helper()
	ctx := context.Background()

	require.NoError(t, env.state.AddAdminToProject(ctx, env.projectID, adminID, role))
	require.NoError(t, access.ProvisionProjectRole(ctx, env.engine, adminID, env.projectID, role))
}

func (env projectAdminEnv) request(method string, adminID uuid.UUID, body any) *http.Request {
	return env.requestAs(method, rbac.NewActor(rbac.ActorAdmin, adminID.String(), rbac.WithOrganizationID(env.orgID)), body)
}

func (env projectAdminEnv) requestAs(method string, actor *rbac.Actor, body any) *http.Request {
	var reader io.Reader = http.NoBody
	if body != nil {
		bb, err := json.Marshal(body)
		if err != nil {
			panic(err)
		}
		reader = bytes.NewReader(bb)
	}
	req := httptest.NewRequest(method, "/", reader)
	return req.WithContext(rbac.WithActor(context.Background(), actor))
}

func (env projectAdminEnv) canRead(t *testing.T, adminID uuid.UUID) bool {
	t.Helper()
	allowed, err := env.engine.Check(context.Background(), "user:"+adminID.String(), "read",
		rbac.ProjectResourceScope("users", env.projectID))
	require.NoError(t, err)
	return allowed
}

func (env projectAdminEnv) canDeleteJourneys(t *testing.T, adminID uuid.UUID) bool {
	t.Helper()
	allowed, err := env.engine.Check(context.Background(), "user:"+adminID.String(), "delete",
		rbac.ProjectResourceScope("journeys", env.projectID))
	require.NoError(t, err)
	return allowed
}

// TestProjectAdminHandlersDenyOutsiders is the core regression: before the
// members resource existed these four handlers ran no permission check at all,
// so any authenticated admin holding a project uuid could read the roster and
// re-role or remove its members.
func TestProjectAdminHandlersDenyOutsiders(t *testing.T) {
	t.Parallel()

	env := newProjectAdminEnv(t)
	member := env.newAdmin(t, "member@example.com", rbac.OrganizationMember)
	env.grantProjectRole(t, member, rbac.ProjectAdmin)

	// An admin of another organization entirely, with no relation to the project.
	outsiderOrg, err := env.state.CreateOrganization(context.Background(), "Outsider Org")
	require.NoError(t, err)
	outsider, err := env.state.CreateAdmin(context.Background(), management.Admin{
		OrganizationID: outsiderOrg,
		Email:          "outsider@example.com",
		Role:           rbac.OrganizationOwner,
	})
	require.NoError(t, err)
	require.NoError(t, env.state.AddMember(context.Background(), outsiderOrg, outsider, rbac.OrganizationOwner))
	require.NoError(t, env.engine.WriteTuples(context.Background(),
		access.OrganizationRoleTuples(outsider, outsiderOrg, rbac.OrganizationOwner)))

	// A member of the right organization who has no role in the project. They
	// pass the organization-level checks that guard the neighbouring handlers.
	bystander := env.newAdmin(t, "bystander@example.com", rbac.OrganizationMember)

	for name, actor := range map[string]uuid.UUID{
		"foreign organization owner": outsider,
		"organization member":        bystander,
	} {
		t.Run(name, func(t *testing.T) {
			limit := oapi.Limit(10)
			res := httptest.NewRecorder()
			env.controller.ListProjectAdmins(res, env.request(http.MethodGet, actor, nil), env.projectID,
				oapi.ListProjectAdminsParams{Limit: &limit})
			require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())

			res = httptest.NewRecorder()
			env.controller.GetProjectAdmin(res, env.request(http.MethodGet, actor, nil), env.projectID, member)
			require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())

			res = httptest.NewRecorder()
			env.controller.UpdateProjectAdmin(res,
				env.request(http.MethodPatch, actor, oapi.UpdateProjectAdmin{Role: oapi.ProjectRoleSupport}),
				env.projectID, member)
			require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())

			res = httptest.NewRecorder()
			env.controller.DeleteProjectAdmin(res, env.request(http.MethodDelete, actor, nil), env.projectID, member)
			require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())

			// The roster must be untouched by any of the rejected calls.
			pa, err := env.state.GetProjectAdmin(context.Background(), env.projectID, member)
			require.NoError(t, err)
			require.Equal(t, rbac.ProjectAdmin, pa.Role)
		})
	}
}

// TestProjectAdminReadsAllowSupport pins the chosen permission level: reading
// the roster is open to everyone on the project, mutating it is not.
func TestProjectAdminReadsAllowSupport(t *testing.T) {
	t.Parallel()

	env := newProjectAdminEnv(t)
	owner := env.newAdmin(t, "owner@example.com", rbac.OrganizationOwner)
	env.grantProjectRole(t, owner, rbac.ProjectAdmin)
	viewer := env.newAdmin(t, "viewer@example.com", rbac.OrganizationMember)
	env.grantProjectRole(t, viewer, rbac.ProjectSupport)

	limit := oapi.Limit(10)
	res := httptest.NewRecorder()
	env.controller.ListProjectAdmins(res, env.request(http.MethodGet, viewer, nil), env.projectID,
		oapi.ListProjectAdminsParams{Limit: &limit})
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	var list oapi.ProjectAdminList
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &list))
	require.Equal(t, 2, list.Total)

	res = httptest.NewRecorder()
	env.controller.GetProjectAdmin(res, env.request(http.MethodGet, viewer, nil), env.projectID, owner)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	res = httptest.NewRecorder()
	env.controller.GetProjectAdmin(res, env.request(http.MethodGet, viewer, nil), env.projectID, uuid.New())
	require.Equal(t, http.StatusNotFound, res.Code, res.Body.String())

	// A support member may look, not touch.
	res = httptest.NewRecorder()
	env.controller.UpdateProjectAdmin(res,
		env.request(http.MethodPatch, viewer, oapi.UpdateProjectAdmin{Role: oapi.ProjectRoleAdmin}),
		env.projectID, viewer)
	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
}

// TestUpdateProjectAdminAppliesRoleToRBAC is the promotion/demotion defect: the
// role used to be written to Postgres only, so it changed the label in the UI
// and nothing about the member's actual permissions.
func TestUpdateProjectAdminAppliesRoleToRBAC(t *testing.T) {
	t.Parallel()

	env := newProjectAdminEnv(t)
	owner := env.newAdmin(t, "owner@example.com", rbac.OrganizationOwner)
	env.grantProjectRole(t, owner, rbac.ProjectAdmin)
	target := env.newAdmin(t, "target@example.com", rbac.OrganizationMember)
	env.grantProjectRole(t, target, rbac.ProjectSupport)

	require.True(t, env.canRead(t, target))
	require.False(t, env.canDeleteJourneys(t, target), "support must not be able to delete journeys")

	res := httptest.NewRecorder()
	env.controller.UpdateProjectAdmin(res,
		env.request(http.MethodPatch, owner, oapi.UpdateProjectAdmin{Role: oapi.ProjectRoleEditor}),
		env.projectID, target)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	pa, err := env.state.GetProjectAdmin(context.Background(), env.projectID, target)
	require.NoError(t, err)
	require.Equal(t, rbac.ProjectEditor, pa.Role)
	require.True(t, env.canDeleteJourneys(t, target), "promotion must take effect in OpenFGA")

	// And the demotion back must actually withdraw the privilege.
	res = httptest.NewRecorder()
	env.controller.UpdateProjectAdmin(res,
		env.request(http.MethodPatch, owner, oapi.UpdateProjectAdmin{Role: oapi.ProjectRoleSupport}),
		env.projectID, target)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	require.False(t, env.canDeleteJourneys(t, target), "demotion must take effect in OpenFGA")
	require.True(t, env.canRead(t, target))
}

// TestDeleteProjectAdminRevokesAccess is the most serious of the lifecycle
// defects: removing a member used to delete the project_admins row only, so the
// removed person vanished from the roster while keeping full API access.
func TestDeleteProjectAdminRevokesAccess(t *testing.T) {
	t.Parallel()

	env := newProjectAdminEnv(t)
	owner := env.newAdmin(t, "owner@example.com", rbac.OrganizationOwner)
	env.grantProjectRole(t, owner, rbac.ProjectAdmin)
	target := env.newAdmin(t, "target@example.com", rbac.OrganizationMember)
	env.grantProjectRole(t, target, rbac.ProjectEditor)

	require.True(t, env.canRead(t, target))

	res := httptest.NewRecorder()
	env.controller.DeleteProjectAdmin(res, env.request(http.MethodDelete, owner, nil), env.projectID, target)
	require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())

	_, err := env.state.GetProjectAdmin(context.Background(), env.projectID, target)
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.False(t, env.canRead(t, target), "a removed member must lose project access")
}

// TestProjectAdminLockoutGuard covers both halves of the guard: the change is
// refused when it would strip the project of its last administrator, and
// allowed when somebody else can still administer it.
func TestProjectAdminLockoutGuard(t *testing.T) {
	t.Parallel()

	t.Run("refuses to remove the last administrator", func(t *testing.T) {
		env := newProjectAdminEnv(t)
		// The only member of the organization is a plain member holding project
		// admin explicitly, so nobody inherits project admin.
		sole := env.newAdmin(t, "sole@example.com", rbac.OrganizationMember)
		env.grantProjectRole(t, sole, rbac.ProjectAdmin)

		res := httptest.NewRecorder()
		env.controller.DeleteProjectAdmin(res, env.request(http.MethodDelete, sole, nil), env.projectID, sole)
		require.Equal(t, http.StatusConflict, res.Code, res.Body.String())

		res = httptest.NewRecorder()
		env.controller.UpdateProjectAdmin(res,
			env.request(http.MethodPatch, sole, oapi.UpdateProjectAdmin{Role: oapi.ProjectRoleSupport}),
			env.projectID, sole)
		require.Equal(t, http.StatusConflict, res.Code, res.Body.String())

		pa, err := env.state.GetProjectAdmin(context.Background(), env.projectID, sole)
		require.NoError(t, err)
		require.Equal(t, rbac.ProjectAdmin, pa.Role)
	})

	t.Run("allows removal while an organization owner can still inherit", func(t *testing.T) {
		env := newProjectAdminEnv(t)
		owner := env.newAdmin(t, "owner@example.com", rbac.OrganizationOwner)
		departing := env.newAdmin(t, "departing@example.com", rbac.OrganizationMember)
		env.grantProjectRole(t, departing, rbac.ProjectAdmin)

		res := httptest.NewRecorder()
		env.controller.DeleteProjectAdmin(res, env.request(http.MethodDelete, owner, nil), env.projectID, departing)
		require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())
		require.False(t, env.canRead(t, departing))
	})
}

// TestUpdateProjectAdminRejectsUnknownRole is the fail-closed guard: a role the
// hierarchy cannot rank would compare as lower than everything and would be
// written to OpenFGA as an undefined relation.
func TestUpdateProjectAdminRejectsUnknownRole(t *testing.T) {
	t.Parallel()

	env := newProjectAdminEnv(t)
	owner := env.newAdmin(t, "owner@example.com", rbac.OrganizationOwner)
	env.grantProjectRole(t, owner, rbac.ProjectAdmin)
	target := env.newAdmin(t, "target@example.com", rbac.OrganizationMember)
	env.grantProjectRole(t, target, rbac.ProjectSupport)

	res := httptest.NewRecorder()
	env.controller.UpdateProjectAdmin(res,
		env.request(http.MethodPatch, owner, oapi.UpdateProjectAdmin{Role: oapi.ProjectRole("superuser")}),
		env.projectID, target)
	require.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())

	pa, err := env.state.GetProjectAdmin(context.Background(), env.projectID, target)
	require.NoError(t, err)
	require.Equal(t, rbac.ProjectSupport, pa.Role)
}

// TestDeleteAdminRevokesProjectAccess covers the organization-removal leak:
// project access resolves from the direct project role tuple, so dropping the
// organization membership alone left the removed person with working access to
// every project of that organization.
func TestDeleteAdminRevokesProjectAccess(t *testing.T) {
	t.Parallel()

	env := newProjectAdminEnv(t)
	owner := env.newAdmin(t, "owner@example.com", rbac.OrganizationOwner)
	target := env.newAdmin(t, "target@example.com", rbac.OrganizationMember)
	env.grantProjectRole(t, target, rbac.ProjectEditor)

	require.True(t, env.canRead(t, target))

	res := httptest.NewRecorder()
	env.controller.DeleteAdmin(res, env.request(http.MethodDelete, owner, nil), target)
	require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())

	require.False(t, env.canRead(t, target), "a removed org member must lose project access")

	// They must also disappear from the roster, not linger as a ghost member.
	_, err := env.state.GetProjectAdmin(context.Background(), env.projectID, target)
	require.ErrorIs(t, err, sql.ErrNoRows)
}

// TestDeleteAdminIsReplayableAfterInterruptedRevoke: the tuples come out before
// the transaction, so an interruption leaves the rows behind rather than the
// access. A replay reads those rows and finishes.
func TestDeleteAdminIsReplayableAfterInterruptedRevoke(t *testing.T) {
	t.Parallel()

	env := newProjectAdminEnv(t)
	owner := env.newAdmin(t, "owner@example.com", rbac.OrganizationOwner)
	target := env.newAdmin(t, "target@example.com", rbac.OrganizationMember)
	env.grantProjectRole(t, target, rbac.ProjectEditor)

	// The state a crash between the revoke and the commit leaves behind.
	require.NoError(t, access.DeprovisionProjectRole(context.Background(), env.engine, target, env.projectID, rbac.ProjectEditor))
	require.NoError(t, access.DeprovisionOrganizationRole(context.Background(), env.engine, target, env.orgID, rbac.OrganizationMember))
	require.False(t, env.canRead(t, target))

	// The membership rows are still there, so the replay has something to read.
	member, err := env.state.GetMember(context.Background(), env.orgID, target)
	require.NoError(t, err)
	require.Equal(t, rbac.OrganizationMember, member.Role)

	res := httptest.NewRecorder()
	env.controller.DeleteAdmin(res, env.request(http.MethodDelete, owner, nil), target)
	require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())

	_, err = env.state.GetMember(context.Background(), env.orgID, target)
	require.ErrorIs(t, err, sql.ErrNoRows)
	_, err = env.state.GetProjectAdmin(context.Background(), env.projectID, target)
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.False(t, env.canRead(t, target))
}

// TestProjectAdminRevocationIsReplayable pins the revocation ordering. The
// tuple is removed before the row, so an interruption between the two leaves
// access already gone and a roster entry that a replay can still read and
// finish. The reverse order is unrecoverable: the row a repair needs is the row
// the failed attempt destroyed.
func TestProjectAdminRevocationIsReplayable(t *testing.T) {
	t.Parallel()

	t.Run("interrupted removal is completed by a replay", func(t *testing.T) {
		env := newProjectAdminEnv(t)
		owner := env.newAdmin(t, "owner@example.com", rbac.OrganizationOwner)
		target := env.newAdmin(t, "target@example.com", rbac.OrganizationMember)
		env.grantProjectRole(t, target, rbac.ProjectEditor)

		// The state a crash after the tuple revoke leaves behind: access gone,
		// roster entry still present.
		require.NoError(t, access.DeprovisionProjectRole(context.Background(), env.engine, target, env.projectID, rbac.ProjectEditor))
		require.False(t, env.canRead(t, target))

		res := httptest.NewRecorder()
		env.controller.DeleteProjectAdmin(res, env.request(http.MethodDelete, owner, nil), env.projectID, target)
		require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())

		_, err := env.state.GetProjectAdmin(context.Background(), env.projectID, target)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("interrupted demotion is completed by a replay", func(t *testing.T) {
		env := newProjectAdminEnv(t)
		owner := env.newAdmin(t, "owner@example.com", rbac.OrganizationOwner)
		target := env.newAdmin(t, "target@example.com", rbac.OrganizationMember)
		env.grantProjectRole(t, target, rbac.ProjectAdmin)

		// A crash after the old tuple was revoked but before the row was
		// rewritten: the row still says admin, the member holds nothing.
		require.NoError(t, access.DeprovisionProjectRole(context.Background(), env.engine, target, env.projectID, rbac.ProjectAdmin))
		require.False(t, env.canRead(t, target))

		res := httptest.NewRecorder()
		env.controller.UpdateProjectAdmin(res,
			env.request(http.MethodPatch, owner, oapi.UpdateProjectAdmin{Role: oapi.ProjectRoleSupport}),
			env.projectID, target)
		require.Equal(t, http.StatusOK, res.Code, res.Body.String())

		pa, err := env.state.GetProjectAdmin(context.Background(), env.projectID, target)
		require.NoError(t, err)
		require.Equal(t, rbac.ProjectSupport, pa.Role)

		// Converged: the demotion took effect in both stores, with no admin
		// tuple surviving.
		require.True(t, env.canRead(t, target))
		require.False(t, env.canDeleteJourneys(t, target))
		allowed, err := env.engine.Check(context.Background(), "user:"+target.String(), rbac.ProjectAdmin, rbac.ProjectScope(env.projectID))
		require.NoError(t, err)
		require.False(t, allowed, "the stale admin tuple must not survive a replayed demotion")
	})
}

// TestUpdateProjectAdminCeilingForPolicyActor: an access policy can hold
// members:update as a direct grant without being a project admin, which is
// exactly the actor the least-privilege ceiling exists for. It must be
// evaluated, not refused outright.
func TestUpdateProjectAdminCeilingForPolicyActor(t *testing.T) {
	t.Parallel()

	env := newProjectAdminEnv(t)
	target := env.newAdmin(t, "target@example.com", rbac.OrganizationMember)
	env.grantProjectRole(t, target, rbac.ProjectSupport)
	// A second explicit admin so the lockout guard never colours these results.
	keeper := env.newAdmin(t, "keeper@example.com", rbac.OrganizationMember)
	env.grantProjectRole(t, keeper, rbac.ProjectAdmin)

	// A policy with a custom permission set that includes members:update, plus
	// the editor project role.
	editorPolicy := uuid.New()
	require.NoError(t, access.ProvisionPolicyGrants(context.Background(), env.engine, editorPolicy, env.projectID,
		[]access.Grant{{Resource: "members", Verb: rbac.Update}}))
	require.NoError(t, access.ProvisionProjectRole(context.Background(), env.engine, editorPolicy, env.projectID, rbac.ProjectEditor))

	editorActor := rbac.NewActor(rbac.ActorAPIKey, editorPolicy.String(),
		rbac.WithOrganizationID(env.orgID), rbac.WithProjectID(env.projectID))

	// Within its own rank: allowed.
	res := httptest.NewRecorder()
	env.controller.UpdateProjectAdmin(res,
		env.requestAs(http.MethodPatch, editorActor, oapi.UpdateProjectAdmin{Role: oapi.ProjectRoleEditor}),
		env.projectID, target)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	// Above its own rank: refused by the ceiling, not by the resource check.
	res = httptest.NewRecorder()
	env.controller.UpdateProjectAdmin(res,
		env.requestAs(http.MethodPatch, editorActor, oapi.UpdateProjectAdmin{Role: oapi.ProjectRoleAdmin}),
		env.projectID, target)
	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	require.Contains(t, res.Body.String(), "equal to or lower than your own role")

	pa, err := env.state.GetProjectAdmin(context.Background(), env.projectID, target)
	require.NoError(t, err)
	require.Equal(t, rbac.ProjectEditor, pa.Role, "the refused escalation must not have been written")

	// A policy holding only the custom grant carries no rank at all, so it can
	// assign nothing.
	barePolicy := uuid.New()
	require.NoError(t, access.ProvisionPolicyGrants(context.Background(), env.engine, barePolicy, env.projectID,
		[]access.Grant{{Resource: "members", Verb: rbac.Update}}))
	bareActor := rbac.NewActor(rbac.ActorAPIKey, barePolicy.String(),
		rbac.WithOrganizationID(env.orgID), rbac.WithProjectID(env.projectID))

	res = httptest.NewRecorder()
	env.controller.UpdateProjectAdmin(res,
		env.requestAs(http.MethodPatch, bareActor, oapi.UpdateProjectAdmin{Role: oapi.ProjectRoleSupport}),
		env.projectID, target)
	require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
}
