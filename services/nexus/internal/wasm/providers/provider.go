package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/lunogram/platform/pkg/modules/providers"
	"github.com/lunogram/platform/services/nexus/internal/wasm"
)

// Provider wraps a WASM module with provider-specific functionality.
type Provider struct {
	*wasm.Module[providers.ProviderManifest]
}

// Send invokes the provider's send function.
func (p *Provider) Send(ctx context.Context, req *providers.SendRequest[map[string]any]) (*providers.SendResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal send request: %w", err)
	}

	code, res, err := p.Call(ctx, "send", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to call provider send: %w", err)
	}

	if code != 0 {
		return nil, fmt.Errorf("provider send returned code %d: %s", code, string(res))
	}

	var response providers.SendResponse
	if err := json.Unmarshal(res, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal send response: %w", err)
	}

	return &response, nil
}

// SupportsChannel checks if this provider supports a specific channel.
func (p *Provider) SupportsChannel(ch providers.Channel) bool {
	for _, c := range p.Manifest().Spec.Channels {
		if c == ch {
			return true
		}
	}
	return false
}

// Registry is a provider-specific registry that wraps the generic WASM module registry.
type Registry struct {
	*wasm.Registry[providers.ProviderManifest]
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		Registry: wasm.NewRegistry[providers.ProviderManifest](),
	}
}

// LoadFromFS loads all provider modules from an embedded filesystem.
func (r *Registry) LoadFromFS(ctx context.Context, fsys fs.FS, dir string) error {
	if err := r.Registry.LoadFromFS(ctx, fsys, dir); err != nil {
		return err
	}

	// Validate that all loaded modules have at least one channel
	for _, module := range r.Registry.All() {
		if len(module.Manifest().Spec.Channels) == 0 {
			return fmt.Errorf("provider %s must support at least one channel", module.Manifest().Metadata.ID)
		}
	}

	return nil
}

// Get retrieves a provider by ID.
func (r *Registry) Get(id string) (*Provider, bool) {
	module, exists := r.Registry.Get(id)
	if !exists {
		return nil, false
	}
	return &Provider{Module: module}, true
}

// All returns all registered providers.
func (r *Registry) All() []*Provider {
	modules := r.Registry.All()
	result := make([]*Provider, len(modules))
	for i, module := range modules {
		result[i] = &Provider{Module: module}
	}
	return result
}

// SupportsChannel checks if a module supports a specific channel.
func (r *Registry) SupportsChannel(moduleID string, channel providers.Channel) bool {
	provider, exists := r.Get(moduleID)
	if !exists {
		return false
	}
	return provider.SupportsChannel(channel)
}
