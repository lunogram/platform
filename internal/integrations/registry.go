package integrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/config"
	wasmintegrations "github.com/lunogram/platform/internal/wasm/integrations"
	"go.uber.org/zap"
)

//go:embed modules/*
var modulesFS embed.FS

// Loader is implemented by registries that can load WASM modules from an fs.FS.
type Loader interface {
	LoadFromFS(ctx context.Context, fsys fs.FS, dir string) error
}

// Registry is a type alias for the unified WASM integration registry.
type Registry = wasmintegrations.Registry

// LoadModules loads all embedded integration WASM modules into the provided registry.
func LoadModules(ctx context.Context, registry Loader) error {
	return registry.LoadFromFS(ctx, modulesFS, "modules")
}

// NewRegistry creates a unified integration registry and loads all embedded modules.
func NewRegistry(ctx graceful.Context, cfg config.WASM, logger *zap.Logger) (*Registry, error) {
	registry := wasmintegrations.NewRegistry(cfg, logger)

	err := LoadModules(ctx, registry)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize integration registry: %w", err)
	}

	return registry, nil
}
