package providers

import (
	"embed"
	"fmt"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/wasm/providers"
)

//go:embed modules/*
var modulesFS embed.FS

// Registry is a type alias for the provider registry.
type Registry = providers.Registry

// Provider is a type alias for the provider wrapper.
type Provider = providers.Provider

// NewRegistry creates a new registry and loads all embedded WASM provider modules.
func NewRegistry(ctx graceful.Context, cfg config.WASM) (*Registry, error) {
	registry := providers.NewRegistry(cfg)

	err := registry.LoadFromFS(ctx, modulesFS, "modules")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize provider registry: %w", err)
	}

	return registry, nil
}
