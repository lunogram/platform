package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/claim/rbac"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestGetOrganization(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(postgresURI)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: postgresURI,
		UsersURI:      postgresURI,
		JourneyURI:    postgresURI,
	})
	require.NoError(t, err)

	orgs := management.NewOrganizationsStore(db.Management)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(db.Management)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	organizations := NewOrganizationsController(logger, db.Management)

	type test struct {
		code int
	}

	tests := map[string]test{
		"success": {
			code: http.StatusOK,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/admin/organizations", nil)

			claimAdmin := &rbac.Scope{
				OrganizationID: admin.OrganizationID,
			}
			req = req.WithContext(rbac.WithScope(req.Context(), claimAdmin))

			organizations.GetOrganization(res, req)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if res.Code == http.StatusOK {
				var org oapi.Organization
				err := json.NewDecoder(res.Body).Decode(&org)
				require.NoError(t, err)
				require.Equal(t, orgID, org.Id)
				require.Equal(t, "Test Organization", org.Name)
			}
		})
	}
}

func TestUpdateOrganization(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(postgresURI)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: postgresURI,
		UsersURI:      postgresURI,
		JourneyURI:    postgresURI,
	})
	require.NoError(t, err)

	orgs := management.NewOrganizationsStore(db.Management)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(db.Management)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "admin@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	organizations := NewOrganizationsController(logger, db.Management)

	type test struct {
		body oapi.UpdateOrganizationJSONRequestBody
		code int
	}

	tests := map[string]test{
		"success with tracking url": {
			body: oapi.UpdateOrganizationJSONRequestBody{
				TrackingDeeplinkMirrorUrl: ptr("https://example.com/track"),
			},
			code: http.StatusOK,
		},
		"success with empty body": {
			body: oapi.UpdateOrganizationJSONRequestBody{},
			code: http.StatusOK,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("PATCH", "/api/admin/organizations", bytes.NewReader(bb))

			claimAdmin := &rbac.Scope{
				OrganizationID: admin.OrganizationID,
			}
			req = req.WithContext(rbac.WithScope(req.Context(), claimAdmin))

			organizations.UpdateOrganization(res, req)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if res.Code == http.StatusOK {
				var org oapi.Organization
				err := json.NewDecoder(res.Body).Decode(&org)
				require.NoError(t, err)
				require.Equal(t, orgID, org.Id)

				if test.body.TrackingDeeplinkMirrorUrl != nil {
					require.NotNil(t, org.TrackingDeeplinkMirrorUrl)
					require.Equal(t, *test.body.TrackingDeeplinkMirrorUrl, *org.TrackingDeeplinkMirrorUrl)
				}
			}
		})
	}
}

func TestDeleteOrganization(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(postgresURI)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: postgresURI,
		UsersURI:      postgresURI,
		JourneyURI:    postgresURI,
	})
	require.NoError(t, err)

	orgs := management.NewOrganizationsStore(db.Management)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(db.Management)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "admin@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	organizations := NewOrganizationsController(logger, db.Management)

	type test struct {
		code int
	}

	tests := map[string]test{
		"success": {
			code: http.StatusNoContent,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("DELETE", "/api/admin/organizations", nil)

			claimAdmin := &rbac.Scope{
				OrganizationID: admin.OrganizationID,
			}
			req = req.WithContext(rbac.WithScope(req.Context(), claimAdmin))

			organizations.DeleteOrganization(res, req)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if res.Code == http.StatusNoContent {
				// Verify organization is soft deleted
				org, err := orgs.GetOrganization(ctx, orgID)
				require.Error(t, err) // Should return error since it's soft deleted
				require.Nil(t, org)
			}
		})
	}
}

func TestGetOrganizationIntegrations(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	postgresURI := container.RunPostgreSQL(t)

	err := management.Migrate(postgresURI)
	require.NoError(t, err)

	db, err := store.New(ctx, logger, store.Config{
		ManagementURI: postgresURI,
		UsersURI:      postgresURI,
		JourneyURI:    postgresURI,
	})
	require.NoError(t, err)

	orgs := management.NewOrganizationsStore(db.Management)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(db.Management)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	// Create a project for this organization
	projects := management.NewProjectsStore(db.Management)
	projectID, err := projects.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "America/New_York",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	// Create a provider for this project
	providers := management.NewProvidersStore(db.Management)
	providerData := []byte(`{"api_key": "test"}`)
	_, err = providers.CreateProvider(ctx, management.Provider{
		ProjectID: projectID,
		Module:    "sendgrid",
		Channel:   "email",
		Data:      providerData,
		Name:      "Test Provider",
		IsDefault: true,
	})
	require.NoError(t, err)

	organizations := NewOrganizationsController(logger, db.Management)

	type test struct {
		code int
	}

	tests := map[string]test{
		"success": {
			code: http.StatusOK,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/admin/organizations/integrations", nil)

			claimAdmin := &rbac.Scope{
				OrganizationID: admin.OrganizationID,
			}
			req = req.WithContext(rbac.WithScope(req.Context(), claimAdmin))

			organizations.GetOrganizationIntegrations(res, req)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if res.Code == http.StatusOK {
				var providers []oapi.Provider
				err := json.NewDecoder(res.Body).Decode(&providers)
				require.NoError(t, err)
				require.Len(t, providers, 1)
				require.Equal(t, "sendgrid", providers[0].Module)
				require.Equal(t, "Test Provider", providers[0].Name)
			}
		})
	}
}
