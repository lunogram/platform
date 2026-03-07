package actions

import "encoding/json"

// ExecuteRequest is the input to an action function.
// Config contains the shared module-level configuration (API keys, etc.).
// Input contains the per-function input data.
type ExecuteRequest[T any] struct {
	Config T               `json:"config"`
	Input  json.RawMessage `json:"input,omitempty"`
}

// ExecuteResponse is the output from an action function.
type ExecuteResponse struct {
	StatusCode int            `json:"status_code"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// ValidateRequest is the input to the validate function.
// Config contains the module-level configuration to validate (API keys, OAuth tokens, etc.).
type ValidateRequest[T any] struct {
	Config T `json:"config"`
}

// ValidateResponse is the output from the validate function.
type ValidateResponse struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message,omitempty"`
}
