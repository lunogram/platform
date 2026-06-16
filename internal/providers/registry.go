package providers

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/config"
	integrationloader "github.com/lunogram/platform/internal/integrations"
	wasmproviders "github.com/lunogram/platform/internal/wasm/providers"
)

// Registry is a type alias for the provider registry.
type Registry = wasmproviders.Registry

// Provider is a type alias for the provider wrapper.
type Provider = wasmproviders.Provider

// NewRegistryFromIntegrations creates a provider registry facade over a unified integration registry.
func NewRegistryFromIntegrations(registry *integrationloader.Registry) *Registry {
	return &wasmproviders.Registry{Registry: registry}
}

// NewRegistry creates a new registry and loads all embedded WASM provider modules.
func NewRegistry(ctx graceful.Context, cfg config.WASM, logger *zap.Logger) (*Registry, error) {
	integrationRegistry, err := integrationloader.NewRegistry(ctx, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider registry: %w", err)
	}

	return NewRegistryFromIntegrations(integrationRegistry), nil
}
