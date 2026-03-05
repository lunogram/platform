// Package actions implements a WASM-based action system for workflow steps.
//
// It builds on the generic wasm package to provide action-specific functionality
// including execute and preview interfaces for automation actions.
package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"

	"go.uber.org/zap"

	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/wasm"
	actiontypes "github.com/lunogram/platform/pkg/modules/actions"
)

// Action wraps a WASM module with action-specific functionality.
type Action struct {
	*wasm.Module[actiontypes.ActionManifest]
}

// Execute invokes the action's execute function.
func (a *Action) Execute(ctx context.Context, req *actiontypes.ExecuteRequest[json.RawMessage]) (*actiontypes.ExecuteResponse, error) {
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal execute request: %w", err)
	}

	code, res, err := a.Call(ctx, "execute", payload)
	if err != nil {
		return nil, fmt.Errorf("failed to call action execute: %w", err)
	}

	if code != 0 {
		return nil, fmt.Errorf("action execute returned code %d: %s", code, string(res))
	}

	var response actiontypes.ExecuteResponse
	if err := json.Unmarshal(res, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal execute response: %w", err)
	}

	return &response, nil
}

// Preview invokes the action's preview function and returns raw HTML bytes.
func (a *Action) Preview(ctx context.Context) ([]byte, error) {
	code, res, err := a.Call(ctx, "preview", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call action preview: %w", err)
	}

	if code != 0 {
		return nil, fmt.Errorf("action preview returned code %d: %s", code, string(res))
	}

	return res, nil
}

// Registry is an action-specific registry that wraps the generic WASM module registry.
type Registry struct {
	*wasm.Registry[actiontypes.ActionManifest]
}

// NewRegistry creates a new action registry with the given configuration.
func NewRegistry(config config.WASM, logger *zap.Logger) *Registry {
	return &Registry{
		Registry: wasm.NewRegistry[actiontypes.ActionManifest](config, logger),
	}
}

// LoadFromFS loads all action modules from an embedded filesystem.
func (r *Registry) LoadFromFS(ctx context.Context, fsys fs.FS, dir string) error {
	return r.Registry.LoadFromFS(ctx, fsys, dir)
}

// Get retrieves an action by ID.
func (r *Registry) Get(id string) (*Action, bool) {
	module, exists := r.Registry.Get(id)
	if !exists {
		return nil, false
	}
	return &Action{Module: module}, true
}

// All returns all registered actions.
func (r *Registry) All() []*Action {
	modules := r.Registry.All()
	result := make([]*Action, len(modules))
	for i, module := range modules {
		result[i] = &Action{Module: module}
	}
	return result
}
