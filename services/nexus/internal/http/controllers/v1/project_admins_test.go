package v1

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/lunogram/platform/services/nexus/oapi"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestAddProjectAdmin(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	organizations := store.NewOrganizationsStore(db)
	orgID, err := organizations.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, store.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	controller := NewProjectAdminsController(logger, db)

	type test struct {
		body oapi.AddProjectAdminJSONRequestBody
		code int
	}

	tests := map[string]test{
		"add new admin": {
			body: oapi.AddProjectAdminJSONRequestBody{
				Email: "newadmin@example.com",
				Role:  oapi.AddProjectAdminRoleAdmin,
			},
			code: 201,
		},
		"add editor": {
			body: oapi.AddProjectAdminJSONRequestBody{
				Email: "editor@example.com",
				Role:  oapi.AddProjectAdminRoleEditor,
			},
			code: 201,
		},
		"add support": {
			body: oapi.AddProjectAdminJSONRequestBody{
				Email: "support@example.com",
				Role:  oapi.AddProjectAdminRoleSupport,
			},
			code: 201,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/project_admins", bytes.NewReader(bb))
			controller.AddProjectAdmin(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if res.Code == 201 {
				var response oapi.ProjectAdmin
				err := json.Unmarshal(res.Body.Bytes(), &response)
				require.NoError(t, err)
				require.Equal(t, projectID, response.ProjectId)
				require.Equal(t, string(test.body.Email), string(*response.Email))
				require.Equal(t, string(test.body.Role), string(response.Role))
			}
		})
	}
}

func TestListProjectAdmins(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	organizations := store.NewOrganizationsStore(db)
	orgID, err := organizations.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, store.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	admins := store.NewAdminsStore(db)
	projectAdmins := store.NewAdminsStore(db)

	for i := 0; i < 3; i++ {
		email := "admin" + string(rune('0'+i)) + "@example.com"
		admin := store.Admin{
			OrganizationID: orgID,
			Email:          email,
			Role:           "member",
		}

		adminID, err := admins.CreateAdmin(ctx, admin)
		require.NoError(t, err)

		err = projectAdmins.AddAdminToProject(ctx, projectID, adminID, "admin")
		require.NoError(t, err)
	}

	controller := NewProjectAdminsController(logger, db)

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
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	organizations := store.NewOrganizationsStore(db)
	orgID, err := organizations.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, store.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	admins := store.NewAdminsStore(db)
	admin := store.Admin{
		OrganizationID: orgID,
		Email:          "admin@example.com",
		Role:           "member",
	}

	adminID, err := admins.CreateAdmin(ctx, admin)
	require.NoError(t, err)

	projectAdmins := store.NewAdminsStore(db)
	err = projectAdmins.AddAdminToProject(ctx, projectID, adminID, "admin")
	require.NoError(t, err)

	controller := NewProjectAdminsController(logger, db)

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
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	organizations := store.NewOrganizationsStore(db)
	orgID, err := organizations.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, store.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	admins := store.NewAdminsStore(db)
	admin := store.Admin{
		OrganizationID: orgID,
		Email:          "admin@example.com",
		Role:           "member",
	}

	adminID, err := admins.CreateAdmin(ctx, admin)
	require.NoError(t, err)

	projectAdmins := store.NewAdminsStore(db)
	err = projectAdmins.AddAdminToProject(ctx, projectID, adminID, "support")
	require.NoError(t, err)

	controller := NewProjectAdminsController(logger, db)

	type test struct {
		body oapi.UpdateProjectAdminJSONRequestBody
		code int
	}

	tests := map[string]test{
		"update to admin": {
			body: oapi.UpdateProjectAdminJSONRequestBody{
				Role: oapi.UpdateProjectAdminRoleAdmin,
			},
			code: 200,
		},
		"update to editor": {
			body: oapi.UpdateProjectAdminJSONRequestBody{
				Role: oapi.UpdateProjectAdminRoleEditor,
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
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	organizations := store.NewOrganizationsStore(db)
	orgID, err := organizations.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, store.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	admins := store.NewAdminsStore(db)
	admin := store.Admin{
		OrganizationID: orgID,
		Email:          "admin@example.com",
		Role:           "member",
	}

	adminID, err := admins.CreateAdmin(ctx, admin)
	require.NoError(t, err)

	projectAdmins := store.NewAdminsStore(db)
	err = projectAdmins.AddAdminToProject(ctx, projectID, adminID, "admin")
	require.NoError(t, err)

	controller := NewProjectAdminsController(logger, db)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/v1/project_admins/"+adminID.String(), nil)
	controller.DeleteProjectAdmin(res, req, projectID, adminID)

	require.Equal(t, 204, res.Code, res.Body.String())

	_, err = projectAdmins.GetProjectAdmin(ctx, projectID, adminID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}
