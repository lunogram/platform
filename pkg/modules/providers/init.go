package providers

import (
	"encoding/json"
	"fmt"
)

// WebhookURL constructs the fully-qualified webhook callback URL for a
// provider instance. This is the URL that external providers (Resend,
// Twilio, etc.) should POST delivery status updates to.
func WebhookURL(baseURL string, projectID, providerID fmt.Stringer) string {
	return fmt.Sprintf("%s/webhooks/%s/providers/%s", baseURL, projectID, providerID)
}

// InitRequest is the input to the provider's optional init() function.
// It is called once when a provider instance is created for a project,
// giving the module an opportunity to register webhooks, validate
// credentials, or perform other one-time setup with the external service.
type InitRequest struct {
	// Config is the provider's configuration (API keys, etc.)
	Config json.RawMessage `json:"config"`

	// WebhookURL is the fully-qualified URL the platform has allocated
	// for this provider instance's inbound webhooks.
	// e.g. "https://app.lunogram.com/webhooks/{projectID}/providers/{providerID}"
	WebhookURL string `json:"webhook_url"`

	// ProviderID is the UUID of the newly created provider instance.
	ProviderID string `json:"provider_id"`

	// ProjectID is the UUID of the project this provider belongs to.
	ProjectID string `json:"project_id"`
}

// InitResponse is the output from the provider's init() function.
type InitResponse struct {
	// ConfigPatch contains config fields that should be merged back into
	// the provider's stored configuration. For example, the module might
	// register a webhook with Resend and return the signing secret.
	// This is a JSON merge-patch applied on top of the existing config.
	ConfigPatch json.RawMessage `json:"config_patch,omitempty"`
}

// ValidateRequest is the input to the provider's optional validate() function.
// It is called before init() to verify that the provider's configuration is
// valid (e.g., API keys are correct, required fields are present).
type ValidateRequest struct {
	// Config is the provider's configuration to validate.
	Config json.RawMessage `json:"config"`
}

// ValidateResponse is the output from the provider's validate() function.
type ValidateResponse struct {
	// Valid indicates whether the configuration passed validation.
	Valid bool `json:"valid"`

	// Errors contains validation error messages keyed by config field name.
	// Empty when Valid is true.
	Errors map[string]string `json:"errors,omitempty"`

	// Message is an optional human-readable summary.
	Message string `json:"message,omitempty"`
}

// DestroyRequest is the input to the provider's optional destroy() function.
// It is called when a provider instance is deleted, giving the module an
// opportunity to deregister webhooks or clean up external resources.
type DestroyRequest struct {
	// Config is the provider's configuration (includes webhook IDs, API keys, etc.)
	Config json.RawMessage `json:"config"`

	// ProviderID is the UUID of the provider instance being deleted.
	ProviderID string `json:"provider_id"`

	// ProjectID is the UUID of the project this provider belongs to.
	ProjectID string `json:"project_id"`
}

// DestroyResponse is the output from the provider's destroy() function.
type DestroyResponse struct{}
