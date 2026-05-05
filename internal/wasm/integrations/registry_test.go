package integrations

import (
	"encoding/json"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/wasm"
	wasmtest "github.com/lunogram/platform/internal/wasm/test"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestRegistryLoadIntegrationProviderAndAction(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	reg := NewRegistry(config.WASM{CallTimeout: 30 * time.Second}, logger)

	fsys := fstest.MapFS{
		"modules/provider.wasm": &fstest.MapFile{Data: wasmtest.ProviderWASM},
		"modules/action.wasm":   &fstest.MapFile{Data: wasmtest.ActionWASM},
	}

	require.NoError(t, reg.LoadFromFS(t.Context(), fsys, "modules"))

	providerIntegration, ok := reg.Get("testprovider")
	require.True(t, ok)
	providerSpec, ok := providerIntegration.ProviderSpec()
	require.True(t, ok)
	assert.NotEmpty(t, providerSpec.Channels)

	_, hasActions := providerIntegration.ActionsSpec()
	assert.False(t, hasActions)

	actionIntegration, ok := reg.Get("test")
	require.True(t, ok)
	actionsSpec, ok := actionIntegration.ActionsSpec()
	require.True(t, ok)
	require.Len(t, actionsSpec.Functions, 1)
	assert.Equal(t, "run", actionsSpec.Functions[0].ID)

	_, hasProvider := actionIntegration.ProviderSpec()
	assert.False(t, hasProvider)
}

func TestRegistryRegisterProviderModule(t *testing.T) {
	logger := zaptest.NewLogger(t)
	reg := NewRegistry(config.WASM{CallTimeout: 30 * time.Second}, logger)

	providerSpec, err := json.Marshal(modules.ProviderSpec{
		Channels: []modules.Channel{modules.ChannelEmail},
	})
	require.NoError(t, err)

	providerModule := wasm.NewTestModule(modules.IntegrationManifest{
		APIVersion: "v1",
		Metadata:   modules.Metadata{ID: "provider-module", Title: "Provider Module"},
		Version:    "1.0.0",
		Capabilities: []modules.Capability{
			{Type: "provider", Version: "v1", Spec: providerSpec},
		},
	}, config.WASM{})

	require.NoError(t, reg.Register(providerModule))

	integration, ok := reg.Get("provider-module")
	require.True(t, ok)
	_, hasProvider := integration.ProviderSpec()
	assert.True(t, hasProvider)
}

func TestRegistryRegisterActionModule(t *testing.T) {
	logger := zaptest.NewLogger(t)
	reg := NewRegistry(config.WASM{CallTimeout: 30 * time.Second}, logger)

	actionsSpec, err := json.Marshal(modules.ActionsSpec{
		Functions: []modules.ActionFunction{
			{ID: "run", Title: "Run", Description: "Run action"},
		},
	})
	require.NoError(t, err)

	actionModule := wasm.NewTestModule(modules.IntegrationManifest{
		APIVersion: "v1",
		Metadata:   modules.Metadata{ID: "action-module", Title: "Action Module"},
		Version:    "1.0.0",
		Capabilities: []modules.Capability{
			{Type: "actions", Version: "v1", Spec: actionsSpec},
		},
	}, config.WASM{})

	require.NoError(t, reg.Register(actionModule))

	integration, ok := reg.Get("action-module")
	require.True(t, ok)
	_, hasActions := integration.ActionsSpec()
	assert.True(t, hasActions)
}
