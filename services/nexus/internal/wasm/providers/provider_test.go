package providers

import (
	"context"
	"encoding/json"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/wasm/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

var testProviderWasm = test.ProviderWASM

func TestNewRegistry(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := config.WASM{CallTimeout: 30 * time.Second}
	registry := NewRegistry(cfg, logger)

	assert.NotNil(t, registry)
	assert.Empty(t, registry.List())
}

func TestRegistryLoadFromFS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := config.WASM{CallTimeout: 30 * time.Second}

	type test struct {
		fsys    fs.FS
		dir     string
		wantErr bool
		errMsg  string
		count   int
	}

	tests := map[string]test{
		"valid provider": {
			fsys: fstest.MapFS{
				"modules/testprovider.wasm": &fstest.MapFile{Data: testProviderWasm},
			},
			dir:     "modules",
			wantErr: false,
			count:   1,
		},
		"empty directory": {
			fsys: fstest.MapFS{
				"modules/.gitkeep": &fstest.MapFile{Data: []byte{}},
			},
			dir:     "modules",
			wantErr: false,
			count:   0,
		},
		"invalid wasm": {
			fsys: fstest.MapFS{
				"modules/bad.wasm": &fstest.MapFile{Data: []byte("invalid")},
			},
			dir:     "modules",
			wantErr: true,
			errMsg:  "failed to load module",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			registry := NewRegistry(cfg, logger)

			err := registry.LoadFromFS(t.Context(), test.fsys, test.dir)

			if test.wantErr {
				require.Error(t, err)
				if test.errMsg != "" {
					assert.Contains(t, err.Error(), test.errMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.Len(t, registry.List(), test.count)
		})
	}
}

func TestRegistryGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	registry := loadTestRegistry(t)
	defer registry.Close(t.Context())

	type test struct {
		id        string
		wantFound bool
	}

	tests := map[string]test{
		"existing provider": {
			id:        "testprovider",
			wantFound: true,
		},
		"non-existent provider": {
			id:        "does-not-exist",
			wantFound: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			provider, found := registry.Get(test.id)

			assert.Equal(t, test.wantFound, found)
			if test.wantFound {
				assert.NotNil(t, provider)
			} else {
				assert.Nil(t, provider)
			}
		})
	}
}

func TestRegistryAll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	registry := loadTestRegistry(t)
	defer registry.Close(t.Context())

	all := registry.All()
	assert.Len(t, all, 1)
	assert.Equal(t, "testprovider", all[0].Manifest().Metadata.ID)
}

func TestRegistrySupportsChannel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	registry := loadTestRegistry(t)
	defer registry.Close(t.Context())

	type test struct {
		moduleID string
		channel  providers.Channel
		expected bool
	}

	tests := map[string]test{
		"testprovider supports email": {
			moduleID: "testprovider",
			channel:  providers.ChannelEmail,
			expected: true,
		},
		"testprovider supports sms": {
			moduleID: "testprovider",
			channel:  providers.ChannelSMS,
			expected: true,
		},
		"testprovider supports push": {
			moduleID: "testprovider",
			channel:  providers.ChannelPush,
			expected: true,
		},
		"non-existent module": {
			moduleID: "does-not-exist",
			channel:  providers.ChannelEmail,
			expected: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := registry.SupportsChannel(test.moduleID, test.channel)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestProviderManifest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	registry := loadTestRegistry(t)
	defer registry.Close(t.Context())

	provider, found := registry.Get("testprovider")
	require.True(t, found)

	manifest := provider.Manifest()

	assert.Equal(t, "testprovider", manifest.Metadata.ID)
	assert.Equal(t, "Test Provider", manifest.Metadata.Title)
	assert.Equal(t, "1.0.0", manifest.Version)
	assert.Equal(t, "MIT", manifest.License)
}

func TestProviderSupportsChannel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	registry := loadTestRegistry(t)
	defer registry.Close(t.Context())

	provider, found := registry.Get("testprovider")
	require.True(t, found)

	assert.True(t, provider.SupportsChannel(providers.ChannelEmail))
	assert.True(t, provider.SupportsChannel(providers.ChannelSMS))
	assert.True(t, provider.SupportsChannel(providers.ChannelPush))
	assert.False(t, provider.SupportsChannel(providers.Channel("unknown")))
}

func TestProviderSend(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	registry := loadTestRegistry(t)
	defer registry.Close(t.Context())

	provider, found := registry.Get("testprovider")
	require.True(t, found)

	emailPayload, _ := json.Marshal(providers.EmailPayload{
		To:      "test@example.com",
		From:    providers.EmailAddress{Name: "Test", Address: "from@example.com"},
		Subject: "Test Subject",
		Text:    "Test body",
		HTML:    "<p>Test body</p>",
	})

	smsPayload, _ := json.Marshal(providers.SMSPayload{
		To:   "+1234567890",
		From: "+0987654321",
		Body: "Test SMS",
	})

	pushPayload, _ := json.Marshal(providers.PushPayload{
		Tokens: []string{"token1", "token2"},
		Title:  "Test Push",
		Body:   "Test push body",
	})

	type test struct {
		req     *providers.SendRequest[map[string]any]
		wantErr bool
		errMsg  string
	}

	tests := map[string]test{
		"email": {
			req: &providers.SendRequest[map[string]any]{
				Channel: providers.ChannelEmail,
				Config:  map[string]any{},
				Payload: emailPayload,
			},
			wantErr: false,
		},
		"sms": {
			req: &providers.SendRequest[map[string]any]{
				Channel: providers.ChannelSMS,
				Config:  map[string]any{},
				Payload: smsPayload,
			},
			wantErr: false,
		},
		"push": {
			req: &providers.SendRequest[map[string]any]{
				Channel: providers.ChannelPush,
				Config:  map[string]any{},
				Payload: pushPayload,
			},
			wantErr: false,
		},
		"unsupported channel": {
			req: &providers.SendRequest[map[string]any]{
				Channel: providers.Channel("unknown"),
				Config:  map[string]any{},
				Payload: []byte("{}"),
			},
			wantErr: true,
			errMsg:  "unsupported channel",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			resp, err := provider.Send(t.Context(), test.req)

			if test.wantErr {
				require.Error(t, err)
				if test.errMsg != "" {
					assert.Contains(t, err.Error(), test.errMsg)
				}
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, "sent", resp.Status)
		})
	}
}

func TestProviderSendInvalidPayload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	registry := loadTestRegistry(t)
	defer registry.Close(t.Context())

	provider, found := registry.Get("testprovider")
	require.True(t, found)

	req := &providers.SendRequest[map[string]any]{
		Channel: providers.ChannelEmail,
		Config:  map[string]any{},
		Payload: []byte("invalid json"),
	}

	_, err := provider.Send(t.Context(), req)
	assert.Error(t, err)
}

func TestRegistryClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	registry := loadTestRegistry(t)

	assert.Len(t, registry.List(), 1)

	registry.Close(t.Context())

	assert.Empty(t, registry.List())
}

func TestRegistryCloseWithCanceledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	registry := loadTestRegistry(t)

	canceledCtx, cancel := context.WithCancel(t.Context())
	cancel()

	registry.Close(canceledCtx)

	assert.Empty(t, registry.List())
}

func loadTestRegistry(t *testing.T) *Registry {
	t.Helper()

	logger := zaptest.NewLogger(t)
	cfg := config.WASM{CallTimeout: 30 * time.Second}

	fsys := fstest.MapFS{
		"modules/testprovider.wasm": &fstest.MapFile{Data: testProviderWasm},
	}

	registry := NewRegistry(cfg, logger)

	err := registry.LoadFromFS(t.Context(), fsys, "modules")
	require.NoError(t, err)

	return registry
}
