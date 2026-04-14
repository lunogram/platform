package providers

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/config"
	integrationloader "github.com/lunogram/platform/internal/integrations"
	"github.com/lunogram/platform/internal/wasm/providers"
)

// Registry is a type alias for the provider registry.
type Registry = providers.Registry

// Provider is a type alias for the provider wrapper.
type Provider = providers.Provider

// NewRegistry creates a new registry and loads all embedded WASM provider modules.
func NewRegistry(ctx graceful.Context, cfg config.WASM, logger *zap.Logger) (*Registry, error) {
	registry := providers.NewRegistry(cfg, logger)

	err := integrationloader.LoadModules(ctx, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider registry: %w", err)
	}

	return registry, nil
}
