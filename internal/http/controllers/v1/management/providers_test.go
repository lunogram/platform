package v1

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	internalProviders "github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestListProviders(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	wasmCfg := config.WASM{
		CallTimeout: 30,
	}

	registry, err := internalProviders.NewRegistry(ctx, wasmCfg, logger)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewProvidersController(logger, mgmt, registry, engine)

	type test struct {
		params oapi.ListProvidersParams
		code   int
	}

	tests := map[string]test{
		"default pagination": {
			params: oapi.ListProvidersParams{
				Limit:  ptr(oapi.Limit(20)),
				Offset: ptr(oapi.Offset(0)),
			},
			code: 200,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/providers", nil)
			req = req.WithContext(actorCtx)
			controller.ListProviders(res, req, projectID, test.params)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var result oapi.ProviderListResponse
				err = json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
			}
		})
	}
}

func TestListAllProviders(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	wasmCfg := config.WASM{
		CallTimeout: 30,
	}

	registry, err := internalProviders.NewRegistry(ctx, wasmCfg, logger)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewProvidersController(logger, mgmt, registry, engine)

	type test struct {
		code int
	}

	tests := map[string]test{
		"success": {
			code: 200,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/providers/all", nil)
			req = req.WithContext(actorCtx)
			controller.ListAllProviders(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var result []oapi.Provider
				err = json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
			}
		})
	}
}

func TestListProviderMeta(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	wasmCfg := config.WASM{
		CallTimeout: 30,
	}

	registry, err := internalProviders.NewRegistry(ctx, wasmCfg, logger)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewProvidersController(logger, mgmt, registry, engine)

	type test struct {
		code int
	}

	tests := map[string]test{
		"success": {
			code: 200,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/providers/meta", nil)
			req = req.WithContext(actorCtx)
			controller.ListProviderMeta(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var result []oapi.ProviderMeta
				err = json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
			}
		})
	}
}

func TestGetProvider(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	wasmCfg := config.WASM{
		CallTimeout: 30,
	}

	registry, err := internalProviders.NewRegistry(ctx, wasmCfg, logger)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewProvidersController(logger, mgmt, registry, engine)

	type test struct {
		providerID uuid.UUID
		code       int
	}

	tests := map[string]test{
		"not found": {
			providerID: uuid.New(),
			code:       404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/providers/email/test/"+test.providerID.String(), nil)
			req = req.WithContext(actorCtx)
			controller.GetProvider(res, req, projectID, "email", "test", test.providerID)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestDeleteProvider(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	wasmCfg := config.WASM{
		CallTimeout: 30,
	}

	registry, err := internalProviders.NewRegistry(ctx, wasmCfg, logger)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewProvidersController(logger, mgmt, registry, engine)

	type test struct {
		providerID uuid.UUID
		code       int
	}

	tests := map[string]test{
		"not found": {
			providerID: uuid.New(),
			code:       404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("DELETE", "/v1/providers/"+test.providerID.String(), nil)
			req = req.WithContext(actorCtx)
			controller.DeleteProvider(res, req, projectID, test.providerID)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestCreateProviderWithInvalidModule(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	wasmCfg := config.WASM{
		CallTimeout: 30,
	}

	registry, err := internalProviders.NewRegistry(ctx, wasmCfg, logger)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewProvidersController(logger, mgmt, registry, engine)

	type test struct {
		body oapi.CreateProviderJSONRequestBody
		code int
	}

	tests := map[string]test{
		"invalid module": {
			body: oapi.CreateProviderJSONRequestBody{
				Name: "Test Provider",
			},
			code: 400,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/providers/email/invalid-module", bytes.NewReader(bb))
			req = req.WithContext(actorCtx)
			controller.CreateProvider(res, req, projectID, "email", "invalid-module")

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestUpdateProvider(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	wasmCfg := config.WASM{
		CallTimeout: 30,
	}

	registry, err := internalProviders.NewRegistry(ctx, wasmCfg, logger)
	require.NoError(t, err)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewProvidersController(logger, mgmt, registry, engine)

	type test struct {
		body       oapi.UpdateProviderJSONRequestBody
		providerID uuid.UUID
		code       int
	}

	tests := map[string]test{
		"not found": {
			body: oapi.UpdateProviderJSONRequestBody{
				Name: ptr("Updated Provider"),
			},
			providerID: uuid.New(),
			code:       404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("PATCH", "/v1/providers/email/test/"+test.providerID.String(), bytes.NewReader(bb))
			req = req.WithContext(actorCtx)
			controller.UpdateProvider(res, req, projectID, "email", "test", test.providerID)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}
