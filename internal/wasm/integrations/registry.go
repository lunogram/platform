package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"

	"go.uber.org/zap"

	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/wasm"
	"github.com/lunogram/platform/pkg/modules"
	actiontypes "github.com/lunogram/platform/pkg/modules/actions"
	providertypes "github.com/lunogram/platform/pkg/modules/providers"
)

// Registry is an integration-specific registry that wraps the generic WASM module registry.
type Registry struct {
	*wasm.Registry[modules.IntegrationManifest]
}

// Register adds an integration module or adapts provider/action compatibility modules.
func (r *Registry) Register(module any) error {
	switch m := module.(type) {
	case *wasm.Module[modules.IntegrationManifest]:
		return r.Registry.Register(m)
	case *wasm.Module[providertypes.ProviderManifest]:
		adapted, err := adaptCompatProviderModule(m, r.Config())
		if err != nil {
			return err
		}
		return r.Registry.Register(adapted)
	case *wasm.Module[actiontypes.ActionManifest]:
		adapted, err := adaptCompatActionModule(m, r.Config())
		if err != nil {
			return err
		}
		return r.Registry.Register(adapted)
	default:
		return fmt.Errorf("unsupported module type %T", module)
	}
}

// NewRegistry creates a new integration registry.
func NewRegistry(cfg config.WASM, logger *zap.Logger) *Registry {
	return &Registry{Registry: wasm.NewRegistry[modules.IntegrationManifest](cfg, logger)}
}

// LoadFromFS loads integration modules and provider/action compatibility modules from a filesystem.
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
			mod, err = r.loadCompatProvider(ctx, data)
		}
		if err != nil {
			mod, err = r.loadCompatAction(ctx, data)
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

func (r *Registry) loadCompatProvider(ctx context.Context, wasmBytes []byte) (*wasm.Module[modules.IntegrationManifest], error) {
	providerModule, err := wasm.LoadModule[providertypes.ProviderManifest](ctx, wasmBytes, r.Config(), r.Logger())
	if err != nil {
		return nil, err
	}

	adapted, err := adaptCompatProviderModule(providerModule, r.Config())
	if err != nil {
		providerModule.Close(ctx)
		return nil, err
	}

	return adapted, nil
}

func adaptCompatProviderModule(providerModule *wasm.Module[providertypes.ProviderManifest], cfg config.WASM) (*wasm.Module[modules.IntegrationManifest], error) {
	manifest := providerModule.Manifest()
	if manifest.Metadata.ID == "" {
		return nil, fmt.Errorf("provider manifest missing metadata.id")
	}
	if len(manifest.Spec.Channels) == 0 {
		return nil, fmt.Errorf("provider manifest has no channels")
	}

	capSpec, err := json.Marshal(modules.ProviderSpec{
		Channels:  toUnifiedChannels(manifest.Spec.Channels),
		Platforms: toUnifiedPlatforms(manifest.Spec.Platforms),
		Webhook:   manifest.Spec.Webhook,
		Locked:    manifest.Spec.Locked,
		RateLimit: toUnifiedRateLimit(manifest.Spec.RateLimit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal provider capability spec: %w", err)
	}

	integrationManifest := modules.IntegrationManifest{
		APIVersion: "v1",
		Metadata:   manifest.Metadata,
		Version:    manifest.Version,
		License:    manifest.License,
		Author:     manifest.Author,
		Website:    manifest.Website,
		Config:     manifest.Spec.Config,
		Capabilities: []modules.Capability{
			{Type: "provider", Version: "v1", Spec: capSpec},
		},
	}

	return wasm.NewModule(integrationManifest, providerModule.Plugin(), cfg), nil
}

func (r *Registry) loadCompatAction(ctx context.Context, wasmBytes []byte) (*wasm.Module[modules.IntegrationManifest], error) {
	actionModule, err := wasm.LoadModule[actiontypes.ActionManifest](ctx, wasmBytes, r.Config(), r.Logger())
	if err != nil {
		return nil, err
	}

	adapted, err := adaptCompatActionModule(actionModule, r.Config())
	if err != nil {
		actionModule.Close(ctx)
		return nil, err
	}

	return adapted, nil
}

func adaptCompatActionModule(actionModule *wasm.Module[actiontypes.ActionManifest], cfg config.WASM) (*wasm.Module[modules.IntegrationManifest], error) {

	manifest := actionModule.Manifest()
	if manifest.Metadata.ID == "" {
		return nil, fmt.Errorf("action manifest missing metadata.id")
	}
	if len(manifest.Functions) == 0 {
		return nil, fmt.Errorf("action manifest has no functions")
	}

	functions := make([]modules.ActionFunction, len(manifest.Functions))
	for i, fn := range manifest.Functions {
		functions[i] = modules.ActionFunction{
			ID:          fn.ID,
			Title:       fn.Title,
			Description: fn.Description,
			Input:       fn.Input,
		}
	}

	capSpec, err := json.Marshal(modules.ActionsSpec{Functions: functions})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal actions capability spec: %w", err)
	}

	integrationManifest := modules.IntegrationManifest{
		APIVersion: "v1",
		Metadata:   manifest.Metadata,
		Version:    manifest.Version,
		License:    manifest.License,
		Author:     manifest.Author,
		Config:     manifest.Config,
		Capabilities: []modules.Capability{
			{Type: "actions", Version: "v1", Spec: capSpec},
		},
	}

	return wasm.NewModule(integrationManifest, actionModule.Plugin(), cfg), nil
}

func toUnifiedChannels(channels []providertypes.Channel) []modules.Channel {
	result := make([]modules.Channel, len(channels))
	for i, ch := range channels {
		result[i] = modules.Channel(ch)
	}
	return result
}

func toUnifiedPlatforms(platforms []providertypes.Platform) []modules.Platform {
	result := make([]modules.Platform, len(platforms))
	for i, p := range platforms {
		result[i] = modules.Platform(p)
	}
	return result
}

func toUnifiedRateLimit(limit *providertypes.RateLimit) *modules.RateLimit {
	if limit == nil {
		return nil
	}

	return &modules.RateLimit{
		Limit:    limit.Limit,
		Interval: limit.Interval,
		Override: limit.Override,
	}
}
