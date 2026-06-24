package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/pubsub/schemas"
)

// ActionCaller wraps a NATS caller to provide validate and execute
// operations for actions via the WASM action runner service.
type ActionCaller struct {
	caller Caller
}

// NewActionCaller creates a new ActionCaller backed by the given NATS caller.
func NewActionCaller(caller Caller) *ActionCaller {
	return &ActionCaller{caller: caller}
}

// Validate sends an action configuration validation request via NATS
// request/reply and returns the validation response.
func (a *ActionCaller) Validate(ctx context.Context, projectID uuid.UUID, actionType string, config map[string]any) (*schemas.ValidateActionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	reply, err := a.caller.Call(ctx, schemas.ActionsValidate(projectID), schemas.ValidateAction{
		ProjectID: projectID,
		Type:      actionType,
		Config:    config,
	})
	if err != nil {
		return nil, fmt.Errorf("validate action: %w", err)
	}

	var resp schemas.ValidateActionResponse
	if err := json.Unmarshal(reply, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal validate response: %w", err)
	}

	return &resp, nil
}

// Execute sends an action execution request via NATS request/reply
// and returns the execution response.
func (a *ActionCaller) Execute(ctx context.Context, projectID uuid.UUID, actionID uuid.UUID, actionType string, functionID string, config map[string]any, input any) (*schemas.ExecuteActionResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	reply, err := a.caller.Call(ctx, schemas.ActionsExecute(projectID), schemas.ExecuteAction{
		ProjectID:  projectID,
		ActionID:   actionID,
		Type:       actionType,
		FunctionID: functionID,
		Config:     config,
		Input:      input,
	})
	if err != nil {
		return nil, fmt.Errorf("execute action: %w", err)
	}

	var resp schemas.ExecuteActionResponse
	if err := json.Unmarshal(reply, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal execute response: %w", err)
	}

	return &resp, nil
}
