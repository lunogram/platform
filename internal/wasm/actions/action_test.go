package actions

import (
	"encoding/json"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lunogram/platform/internal/config"
	wasmtest "github.com/lunogram/platform/internal/wasm/test"
	actiontypes "github.com/lunogram/platform/pkg/modules/actions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestRegistryLoadActionModule(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	reg := NewRegistry(config.WASM{CallTimeout: 30 * time.Second}, logger)

	fsys := fstest.MapFS{
		"modules/action.wasm": &fstest.MapFile{Data: wasmtest.ActionWASM},
	}

	require.NoError(t, reg.LoadFromFS(t.Context(), fsys, "modules"))

	action, ok := reg.Get("test")
	require.True(t, ok)
	require.NotNil(t, action)

	manifest := action.Manifest()
	assert.Equal(t, "test", manifest.Metadata.ID)
	require.Len(t, manifest.Functions, 1)
	assert.Equal(t, "run", manifest.Functions[0].ID)
}

func TestActionExecuteAndPreview(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	reg := NewRegistry(config.WASM{CallTimeout: 30 * time.Second}, logger)

	fsys := fstest.MapFS{
		"modules/action.wasm": &fstest.MapFile{Data: wasmtest.ActionWASM},
	}

	require.NoError(t, reg.LoadFromFS(t.Context(), fsys, "modules"))

	action, ok := reg.Get("test")
	require.True(t, ok)

	execReq := &actiontypes.ExecuteRequest[json.RawMessage]{
		Config: json.RawMessage(`{"api_key":"abc"}`),
		Input:  json.RawMessage(`{"message":"hello"}`),
	}

	resp, err := action.Execute(t.Context(), "run", execReq)
	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "test", resp.Metadata["action"])

	preview, err := action.Preview(t.Context())
	require.NoError(t, err)
	assert.Contains(t, string(preview), "test")
}
