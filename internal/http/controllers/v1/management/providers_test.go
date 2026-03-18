package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	internalProviders "github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/lunogram/platform/internal/wasm"
	wasmProviders "github.com/lunogram/platform/internal/wasm/providers"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
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

	controller := NewProvidersController(logger, mgmt, registry, engine, "http://localhost:8080")

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

	controller := NewProvidersController(logger, mgmt, registry, engine, "http://localhost:8080")

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

	controller := NewProvidersController(logger, mgmt, registry, engine, "http://localhost:8080")

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

	controller := NewProvidersController(logger, mgmt, registry, engine, "http://localhost:8080")

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

	controller := NewProvidersController(logger, mgmt, registry, engine, "http://localhost:8080")

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

	controller := NewProvidersController(logger, mgmt, registry, engine, "http://localhost:8080")

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

// newTestProviderRegistry creates a provider registry with test modules for
// locked/unlocked testing without requiring real WASM binaries.
func newTestProviderRegistry(t *testing.T) *internalProviders.Registry {
	t.Helper()

	logger := zaptest.NewLogger(t)
	wasmCfg := config.WASM{CallTimeout: 30 * time.Second}

	registry := wasmProviders.NewRegistry(wasmCfg, logger)

	// Register a locked module
	lockedModule := wasm.NewTestModule(providers.ProviderManifest{
		Metadata: modules.Metadata{
			ID:    "locked-provider",
			Title: "Locked Provider",
		},
		Version: "1.0.0",
		Spec: providers.ProviderSpec{
			Channels: []providers.Channel{providers.ChannelEmail},
			Locked:   true,
		},
	}, wasmCfg)
	require.NoError(t, registry.Registry.Register(lockedModule))

	// Register a non-locked module
	unlockedModule := wasm.NewTestModule(providers.ProviderManifest{
		Metadata: modules.Metadata{
			ID:    "unlocked-provider",
			Title: "Unlocked Provider",
		},
		Version: "1.0.0",
		Spec: providers.ProviderSpec{
			Channels: []providers.Channel{providers.ChannelEmail},
			Locked:   false,
		},
	}, wasmCfg)
	require.NoError(t, registry.Registry.Register(unlockedModule))

	return registry
}

func TestDeleteLockedProvider(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	projects := management.NewProjectsStore(mgmt)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	registry := newTestProviderRegistry(t)

	actor := rbac.NewActor(rbac.ActorAdmin, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(projectID),
	)
	engine, actorCtx := rbac.TestSetup(t, ctx, actor, "owner", "admin")

	controller := NewProvidersController(logger, mgmt, registry, engine, "http://localhost:8080")

	providerStore := management.NewProvidersStore(mgmt)

	// Create a provider instance referencing the locked module
	lockedProviderID, err := providerStore.CreateProvider(ctx, management.Provider{
		ProjectID: projectID,
		Module:    "locked-provider",
		Channel:   "email",
		Name:      "My Locked Provider",
		Data:      json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	// Create a provider instance referencing the unlocked module
	unlockedProviderID, err := providerStore.CreateProvider(ctx, management.Provider{
		ProjectID: projectID,
		Module:    "unlocked-provider",
		Channel:   "email",
		Name:      "My Unlocked Provider",
		Data:      json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	t.Run("locked provider returns 403", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", "/v1/providers/"+lockedProviderID.String(), nil)
		req = req.WithContext(actorCtx)
		controller.DeleteProvider(res, req, projectID, lockedProviderID)

		require.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	})

	t.Run("unlocked provider returns 204", func(t *testing.T) {
		res := httptest.NewRecorder()
		req := httptest.NewRequest("DELETE", "/v1/providers/"+unlockedProviderID.String(), nil)
		req = req.WithContext(actorCtx)
		controller.DeleteProvider(res, req, projectID, unlockedProviderID)

		require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())
	})
}
