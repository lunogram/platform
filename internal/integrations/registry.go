package integrations

import (
	"context"
	"embed"
	"io/fs"
)

//go:embed modules/*
var modulesFS embed.FS

// Loader is implemented by registries that can load WASM modules from an fs.FS.
type Loader interface {
	LoadFromFS(ctx context.Context, fsys fs.FS, dir string) error
}

// LoadModules loads all embedded integration WASM modules into the provided registry.
func LoadModules(ctx context.Context, registry Loader) error {
	return registry.LoadFromFS(ctx, modulesFS, "modules")
}
