package modules

import "encoding/json"

// ValidateRequest is the unified config validation request.
type ValidateRequest struct {
	Config json.RawMessage `json:"config"`
}

// ValidateResponse is the unified config validation response.
type ValidateResponse struct {
	Valid   bool              `json:"valid"`
	Errors  map[string]string `json:"errors,omitempty"`
	Message string            `json:"message,omitempty"`
}

// InstallRequest is called once when the integration is first configured.
type InstallRequest struct {
	Config        json.RawMessage `json:"config"`
	WebhookURL    string          `json:"webhook_url,omitempty"`
	IntegrationID string          `json:"integration_id"`
	ProjectID     string          `json:"project_id"`
}

// InstallResponse contains system-managed state returned by install().
type InstallResponse struct {
	State json.RawMessage `json:"state,omitempty"`
}

// UpgradeRequest is called when the integration config changes.
type UpgradeRequest struct {
	Config        json.RawMessage `json:"config"`
	PreviousState json.RawMessage `json:"previous_state"`
	WebhookURL    string          `json:"webhook_url,omitempty"`
	IntegrationID string          `json:"integration_id"`
	ProjectID     string          `json:"project_id"`
}

// UpgradeResponse contains updated system state.
type UpgradeResponse struct {
	State json.RawMessage `json:"state,omitempty"`
}

// UninstallRequest is called when the integration is deleted.
type UninstallRequest struct {
	Config        json.RawMessage `json:"config"`
	State         json.RawMessage `json:"state"`
	IntegrationID string          `json:"integration_id"`
	ProjectID     string          `json:"project_id"`
}

// UninstallResponse is kept for future use.
type UninstallResponse struct{}
