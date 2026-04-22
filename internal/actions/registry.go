package actions

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/config"
	integrationloader "github.com/lunogram/platform/internal/integrations"
	wasmactions "github.com/lunogram/platform/internal/wasm/actions"
)

// Registry is a type alias for the action registry.
type Registry = wasmactions.Registry

// Action is a type alias for the action wrapper.
type Action = wasmactions.Action

// NewRegistryFromIntegrations creates an action registry facade over a unified integration registry.
func NewRegistryFromIntegrations(registry *integrationloader.Registry) *Registry {
	return &wasmactions.Registry{Registry: registry}
}

// NewRegistry creates a new registry and loads all embedded WASM action modules.
func NewRegistry(ctx graceful.Context, cfg config.WASM, logger *zap.Logger) (*Registry, error) {
	integrationRegistry, err := integrationloader.NewRegistry(ctx, cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize action registry: %w", err)
	}

	return NewRegistryFromIntegrations(integrationRegistry), nil
}
