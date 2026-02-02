package wasm

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"

	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/lunogram/platform/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewRegistry(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := config.WASM{CallTimeout: 30 * time.Second}
	registry := NewRegistry[providers.ProviderManifest](cfg, logger)

	assert.NotNil(t, registry)
	assert.Empty(t, registry.List())
}

func TestRegistryRegister(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := config.WASM{CallTimeout: 30 * time.Second}

	t.Run("valid module", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		registry := NewRegistry[providers.ProviderManifest](cfg, logger)

		module := loadTestProviderModule(t)
		defer module.Close(t.Context())

		err := registry.Register(module)
		require.NoError(t, err)
		assert.Len(t, registry.List(), 1)
	})

	t.Run("duplicate module", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		registry := NewRegistry[providers.ProviderManifest](cfg, logger)

		module1 := loadTestProviderModule(t)
		defer module1.Close(t.Context())

		module2 := loadTestProviderModule(t)
		defer module2.Close(t.Context())

		err := registry.Register(module1)
		require.NoError(t, err)

		err = registry.Register(module2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})
}

func TestRegistryGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	cfg := config.WASM{CallTimeout: 30 * time.Second}
	registry := NewRegistry[providers.ProviderManifest](cfg, logger)

	module := loadTestProviderModule(t)
	defer module.Close(t.Context())

	err := registry.Register(module)
	require.NoError(t, err)

	type test struct {
		id        string
		wantFound bool
	}

	tests := map[string]test{
		"existing module": {
			id:        "testprovider",
			wantFound: true,
		},
		"non-existent module": {
			id:        "does-not-exist",
			wantFound: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			m, found := registry.Get(test.id)

			assert.Equal(t, test.wantFound, found)
			if test.wantFound {
				assert.NotNil(t, m)
				assert.Equal(t, test.id, m.Manifest().Metadata.ID)
			} else {
				assert.Nil(t, m)
			}
		})
	}
}

func TestRegistryList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	cfg := config.WASM{CallTimeout: 30 * time.Second}
	registry := NewRegistry[providers.ProviderManifest](cfg, logger)

	assert.Empty(t, registry.List())

	module := loadTestProviderModule(t)
	defer module.Close(t.Context())

	err := registry.Register(module)
	require.NoError(t, err)

	ids := registry.List()
	assert.Len(t, ids, 1)
	assert.Contains(t, ids, "testprovider")
}

func TestRegistryAll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	cfg := config.WASM{CallTimeout: 30 * time.Second}
	registry := NewRegistry[providers.ProviderManifest](cfg, logger)

	assert.Empty(t, registry.All())

	module := loadTestProviderModule(t)
	defer module.Close(t.Context())

	err := registry.Register(module)
	require.NoError(t, err)

	modules := registry.All()
	assert.Len(t, modules, 1)
	assert.Equal(t, "testprovider", modules[0].Manifest().Metadata.ID)
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
		"single module": {
			fsys: fstest.MapFS{
				"modules/testprovider.wasm": &fstest.MapFile{Data: testProviderWASM},
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
		"skip non-wasm files": {
			fsys: fstest.MapFS{
				"modules/testprovider.wasm": &fstest.MapFile{Data: testProviderWASM},
				"modules/readme.txt":        &fstest.MapFile{Data: []byte("readme")},
				"modules/config.json":       &fstest.MapFile{Data: []byte("{}")},
			},
			dir:     "modules",
			wantErr: false,
			count:   1,
		},
		"invalid wasm file": {
			fsys: fstest.MapFS{
				"modules/invalid.wasm": &fstest.MapFile{Data: []byte("not valid wasm")},
			},
			dir:     "modules",
			wantErr: true,
			errMsg:  "failed to load module",
		},
		"non-existent directory": {
			fsys: fstest.MapFS{
				"other/file.txt": &fstest.MapFile{Data: []byte("test")},
			},
			dir:     "modules",
			wantErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)
			registry := NewRegistry[providers.ProviderManifest](cfg, logger)

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

func TestRegistryClose(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	cfg := config.WASM{CallTimeout: 30 * time.Second}
	registry := NewRegistry[providers.ProviderManifest](cfg, logger)

	module := loadTestProviderModule(t)
	err := registry.Register(module)
	require.NoError(t, err)

	assert.Len(t, registry.List(), 1)

	registry.Close(t.Context())

	assert.Empty(t, registry.List())
}

func TestRegistryCloseEmpty(t *testing.T) {
	logger := zaptest.NewLogger(t)
	cfg := config.WASM{CallTimeout: 30 * time.Second}
	registry := NewRegistry[providers.ProviderManifest](cfg, logger)

	registry.Close(t.Context())
}

func TestRegistryCloseMultiple(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	cfg := config.WASM{CallTimeout: 30 * time.Second}

	fsys := fstest.MapFS{
		"modules/testprovider.wasm": &fstest.MapFile{Data: testProviderWASM},
	}

	logger := zaptest.NewLogger(t)
	registry := NewRegistry[providers.ProviderManifest](cfg, logger)

	err := registry.LoadFromFS(t.Context(), fsys, "modules")
	require.NoError(t, err)

	assert.Len(t, registry.List(), 1)

	registry.Close(t.Context())

	assert.Empty(t, registry.List())
}

func TestRegistryConcurrentAccess(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	cfg := config.WASM{CallTimeout: 30 * time.Second}
	registry := NewRegistry[providers.ProviderManifest](cfg, logger)

	module := loadTestProviderModule(t)
	defer module.Close(t.Context())

	err := registry.Register(module)
	require.NoError(t, err)

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = registry.List()
				_ = registry.All()
				_, _ = registry.Get("testprovider")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestRegistryCloseWithCanceledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := zaptest.NewLogger(t)
	cfg := config.WASM{CallTimeout: 30 * time.Second}
	registry := NewRegistry[providers.ProviderManifest](cfg, logger)

	module := loadTestProviderModule(t)
	err := registry.Register(module)
	require.NoError(t, err)

	canceledCtx, cancel := context.WithCancel(t.Context())
	cancel()

	registry.Close(canceledCtx)

	assert.Empty(t, registry.List())
}
