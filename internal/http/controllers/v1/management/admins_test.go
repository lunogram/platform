package v1

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"

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

	adminsStore := management.NewState(mgmt)
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

func TestListProjectAdmins(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	organizations := management.NewOrganizationsStore(mgmt)
	orgID, err := organizations.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	admins := management.NewState(mgmt)

	emails := []string{"admin1@example.com", "admin2@example.com", "admin3@example.com"}
	for _, email := range emails {
		admin := management.Admin{
			OrganizationID: orgID,
			Email:          email,
			Role:           "member",
		}

		adminID, err := admins.CreateAdmin(ctx, admin)
		require.NoError(t, err)

		err = admins.AddAdminToProject(ctx, projectID, adminID, "admin")
		require.NoError(t, err)
	}

	engine := rbac.NewTestEngine(t)
	controller := NewAdminsController(logger, mgmt, engine)

	type test struct {
		limit  int
		offset int
		total  int
		result int
	}

	tests := map[string]test{
		"default": {
			limit:  10,
			offset: 0,
			total:  3,
			result: 3,
		},
		"with limit": {
			limit:  2,
			offset: 0,
			total:  3,
			result: 2,
		},
		"with offset": {
			limit:  10,
			offset: 1,
			total:  3,
			result: 2,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			limit := oapi.Limit(test.limit)
			offset := oapi.Offset(test.offset)

			params := oapi.ListProjectAdminsParams{
				Limit:  &limit,
				Offset: &offset,
			}

			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/project_admins", nil)
			actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
				rbac.WithOrganizationID(orgID),
				rbac.WithProjectID(projectID),
			)
			req = req.WithContext(rbac.WithActor(req.Context(), actor))
			controller.ListProjectAdmins(res, req, projectID, params)

			require.Equal(t, 200, res.Code, res.Body.String())

			var response oapi.ProjectAdminList
			err := json.Unmarshal(res.Body.Bytes(), &response)
			require.NoError(t, err)

			require.Equal(t, test.total, response.Total)
			require.Equal(t, test.result, len(response.Results))
			require.Equal(t, test.limit, response.Limit)
			require.Equal(t, test.offset, response.Offset)
		})
	}
}

func TestGetProjectAdmin(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	organizations := management.NewOrganizationsStore(mgmt)
	orgID, err := organizations.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	admins := management.NewState(mgmt)
	admin := management.Admin{
		OrganizationID: orgID,
		Email:          "admin@example.com",
		Role:           "member",
	}

	adminID, err := admins.CreateAdmin(ctx, admin)
	require.NoError(t, err)

	err = admins.AddAdminToProject(ctx, projectID, adminID, "admin")
	require.NoError(t, err)

	engine := rbac.NewTestEngine(t)
	controller := NewAdminsController(logger, mgmt, engine)

	type test struct {
		adminID uuid.UUID
		code    int
	}

	tests := map[string]test{
		"existing admin": {
			adminID: adminID,
			code:    200,
		},
		"non-existent admin": {
			adminID: uuid.New(),
			code:    404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/project_admins/"+test.adminID.String(), nil)
			actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
				rbac.WithOrganizationID(orgID),
				rbac.WithProjectID(projectID),
			)
			req = req.WithContext(rbac.WithActor(req.Context(), actor))
			controller.GetProjectAdmin(res, req, projectID, test.adminID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if res.Code == 200 {
				var response oapi.ProjectAdmin
				err := json.Unmarshal(res.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, projectID, response.ProjectId)
				require.Equal(t, test.adminID, response.AdminId)
			}
		})
	}
}

func TestUpdateProjectAdmin(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	organizations := management.NewOrganizationsStore(mgmt)
	orgID, err := organizations.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	admins := management.NewState(mgmt)
	admin := management.Admin{
		OrganizationID: orgID,
		Email:          "admin@example.com",
		Role:           "member",
	}

	adminID, err := admins.CreateAdmin(ctx, admin)
	require.NoError(t, err)

	err = admins.AddAdminToProject(ctx, projectID, adminID, "support")
	require.NoError(t, err)

	engine := rbac.NewTestEngine(t)
	controller := NewAdminsController(logger, mgmt, engine)

	type test struct {
		body oapi.UpdateProjectAdminJSONRequestBody
		code int
	}

	tests := map[string]test{
		"update to admin": {
			body: oapi.UpdateProjectAdminJSONRequestBody{
				Role: oapi.ProjectRoleAdmin,
			},
			code: 200,
		},
		"update to editor": {
			body: oapi.UpdateProjectAdminJSONRequestBody{
				Role: oapi.ProjectRoleEditor,
			},
			code: 200,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("PATCH", "/v1/project_admins/"+adminID.String(), bytes.NewReader(bb))
			actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
				rbac.WithOrganizationID(orgID),
				rbac.WithProjectID(projectID),
			)
			req = req.WithContext(rbac.WithActor(req.Context(), actor))
			controller.UpdateProjectAdmin(res, req, projectID, adminID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if res.Code == 200 {
				var response oapi.ProjectAdmin
				err := json.Unmarshal(res.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, string(test.body.Role), string(response.Role))
			}
		})
	}
}

func TestDeleteProjectAdmin(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	organizations := management.NewOrganizationsStore(mgmt)
	orgID, err := organizations.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	admins := management.NewState(mgmt)
	admin := management.Admin{
		OrganizationID: orgID,
		Email:          "admin@example.com",
		Role:           "member",
	}

	adminID, err := admins.CreateAdmin(ctx, admin)
	require.NoError(t, err)

	err = admins.AddAdminToProject(ctx, projectID, adminID, "admin")
	require.NoError(t, err)

	engine := rbac.NewTestEngine(t)
	controller := NewAdminsController(logger, mgmt, engine)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/project_admins/"+adminID.String(), nil)
	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	controller.DeleteProjectAdmin(res, req, projectID, adminID)

	require.Equal(t, 204, res.Code, res.Body.String())

	_, err = admins.GetProjectAdmin(ctx, projectID, adminID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}
