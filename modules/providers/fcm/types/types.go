package types

import "encoding/json"

// Metadata contains common metadata for all modules.
type Metadata struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Icon        string   `json:"icon,omitempty"`
	Color       string   `json:"color,omitempty"`
	Tags        []string `json:"tags"`
	Hidden      bool     `json:"hidden,omitempty"`
}

// Author contains module author information.
type Author struct {
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	URL   string `json:"url,omitempty"`
}

// JSONSchemaProperty pairs a property name with its schema definition.
// Using a slice of these instead of a map preserves declaration order.
type JSONSchemaProperty struct {
	Name   string      `json:"name"`
	Schema *JSONSchema `json:"schema"`
	Hidden bool        `json:"hidden,omitempty"`
}

// JSONSchema represents a JSON Schema object compatible with the frontend.
// This follows the JSON Schema draft-07 specification.
type JSONSchema struct {
	Type          string               `json:"type"`
	Title         string               `json:"title,omitempty"`
	Description   string               `json:"description,omitempty"`
	Format        string               `json:"format,omitempty"`
	Properties    []JSONSchemaProperty `json:"properties,omitempty"`
	Required      []string             `json:"required,omitempty"`
	Enum          []string             `json:"enum,omitempty"`
	MinLength     *int                 `json:"minLength,omitempty"`
	MaxLength     *int                 `json:"maxLength,omitempty"`
	Preview       string               `json:"preview,omitempty"`
	FileUpload    bool                 `json:"fileUpload,omitempty"`
	FileAccept    string               `json:"fileAccept,omitempty"`
	RequireBase64 bool                 `json:"requireBase64,omitempty"`
	Hidden        bool                 `json:"hidden,omitempty"`
}

// ProviderManifest is the manifest for provider modules.
type ProviderManifest struct {
	Metadata Metadata     `json:"metadata"`
	Website  string       `json:"website,omitempty"`
	Version  string       `json:"version"`
	License  string       `json:"license"`
	Author   Author       `json:"author"`
	Spec     ProviderSpec `json:"spec"`
}

// ProviderSpec defines the specification for a provider module.
type ProviderSpec struct {
	Channels []Channel   `json:"channels"`
	Config   *JSONSchema `json:"config,omitempty"`
	Locked   bool        `json:"locked,omitempty"`
	Webhook  bool        `json:"webhook,omitempty"` // true if the module exports a webhook() function
}

// Channel represents a communication channel type.
type Channel string

const ChannelPush Channel = "push"

// SendRequest is the input to the provider's send() function.
// The Payload field contains channel-specific data.
type SendRequest[T any] struct {
	Channel Channel         `json:"channel"`
	Config  T               `json:"config"`
	Payload json.RawMessage `json:"payload"`
}

// SendResponse is the output from the provider's send() function.
type SendResponse struct {
	ID       string         `json:"id,omitempty"`
	Status   string         `json:"status"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// GetPushPayload unmarshals the payload as PushPayload.
func (r SendRequest[T]) GetPushPayload() (PushPayload, error) {
	var payload PushPayload
	if err := json.Unmarshal(r.Payload, &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

// PushPayload contains push notification-specific message data.
type PushPayload struct {
	Tokens     []string       `json:"tokens"`
	APNsTokens []string       `json:"apns_tokens,omitempty"`
	Title      string         `json:"title"`
	Body       string         `json:"body"`
	ImageURL   *string        `json:"image_url,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	Sound      *string        `json:"sound,omitempty"`
	Badge      *int           `json:"badge,omitempty"`
}
