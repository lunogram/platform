// Package actions provides action-oriented wrappers over unified WASM integrations.
package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/wasm/integrations"
	"github.com/lunogram/platform/pkg/modules"
	actiontypes "github.com/lunogram/platform/pkg/modules/actions"
)

// Action wraps a unified integration with action-specific helpers.
type Action struct {
	*integrations.Integration
	functionID string
}

// Manifest returns the compatibility action manifest view derived from integration capabilities.
func (a *Action) Manifest() actiontypes.ActionManifest {
	im := a.Integration.Manifest()
	compat := actiontypes.ActionManifest{
		Metadata: im.Metadata,
		Version:  im.Version,
		License:  im.License,
		Author:   im.Author,
		Config:   im.Config,
	}

	if spec, ok := a.Integration.ActionsSpec(); ok {
		compat.Functions = make([]actiontypes.ActionFunction, len(spec.Functions))
		for i, fn := range spec.Functions {
			compat.Functions[i] = actiontypes.ActionFunction{
				ID:          fn.ID,
				Title:       fn.Title,
				Description: fn.Description,
				Input:       fn.Input,
			}
		}
	}

	return compat
}

// Execute invokes the action function.
func (a *Action) Execute(ctx context.Context, functionName string, req *actiontypes.ExecuteRequest[json.RawMessage]) (*actiontypes.ExecuteResponse, error) {
	fn := functionName
	if fn == "" {
		fn = a.functionID
	}

	if fn == "" {
		return nil, fmt.Errorf("action function id is required")
	}

	return a.Integration.Execute(ctx, fn, req)
}

// Validate invokes action/module validation.
func (a *Action) Validate(ctx context.Context, req *actiontypes.ValidateRequest[json.RawMessage]) (*actiontypes.ValidateResponse, error) {
	res, err := a.Integration.Validate(ctx, toUnifiedValidateRequest(req))
	if err != nil {
		return nil, err
	}

	status := 200
	if !res.Valid {
		status = 400
	}

	return &actiontypes.ValidateResponse{
		StatusCode: status,
		Message:    res.Message,
	}, nil
}

// Preview invokes action preview.
func (a *Action) Preview(ctx context.Context) ([]byte, error) {
	fn := a.functionID
	if fn == "" {
		if spec, ok := a.Integration.ActionsSpec(); ok && len(spec.Functions) == 1 {
			fn = spec.Functions[0].ID
		}
	}

	return a.Integration.Preview(ctx, fn)
}

func toUnifiedValidateRequest(req *actiontypes.ValidateRequest[json.RawMessage]) modules.ValidateRequest {
	if req == nil {
		return modules.ValidateRequest{}
	}

	return modules.ValidateRequest{Config: req.Config}
}

// Registry is an action-specific registry facade over unified integrations.
type Registry struct {
	*integrations.Registry
}

// NewRegistry creates a new action registry facade.
func NewRegistry(cfg config.WASM, logger *zap.Logger) *Registry {
	return &Registry{Registry: integrations.NewRegistry(cfg, logger)}
}

// Get retrieves an action by module ID.
func (r *Registry) Get(id string) (*Action, bool) {
	integration, exists := r.Registry.Get(id)
	if !exists || !integration.HasActions() {
		return nil, false
	}

	return &Action{Integration: integration}, true
}

// GetFunction retrieves an action bound to a specific function ID.
func (r *Registry) GetFunction(id, functionID string) (*Action, bool) {
	action, ok := r.Get(id)
	if !ok {
		return nil, false
	}

	action.functionID = functionID
	return action, true
}

// All returns all action-capable modules.
func (r *Registry) All() []*Action {
	integrations := r.Registry.All()
	result := make([]*Action, 0, len(integrations))
	for _, integration := range integrations {
		if integration.HasActions() {
			result = append(result, &Action{Integration: integration})
		}
	}

	return result
}
