package wasm

import (
	"context"
	"testing"
	"time"

	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadModule(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	type test struct {
		data    []byte
		cfg     config.WASM
		wantErr bool
		errMsg  string
	}

	defaultCfg := config.WASM{CallTimeout: 30 * time.Second}

	tests := map[string]test{
		"valid module": {
			data:    testProviderWASM,
			cfg:     defaultCfg,
			wantErr: false,
		},
		"invalid wasm bytes": {
			data:    []byte("not valid wasm"),
			cfg:     defaultCfg,
			wantErr: true,
			errMsg:  "failed to create plugin",
		},
		"empty wasm bytes": {
			data:    []byte{},
			cfg:     defaultCfg,
			wantErr: true,
			errMsg:  "failed to create plugin",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			module, err := LoadModule[providers.ProviderManifest](t.Context(), test.data, test.cfg)

			if test.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.errMsg)
				return
			}

			require.NoError(t, err)
			defer module.Close(t.Context())

			manifest := module.Manifest()
			assert.NotEmpty(t, manifest.Metadata.ID)
		})
	}
}

func TestModuleManifest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	module := loadTestProviderModule(t)
	defer module.Close(t.Context())

	manifest := module.Manifest()

	assert.Equal(t, "testprovider", manifest.Metadata.ID)
	assert.Equal(t, "Test Provider", manifest.Metadata.Title)
	assert.Equal(t, "1.0.0", manifest.Version)
	assert.Contains(t, manifest.Spec.Channels, providers.ChannelEmail)
	assert.Contains(t, manifest.Spec.Channels, providers.ChannelSMS)
	assert.Contains(t, manifest.Spec.Channels, providers.ChannelPush)
}

func TestModuleCall(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	module := loadTestProviderModule(t)
	defer module.Close(t.Context())

	type test struct {
		fn      string
		input   []byte
		wantErr bool
	}

	tests := map[string]test{
		"manifest function": {
			fn:      "manifest",
			input:   nil,
			wantErr: false,
		},
		"non-existent function": {
			fn:      "does_not_exist",
			input:   nil,
			wantErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			code, output, err := module.Call(t.Context(), test.fn, test.input)

			if test.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, uint32(0), code)
			assert.NotEmpty(t, output)
		})
	}
}

func TestModuleCallWithTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := config.WASM{
		CallTimeout: 5 * time.Second,
	}

	module, err := LoadModule[providers.ProviderManifest](t.Context(), testProviderWASM, cfg)
	require.NoError(t, err)
	defer module.Close(t.Context())

	code, output, err := module.Call(t.Context(), "manifest", nil)
	require.NoError(t, err)
	assert.Equal(t, uint32(0), code)
	assert.NotEmpty(t, output)
}

func TestModuleCallWithContextDeadline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	module := loadTestProviderModule(t)
	defer module.Close(t.Context())

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	code, output, err := module.Call(ctx, "manifest", nil)
	require.NoError(t, err)
	assert.Equal(t, uint32(0), code)
	assert.NotEmpty(t, output)
}

func TestModuleClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	module := loadTestProviderModule(t)

	module.Close(t.Context())

	_, _, err := module.Call(t.Context(), "manifest", nil)
	assert.Error(t, err)
}

func TestModuleCloseWithCanceledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	module := loadTestProviderModule(t)

	canceledCtx, cancel := context.WithCancel(t.Context())
	cancel()

	module.Close(canceledCtx)

	_, _, err := module.Call(t.Context(), "manifest", nil)
	assert.Error(t, err)
}

func TestNewPlugin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	type test struct {
		data    []byte
		wantErr bool
	}

	tests := map[string]test{
		"valid wasm": {
			data:    testProviderWASM,
			wantErr: false,
		},
		"invalid wasm": {
			data:    []byte("invalid"),
			wantErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			plugin, err := NewPlugin(t.Context(), test.data)

			if test.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			defer plugin.Close(t.Context())
			assert.NotNil(t, plugin)
		})
	}
}

func loadTestProviderModule(t *testing.T) *Module[providers.ProviderManifest] {
	t.Helper()

	cfg := config.WASM{CallTimeout: 30 * time.Second}
	module, err := LoadModule[providers.ProviderManifest](t.Context(), testProviderWASM, cfg)
	require.NoError(t, err)

	return module
}
