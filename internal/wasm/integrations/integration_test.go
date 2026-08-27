package integrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/lunogram/platform/internal/config"
	wasmtest "github.com/lunogram/platform/internal/wasm/test"
	providertypes "github.com/lunogram/platform/pkg/modules/providers"
)

func TestNewProviderErrorPrefersGuestErrorOverOutput(t *testing.T) {
	t.Parallel()

	provErr := newProviderError(exitCodePermanent, []byte(`{"status":"stale"}`), errors.New("throttled"))

	assert.Equal(t, "throttled", provErr.Message)
	assert.True(t, provErr.IsPermanent())
}

func TestNewProviderErrorFallsBackToOutputWhenNoGuestError(t *testing.T) {
	t.Parallel()

	provErr := newProviderError(1, []byte("raw module output"), nil)

	assert.Equal(t, "raw module output", provErr.Message)
	assert.False(t, provErr.IsPermanent())
	assert.NoError(t, provErr.Unwrap())
}

func TestProviderErrorUnwrapsCallError(t *testing.T) {
	t.Parallel()

	t.Run("context deadline stays matchable", func(t *testing.T) {
		callErr := fmt.Errorf("call provider_send: %w", context.DeadlineExceeded)

		provErr := newProviderError(1, nil, callErr)

		assert.ErrorIs(t, provErr, context.DeadlineExceeded)
		assert.Equal(t, callErr.Error(), provErr.Message)
	})

	t.Run("sentinel wazero trap stays matchable", func(t *testing.T) {
		trap := errors.New("wasm error: unreachable")
		callErr := fmt.Errorf("module trapped: %w", trap)

		provErr := newProviderError(1, nil, callErr)

		assert.ErrorIs(t, provErr, trap)
	})
}

// TestSendLegacyModuleErrorContract runs the compatibility guarantee against a
// real WASM guest: the test provider module reports failures the way every
// pre-existing module does — pdk.SetError with a plain string plus a non-zero
// exit code — which surfaces on the call's error return with an empty output
// buffer, and must still produce a permanent ProviderError.
func TestSendLegacyModuleErrorContract(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping WASM integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	reg := NewRegistry(config.WASM{CallTimeout: 30 * time.Second}, logger)

	fsys := fstest.MapFS{
		"modules/provider.wasm": &fstest.MapFile{Data: wasmtest.ProviderWASM},
	}
	require.NoError(t, reg.LoadFromFS(t.Context(), fsys, "modules"))

	integration, ok := reg.Get("testprovider")
	require.True(t, ok)

	_, err := integration.Send(t.Context(), providertypes.SendRequest[map[string]any]{
		Channel: providertypes.Channel("carrier-pigeon"),
		Config:  map[string]any{"api_key": "test"},
		Payload: json.RawMessage(`{}`),
	})
	require.Error(t, err)

	var provErr *ProviderError
	require.ErrorAs(t, err, &provErr)
	assert.True(t, provErr.IsPermanent())
	assert.Contains(t, provErr.Message, "unsupported channel: carrier-pigeon")
}
