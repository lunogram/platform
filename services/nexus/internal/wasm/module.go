package wasm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	extism "github.com/extism/go-sdk"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/services/nexus/internal/config"
)

// Module represents a loaded WASM module with its manifest.
type Module[T modules.Manifest] struct {
	manifest T
	plugin   *extism.Plugin
	config   config.WASM
}

// Manifest returns the module's manifest.
func (m *Module[T]) Manifest() T {
	return m.manifest
}

// Call invokes a function on the WASM plugin.
// Applies the configured CallTimeout to the context.
func (m *Module[T]) Call(ctx context.Context, fn string, input []byte) (uint32, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, m.config.CallTimeout)
	defer cancel()

	return m.plugin.CallWithContext(ctx, fn, input)
}

// Close closes the underlying plugin.
// Uses a separate context with timeout if the provided context is already canceled.
func (m *Module[T]) Close(ctx context.Context) {
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
	}

	m.plugin.Close(ctx)
}

// NewPlugin creates a new Extism plugin from WASM bytes.
func NewPlugin(ctx context.Context, wasm []byte) (*extism.Plugin, error) {
	manifest := extism.Manifest{
		AllowedHosts: []string{"*"},
		Wasm: []extism.Wasm{
			extism.WasmData{Name: "plugin", Data: wasm},
		},
	}

	cfg := extism.PluginConfig{
		EnableWasi: true,
	}

	plugin, err := extism.NewPlugin(ctx, manifest, cfg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize plugin: %w", err)
	}

	return plugin, nil
}

// LoadModule loads a WASM module and extracts its manifest.
func LoadModule[T modules.Manifest](ctx context.Context, data []byte, cfg config.WASM) (*Module[T], error) {
	plugin, err := NewPlugin(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin: %w", err)
	}

	code, res, err := plugin.CallWithContext(ctx, "manifest", nil)
	if err != nil {
		plugin.Close(ctx)
		return nil, fmt.Errorf("failed to call manifest: %w", err)
	}

	if code != 0 {
		plugin.Close(ctx)
		return nil, fmt.Errorf("manifest returned non-zero exit code: %d", code)
	}

	var manifest T
	if err := json.Unmarshal(res, &manifest); err != nil {
		plugin.Close(ctx)
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	return &Module[T]{manifest: manifest, plugin: plugin, config: cfg}, nil
}
