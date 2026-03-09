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

	admins := management.NewAdminsStore(mgmt)

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

	admins := management.NewAdminsStore(mgmt)
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

	admins := management.NewAdminsStore(mgmt)
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

	admins := management.NewAdminsStore(mgmt)
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
