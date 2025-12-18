package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/claim/rbac"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCreateProject(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, config.Store)
	require.NoError(t, err)

	orgs := store.NewOrganizationsStore(db)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := store.NewAdminsStore(db)
	adminID, err := admins.CreateAdmin(ctx, store.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	projects := NewProjectsController(logger, db)

	type test struct {
		body oapi.CreateProjectJSONRequestBody
		code int
	}

	tests := map[string]test{
		"success": {
			body: oapi.CreateProjectJSONRequestBody{
				Name:     "Test Project",
				Timezone: "America/New_York",
				Locale:   "en-US",
			},
			code: http.StatusCreated,
		},
		"with description": {
			body: oapi.CreateProjectJSONRequestBody{
				Name:        "Test Project",
				Description: ptr("A test project"),
				Timezone:    "America/New_York",
				Locale:      "en-US",
			},
			code: http.StatusCreated,
		},
		"with tools": {
			body: oapi.CreateProjectJSONRequestBody{
				Name:     "Test Project",
				Timezone: "America/New_York",
				Locale:   "en-US",
				Tools:    &[]string{"analytics", "reporting"},
			},
			code: http.StatusCreated,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/api/admin/projects", bytes.NewReader(bb))

			claimAdmin := &rbac.Scope{
				OrganizationID: admin.OrganizationID,
			}
			req = req.WithContext(rbac.WithScope(req.Context(), claimAdmin))

			projects.CreateProject(res, req)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if res.Code == http.StatusCreated {
				var project oapi.Project
				err = json.NewDecoder(res.Body).Decode(&project)
				require.NoError(t, err)
				require.Equal(t, test.body.Name, project.Name)
				require.Equal(t, test.body.Timezone, project.Timezone)
				require.Equal(t, test.body.Locale, project.Locale)
				require.NotEqual(t, uuid.Nil, project.Id)
			}
		})
	}
}

func TestListProjects(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, config.Store)
	require.NoError(t, err)

	orgs := store.NewOrganizationsStore(db)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := store.NewAdminsStore(db)
	adminID, err := admins.CreateAdmin(ctx, store.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	projectStore := store.NewProjectsStore(db)
	for i := 0; i < 3; i++ {
		projectID, err := projectStore.CreateProject(ctx, store.Project{
			OrganizationID: &orgID,
			Name:           "Test Project",
			Timezone:       "UTC",
			Locale:         "en-US",
		})
		require.NoError(t, err)

		err = projectStore.AddProjectAdmin(ctx, projectID, adminID, "admin")
		require.NoError(t, err)
	}

	projects := NewProjectsController(logger, db)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects", nil)

	claimAdmin := &rbac.Scope{
		OrganizationID: admin.OrganizationID,
	}
	req = req.WithContext(rbac.WithScope(req.Context(), claimAdmin))

	limit := oapi.PaginationLimit(10)
	offset := oapi.PaginationOffset(0)
	params := oapi.ListProjectsParams{
		Limit:  &limit,
		Offset: &offset,
	}

	projects.ListProjects(res, req, params)

	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	var result oapi.ProjectList
	err = json.NewDecoder(res.Body).Decode(&result)
	require.NoError(t, err)
	require.Equal(t, 3, result.Total)
	require.Len(t, result.Results, 3)
}

func TestGetProject(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, config.Store)
	require.NoError(t, err)

	orgs := store.NewOrganizationsStore(db)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := store.NewAdminsStore(db)
	adminID, err := admins.CreateAdmin(ctx, store.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	projectStore := store.NewProjectsStore(db)
	projectID, err := projectStore.CreateProject(ctx, store.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	err = projectStore.AddProjectAdmin(ctx, projectID, adminID, "admin")
	require.NoError(t, err)

	projects := NewProjectsController(logger, db)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/admin/projects/"+projectID.String(), nil)

	claimAdmin := &rbac.Scope{
		OrganizationID: admin.OrganizationID,
	}
	req = req.WithContext(rbac.WithScope(req.Context(), claimAdmin))

	projects.GetProject(res, req, projectID)

	require.Equal(t, http.StatusOK, res.Code, res.Body.String())

	var project oapi.Project
	err = json.NewDecoder(res.Body).Decode(&project)
	require.NoError(t, err)
	require.Equal(t, projectID, project.Id)
	require.Equal(t, "Test Project", project.Name)
}

func TestUpdateProject(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := config.Node{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, config.Store)
	require.NoError(t, err)

	orgs := store.NewOrganizationsStore(db)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := store.NewAdminsStore(db)
	adminID, err := admins.CreateAdmin(ctx, store.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	projectStore := store.NewProjectsStore(db)
	projectID, err := projectStore.CreateProject(ctx, store.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	err = projectStore.AddProjectAdmin(ctx, projectID, adminID, "admin")
	require.NoError(t, err)

	projects := NewProjectsController(logger, db)

	type test struct {
		body oapi.UpdateProjectJSONRequestBody
		code int
	}

	tests := map[string]test{
		"update name": {
			body: oapi.UpdateProjectJSONRequestBody{
				Name: ptr("Updated Project"),
			},
			code: http.StatusOK,
		},
		"update timezone": {
			body: oapi.UpdateProjectJSONRequestBody{
				Timezone: ptr("America/Los_Angeles"),
			},
			code: http.StatusOK,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("PATCH", "/api/admin/projects/"+projectID.String(), bytes.NewReader(bb))

			claimAdmin := &rbac.Scope{
				OrganizationID: admin.OrganizationID,
			}
			req = req.WithContext(rbac.WithScope(req.Context(), claimAdmin))

			projects.UpdateProject(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if res.Code == http.StatusOK {
				var project oapi.Project
				err = json.NewDecoder(res.Body).Decode(&project)
				require.NoError(t, err)
				require.Equal(t, projectID, project.Id)

				if test.body.Name != nil {
					require.Equal(t, *test.body.Name, project.Name)
				}
				if test.body.Timezone != nil {
					require.Equal(t, *test.body.Timezone, project.Timezone)
				}
			}
		})
	}
}
