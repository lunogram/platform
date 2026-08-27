// Package providers provides provider-oriented wrappers over unified WASM integrations.
package providers

import (
	"context"
	"fmt"
	"io/fs"
	"slices"

	"go.uber.org/zap"

	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/wasm/integrations"
	"github.com/lunogram/platform/pkg/modules"
	providertypes "github.com/lunogram/platform/pkg/modules/providers"
)

// Provider wraps a unified integration with provider-specific helpers.
type Provider struct {
	*integrations.Integration
}

// ProviderError is returned when a provider call fails.
type ProviderError = integrations.ProviderError

// Manifest returns the compatibility provider manifest view derived from integration capabilities.
func (p *Provider) Manifest() providertypes.ProviderManifest {
	im := p.Integration.Manifest()
	compat := providertypes.ProviderManifest{
		Metadata: im.Metadata,
		Website:  im.Website,
		Version:  im.Version,
		License:  im.License,
		Author:   im.Author,
	}

	if spec, ok := p.Integration.ProviderSpec(); ok {
		compat.Spec = providertypes.ProviderSpec{
			Channels:          fromUnifiedChannels(spec.Channels),
			Platforms:         fromUnifiedPlatforms(spec.Platforms),
			Webhook:           spec.Webhook,
			Locked:            spec.Locked,
			RateLimit:         fromUnifiedRateLimit(spec.RateLimit),
			Config:            im.Config,
			SelfHandlesOptOut: spec.SelfHandlesOptOut,
		}
	}

	return compat
}

// SupportsWebhook reports whether this provider's module exports a webhook function.
func (p *Provider) SupportsWebhook() bool {
	if spec, ok := p.Integration.ProviderSpec(); ok && spec.Webhook {
		return true
	}

	return p.Integration.HasWebhook()
}

// Init invokes integration install semantics and adapts the compatibility response.
func (p *Provider) Init(ctx context.Context, req providertypes.InitRequest) (*providertypes.InitResponse, error) {
	res, err := p.Integration.Install(ctx, modules.InstallRequest{
		Config:        req.Config,
		WebhookURL:    req.WebhookURL,
		IntegrationID: req.ProviderID,
		ProjectID:     req.ProjectID,
	})
	if err != nil {
		return nil, err
	}

	return &providertypes.InitResponse{ConfigPatch: res.State}, nil
}

// Validate invokes integration validation and adapts the compatibility response.
func (p *Provider) Validate(ctx context.Context, req providertypes.ValidateRequest) (*providertypes.ValidateResponse, error) {
	res, err := p.Integration.Validate(ctx, modules.ValidateRequest{Config: req.Config})
	if err != nil {
		return nil, err
	}

	return &providertypes.ValidateResponse{
		Valid:   res.Valid,
		Errors:  res.Errors,
		Message: res.Message,
	}, nil
}

// Destroy invokes integration uninstall semantics.
func (p *Provider) Destroy(ctx context.Context, req providertypes.DestroyRequest) (*providertypes.DestroyResponse, error) {
	_, err := p.Integration.Uninstall(ctx, modules.UninstallRequest{
		Config:        req.Config,
		IntegrationID: req.ProviderID,
		ProjectID:     req.ProjectID,
	})
	if err != nil {
		return nil, err
	}

	return &providertypes.DestroyResponse{}, nil
}

// Webhook invokes provider webhook processing.
func (p *Provider) Webhook(ctx context.Context, req providertypes.WebhookRequest) (*providertypes.WebhookResponse, error) {
	return p.Integration.Webhook(ctx, req)
}

// Send invokes provider send.
func (p *Provider) Send(ctx context.Context, req providertypes.SendRequest[map[string]any]) (*providertypes.SendResponse, error) {
	return p.Integration.Send(ctx, req)
}

// SupportsChannel checks if this provider supports a specific channel.
func (p *Provider) SupportsChannel(ch providertypes.Channel) bool {
	spec, ok := p.Integration.ProviderSpec()
	if !ok {
		return false
	}

	return slices.Contains(spec.Channels, modules.Channel(ch))
}

// Registry is a provider-specific registry facade over unified integrations.
type Registry struct {
	*integrations.Registry
}

// Register adds a provider module to the registry.
func (r *Registry) Register(module any) error {
	return r.Registry.Register(module)
}

// NewRegistry creates a new provider registry facade.
func NewRegistry(cfg config.WASM, logger *zap.Logger) *Registry {
	return &Registry{Registry: integrations.NewRegistry(cfg, logger)}
}

// LoadFromFS loads all provider-capable modules from an embedded filesystem.
func (r *Registry) LoadFromFS(ctx context.Context, fsys fs.FS, dir string) error {
	if err := r.Registry.LoadFromFS(ctx, fsys, dir); err != nil {
		return err
	}

	for _, provider := range r.All() {
		if len(provider.Manifest().Spec.Channels) == 0 {
			return fmt.Errorf("provider %s must support at least one channel", provider.Manifest().Metadata.ID)
		}
	}

	return nil
}

// Get retrieves a provider by ID.
func (r *Registry) Get(id string) (*Provider, bool) {
	integration, exists := r.Registry.Get(id)
	if !exists {
		return nil, false
	}

	if _, ok := integration.ProviderSpec(); !ok {
		return nil, false
	}

	return &Provider{Integration: integration}, true
}

// All returns all provider-capable modules.
func (r *Registry) All() []*Provider {
	integrations := r.Registry.All()
	result := make([]*Provider, 0, len(integrations))
	for _, integration := range integrations {
		if _, ok := integration.ProviderSpec(); ok {
			result = append(result, &Provider{Integration: integration})
		}
	}

	return result
}

// SupportsChannel checks if a module supports a specific channel.
func (r *Registry) SupportsChannel(moduleID string, channel providertypes.Channel) bool {
	provider, exists := r.Get(moduleID)
	if !exists {
		return false
	}

	return provider.SupportsChannel(channel)
}

// List returns all provider module IDs.
func (r *Registry) List() []string {
	providers := r.All()
	ids := make([]string, 0, len(providers))
	for _, provider := range providers {
		ids = append(ids, provider.Manifest().Metadata.ID)
	}

	return ids
}

func fromUnifiedChannels(channels []modules.Channel) []providertypes.Channel {
	result := make([]providertypes.Channel, len(channels))
	for i, ch := range channels {
		result[i] = providertypes.Channel(ch)
	}
	return result
}

func fromUnifiedPlatforms(platforms []modules.Platform) []providertypes.Platform {
	result := make([]providertypes.Platform, len(platforms))
	for i, p := range platforms {
		result[i] = providertypes.Platform(p)
	}
	return result
}

func fromUnifiedRateLimit(limit *modules.RateLimit) *providertypes.RateLimit {
	if limit == nil {
		return nil
	}

	return &providertypes.RateLimit{
		Limit:    limit.Limit,
		Interval: limit.Interval,
		Override: limit.Override,
	}
}
