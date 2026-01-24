package wasm

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	"github.com/lunogram/platform/pkg/modules"
)

// Registry is a generic registry for WASM modules.
type Registry[T modules.Manifest] struct {
	mu      sync.RWMutex
	modules map[string]*Module[T]
}

// NewRegistry creates a new empty registry.
func NewRegistry[T modules.Manifest]() *Registry[T] {
	return &Registry[T]{
		modules: make(map[string]*Module[T]),
	}
}

// LoadFromFS populates the registry from an embedded filesystem.
func (r *Registry[T]) LoadFromFS(ctx context.Context, fsys fs.FS, dir string) error {
	return fs.WalkDir(fsys, dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".wasm") {
			return nil
		}

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("failed to read module %s: %w", path, err)
		}

		module, err := LoadModule[T](ctx, data)
		if err != nil {
			return fmt.Errorf("failed to load module %s: %w", path, err)
		}

		id := module.manifest.GetMetadata().ID
		if id == "" {
			module.Close(ctx)
			return fmt.Errorf("module %s has no ID in manifest", path)
		}

		r.mu.Lock()
		r.modules[id] = module
		r.mu.Unlock()

		return nil
	})
}

// Register adds a module to the registry.
func (r *Registry[T]) Register(module *Module[T]) error {
	id := module.manifest.GetMetadata().ID
	if id == "" {
		return fmt.Errorf("module has no ID in manifest")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.modules[id]; exists {
		return fmt.Errorf("module %s already registered", id)
	}

	r.modules[id] = module
	return nil
}

// Get retrieves a module by ID.
func (r *Registry[T]) Get(id string) (*Module[T], bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	module, exists := r.modules[id]
	return module, exists
}

// List returns all registered module IDs.
func (r *Registry[T]) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.modules))
	for id := range r.modules {
		ids = append(ids, id)
	}
	return ids
}

// All returns all registered modules.
func (r *Registry[T]) All() []*Module[T] {
	r.mu.RLock()
	defer r.mu.RUnlock()

	modules := make([]*Module[T], 0, len(r.modules))
	for _, module := range r.modules {
		modules = append(modules, module)
	}
	return modules
}

// Close closes all modules in the registry.
func (r *Registry[T]) Close(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, module := range r.modules {
		module.Close(ctx)
	}

	r.modules = make(map[string]*Module[T])
}
