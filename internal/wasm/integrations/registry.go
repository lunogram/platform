package integrations

import (
	"context"
	"fmt"
	"io/fs"

	"go.uber.org/zap"

	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/wasm"
	"github.com/lunogram/platform/pkg/modules"
)

// Registry is an integration-specific registry that wraps the generic WASM module registry.
type Registry struct {
	*wasm.Registry[modules.IntegrationManifest]
}

// Register adds an integration module.
func (r *Registry) Register(module any) error {
	switch m := module.(type) {
	case *wasm.Module[modules.IntegrationManifest]:
		return r.Registry.Register(m)
	default:
		return fmt.Errorf("unsupported module type %T", module)
	}
}

// NewRegistry creates a new integration registry.
func NewRegistry(cfg config.WASM, logger *zap.Logger) *Registry {
	return &Registry{Registry: wasm.NewRegistry[modules.IntegrationManifest](cfg, logger)}
}

// LoadFromFS loads integration modules from a filesystem.
func (r *Registry) LoadFromFS(ctx context.Context, fsys fs.FS, dir string) error {
	return fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		if len(path) < len(".wasm") || path[len(path)-len(".wasm"):] != ".wasm" {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("failed to read module %s: %w", path, err)
		}

		mod, err := wasm.LoadModule[modules.IntegrationManifest](ctx, data, r.Config(), r.Logger())
		if err == nil && len(mod.Manifest().Capabilities) == 0 {
			mod.Close(ctx)
			mod = nil
			err = fmt.Errorf("module has no integration capabilities")
		}
		if err != nil {
			return fmt.Errorf("failed to load module %s: %w", path, err)
		}

		if err := r.Register(mod); err != nil {
			mod.Close(ctx)
			return err
		}

		return nil
	})
}

// Get retrieves an integration by ID.
func (r *Registry) Get(id string) (*Integration, bool) {
	module, exists := r.Registry.Get(id)
	if !exists {
		return nil, false
	}

	return &Integration{Module: module}, true
}

// All returns all integrations.
func (r *Registry) All() []*Integration {
	mods := r.Registry.All()
	result := make([]*Integration, len(mods))
	for i, module := range mods {
		result[i] = &Integration{Module: module}
	}

	return result
}
