package integrations

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/wasm"
	wasmtest "github.com/lunogram/platform/internal/wasm/test"
	"github.com/lunogram/platform/pkg/modules"
	actiontypes "github.com/lunogram/platform/pkg/modules/actions"
	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestRegistryLoadCompatProviderAndAction(t *testing.T) {
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

func TestRegistryRegisterCompatProviderModule(t *testing.T) {
	logger := zaptest.NewLogger(t)
	reg := NewRegistry(config.WASM{CallTimeout: 30 * time.Second}, logger)

	compatProvider := wasm.NewTestModule(providers.ProviderManifest{
		Metadata: modules.Metadata{ID: "compat-provider", Title: "Compat Provider"},
		Version:  "1.0.0",
		Spec: providers.ProviderSpec{
			Channels: []providers.Channel{providers.ChannelEmail},
		},
	}, config.WASM{})

	require.NoError(t, reg.Register(compatProvider))

	integration, ok := reg.Get("compat-provider")
	require.True(t, ok)
	_, hasProvider := integration.ProviderSpec()
	assert.True(t, hasProvider)
}

func TestRegistryRegisterCompatActionModule(t *testing.T) {
	logger := zaptest.NewLogger(t)
	reg := NewRegistry(config.WASM{CallTimeout: 30 * time.Second}, logger)

	compatAction := wasm.NewTestModule(actiontypes.ActionManifest{
		Metadata: modules.Metadata{ID: "compat-action", Title: "Compat Action"},
		Version:  "1.0.0",
		Functions: []actiontypes.ActionFunction{
			{ID: "run", Title: "Run", Description: "Run action"},
		},
	}, config.WASM{})

	require.NoError(t, reg.Register(compatAction))

	integration, ok := reg.Get("compat-action")
	require.True(t, ok)
	_, hasActions := integration.ActionsSpec()
	assert.True(t, hasActions)
}
