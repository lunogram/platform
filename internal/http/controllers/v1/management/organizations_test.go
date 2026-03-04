package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lunogram/platform/internal/claim/rbac"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestGetTenant(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	orgs := management.NewOrganizationsStore(mgmt)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	organizations := NewOrganizationsController(logger, mgmt)

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
			req := httptest.NewRequest("GET", "/api/admin/tenant", nil)

			claimAdmin := &rbac.Scope{
				OrganizationID: admin.OrganizationID,
			}
			req = req.WithContext(rbac.WithScope(req.Context(), claimAdmin))

			organizations.GetTenant(res, req)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if res.Code == http.StatusOK {
				var org oapi.Tenant
				err := json.NewDecoder(res.Body).Decode(&org)
				require.NoError(t, err)
				require.Equal(t, orgID, org.Id)
				require.Equal(t, "Test Organization", org.Name)
			}
		})
	}
}

func TestUpdateTenant(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	orgs := management.NewOrganizationsStore(mgmt)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "admin@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	organizations := NewOrganizationsController(logger, mgmt)

	type test struct {
		body oapi.UpdateTenantJSONRequestBody
		code int
	}

	tests := map[string]test{
		"success with tracking url": {
			body: oapi.UpdateTenantJSONRequestBody{
				TrackingDeeplinkMirrorUrl: ptr("https://example.com/track"),
			},
			code: http.StatusOK,
		},
		"success with empty body": {
			body: oapi.UpdateTenantJSONRequestBody{},
			code: http.StatusOK,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("PATCH", "/api/admin/tenant", bytes.NewReader(bb))

			claimAdmin := &rbac.Scope{
				OrganizationID: admin.OrganizationID,
			}
			req = req.WithContext(rbac.WithScope(req.Context(), claimAdmin))

			organizations.UpdateTenant(res, req)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if res.Code == http.StatusOK {
				var org oapi.Tenant
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

func TestDeleteTenant(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	orgs := management.NewOrganizationsStore(mgmt)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "admin@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	organizations := NewOrganizationsController(logger, mgmt)

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
			req := httptest.NewRequest("DELETE", "/api/admin/tenant", nil)

			claimAdmin := &rbac.Scope{
				OrganizationID: admin.OrganizationID,
			}
			req = req.WithContext(rbac.WithScope(req.Context(), claimAdmin))

			organizations.DeleteTenant(res, req)

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

func TestGetTenantIntegrations(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	orgs := management.NewOrganizationsStore(mgmt)
	orgID, err := orgs.CreateOrganization(ctx, "Test Organization")
	require.NoError(t, err)

	admins := management.NewAdminsStore(mgmt)
	adminID, err := admins.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "test@example.com",
		Role:           "admin",
	})
	require.NoError(t, err)

	admin, err := admins.GetAdmin(ctx, adminID)
	require.NoError(t, err)

	// Create a project for this organization
	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "America/New_York",
		Locale:         "en-US",
	})
	require.NoError(t, err)

	// Create a provider for this project
	providers := management.NewProvidersStore(mgmt)
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

	organizations := NewOrganizationsController(logger, mgmt)

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
			req := httptest.NewRequest("GET", "/api/admin/tenant/integrations", nil)

			claimAdmin := &rbac.Scope{
				OrganizationID: admin.OrganizationID,
			}
			req = req.WithContext(rbac.WithScope(req.Context(), claimAdmin))

			organizations.GetTenantIntegrations(res, req)

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
