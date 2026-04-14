package actions

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/config"
	integrationloader "github.com/lunogram/platform/internal/integrations"
	"github.com/lunogram/platform/internal/wasm/actions"
)

// Registry is a type alias for the action registry.
type Registry = actions.Registry

// Action is a type alias for the action wrapper.
type Action = actions.Action

// NewRegistry creates a new registry and loads all embedded WASM action modules.
func NewRegistry(ctx graceful.Context, cfg config.WASM, logger *zap.Logger) (*Registry, error) {
	registry := actions.NewRegistry(cfg, logger)

	err := integrationloader.LoadModules(ctx, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize action registry: %w", err)
	}

	return registry, nil
}
