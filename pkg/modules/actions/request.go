package actions

import "encoding/json"

// ExecuteRequest is the input to the action's execute() function.
// The Payload field contains action-specific data.
type ExecuteRequest[T any] struct {
	Config    T               `json:"config"`
	Payload   json.RawMessage `json:"payload"`
	Variables map[string]any  `json:"variables,omitempty"`
}

// ExecuteResponse is the output from the action's execute() function.
type ExecuteResponse struct {
	Status     string         `json:"status"`
	StatusCode *int           `json:"status_code,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}
