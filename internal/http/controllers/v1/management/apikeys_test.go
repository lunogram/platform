package v1

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCreateApiKey(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	controller := NewApiKeysController(logger, mgmt)

	type test struct {
		body oapi.CreateApiKeyJSONRequestBody
		code int
	}

	roleSupport := oapi.ProjectRoleSupport
	roleAdmin := oapi.ProjectRoleAdmin

	tests := map[string]test{
		"simple with default role": {
			body: oapi.CreateApiKeyJSONRequestBody{
				Name:  "Test API Key",
				Scope: oapi.Secret,
			},
			code: 201,
		},
		"with support role": {
			body: oapi.CreateApiKeyJSONRequestBody{
				Name:  "Support API Key",
				Scope: oapi.Secret,
				Role:  &roleSupport,
			},
			code: 201,
		},
		"with admin role": {
			body: oapi.CreateApiKeyJSONRequestBody{
				Name:  "Admin API Key",
				Scope: oapi.Secret,
				Role:  &roleAdmin,
			},
			code: 201,
		},
		"with description": {
			body: oapi.CreateApiKeyJSONRequestBody{
				Name:        "Described API Key",
				Scope:       oapi.Secret,
				Description: ptr("This is a test API key"),
			},
			code: 201,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/apikeys", bytes.NewReader(bb))
			controller.CreateApiKey(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 201 {
				var result oapi.ApiKey
				err = json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, test.body.Name, result.Name)
				require.Equal(t, projectID, result.ProjectId)
				require.NotEqual(t, uuid.Nil, result.Id)
			}
		})
	}
}

func TestListApiKeys(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	apiKeysStore := management.NewApiKeysStore(mgmt)

	for i := 0; i < 3; i++ {
		_, err := apiKeysStore.CreateApiKey(ctx, projectID, "Test Key", "project", "support", nil)
		require.NoError(t, err)
	}

	controller := NewApiKeysController(logger, mgmt)

	type test struct {
		params oapi.ListApiKeysParams
		code   int
	}

	tests := map[string]test{
		"default pagination": {
			params: oapi.ListApiKeysParams{
				Limit:  ptr(oapi.Limit(20)),
				Offset: ptr(oapi.Offset(0)),
			},
			code: 200,
		},
		"custom pagination": {
			params: oapi.ListApiKeysParams{
				Limit:  ptr(oapi.Limit(2)),
				Offset: ptr(oapi.Offset(1)),
			},
			code: 200,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/apikeys", nil)
			controller.ListApiKeys(res, req, projectID, test.params)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var result oapi.ApiKeyListResponse
				err = json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, 3, result.Total)
			}
		})
	}
}

func TestGetApiKey(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	apiKeysStore := management.NewApiKeysStore(mgmt)
	apiKey, err := apiKeysStore.CreateApiKey(ctx, projectID, "Test Key", "project", "support", nil)
	require.NoError(t, err)

	controller := NewApiKeysController(logger, mgmt)

	type test struct {
		keyID uuid.UUID
		code  int
	}

	tests := map[string]test{
		"success": {
			keyID: apiKey.ID,
			code:  200,
		},
		"not found": {
			keyID: uuid.New(),
			code:  404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/apikeys/"+test.keyID.String(), nil)
			controller.GetApiKey(res, req, projectID, test.keyID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var result oapi.ApiKey
				err = json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, apiKey.ID, result.Id)
				require.Equal(t, "Test Key", result.Name)
			}
		})
	}
}

func TestUpdateApiKey(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	apiKeysStore := management.NewApiKeysStore(mgmt)
	controller := NewApiKeysController(logger, mgmt)

	type test struct {
		body oapi.UpdateApiKeyJSONRequestBody
		code int
	}

	roleAdmin := oapi.ProjectRoleAdmin

	tests := map[string]test{
		"update name": {
			body: oapi.UpdateApiKeyJSONRequestBody{
				Name: ptr("Updated Key"),
			},
			code: 200,
		},
		"update role": {
			body: oapi.UpdateApiKeyJSONRequestBody{
				Role: &roleAdmin,
			},
			code: 200,
		},
		"update description": {
			body: oapi.UpdateApiKeyJSONRequestBody{
				Description: ptr("Updated description"),
			},
			code: 200,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Create a fresh API key for each test case
			apiKey, err := apiKeysStore.CreateApiKey(ctx, projectID, "Test Key", "project", "support", nil)
			require.NoError(t, err)

			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("PATCH", "/v1/apikeys/"+apiKey.ID.String(), bytes.NewReader(bb))
			controller.UpdateApiKey(res, req, projectID, apiKey.ID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var result oapi.ApiKey
				err = json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, apiKey.ID, result.Id)
			}
		})
	}
}

func TestDeleteApiKey(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := t.Context()
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	apiKeysStore := management.NewApiKeysStore(mgmt)
	apiKey, err := apiKeysStore.CreateApiKey(ctx, projectID, "Test Key", "project", "support", nil)
	require.NoError(t, err)

	controller := NewApiKeysController(logger, mgmt)

	type test struct {
		code int
	}

	tests := map[string]test{
		"success": {
			code: 204,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("DELETE", "/v1/apikeys/"+apiKey.ID.String(), nil)
			controller.DeleteApiKey(res, req, projectID, apiKey.ID)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}
