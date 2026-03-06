package actions

import (
	"embed"
	"fmt"

	"go.uber.org/zap"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/wasm/actions"
)

//go:embed modules/*
var modulesFS embed.FS

// Registry is a type alias for the action registry.
type Registry = actions.Registry

// Action is a type alias for the action wrapper.
type Action = actions.Action

// NewRegistry creates a new registry and loads all embedded WASM action modules.
func NewRegistry(ctx graceful.Context, cfg config.WASM, logger *zap.Logger) (*Registry, error) {
	registry := actions.NewRegistry(cfg, logger)

	err := registry.LoadFromFS(ctx, modulesFS, "modules")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize action registry: %w", err)
	}

	return registry, nil
}
