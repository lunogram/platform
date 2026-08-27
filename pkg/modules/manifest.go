package modules

import "encoding/json"

// Manifest is the interface all module manifests must implement.
type Manifest interface {
	GetMetadata() Metadata
}

// IntegrationManifest is the unified manifest for integration modules.
type IntegrationManifest struct {
	APIVersion   string       `json:"apiVersion"`
	Metadata     Metadata     `json:"metadata"`
	Version      string       `json:"version"`
	License      string       `json:"license,omitempty"`
	Author       Author       `json:"author"`
	Website      string       `json:"website,omitempty"`
	Config       *JSONSchema  `json:"config,omitempty"`
	Capabilities []Capability `json:"capabilities"`
}

// GetMetadata implements Manifest.
func (m IntegrationManifest) GetMetadata() Metadata { return m.Metadata }

// Capability is a typed, versioned capability declaration.
type Capability struct {
	Type    string          `json:"type"`
	Version string          `json:"version"`
	Spec    json.RawMessage `json:"spec"`
}

// ProviderSpec is decoded when Capability.Type == "provider".
type ProviderSpec struct {
	Channels  []Channel  `json:"channels"`
	Platforms []Platform `json:"platforms,omitempty"`
	Webhook   bool       `json:"webhook,omitempty"`
	Locked    bool       `json:"locked,omitempty"`
	RateLimit *RateLimit `json:"rate_limit,omitempty"`

	// SelfHandlesOptOut reports that the provider sends its own opt-out
	// confirmation. The host still records the opt-out; it skips its own reply.
	SelfHandlesOptOut bool `json:"self_handles_opt_out,omitempty"`
}

// RateLimit defines a rate limit as a number of allowed requests within a
// time interval.
type RateLimit struct {
	Limit    int    `json:"limit"`
	Interval string `json:"interval,omitempty"`
	Override bool   `json:"override,omitempty"`
}

// ActionsSpec is decoded when Capability.Type == "actions".
type ActionsSpec struct {
	Functions []ActionFunction `json:"functions"`
}

// ActionFunction declares a single callable operation within an integration.
type ActionFunction struct {
	ID          string      `json:"id"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Input       *JSONSchema `json:"input,omitempty"`
}

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
	Type        string               `json:"type"`
	Title       string               `json:"title,omitempty"`
	Description string               `json:"description,omitempty"`
	Format      string               `json:"format,omitempty"`
	Properties  []JSONSchemaProperty `json:"properties,omitempty"`
	Required    []string             `json:"required,omitempty"`
	Enum        []string             `json:"enum,omitempty"`
	MinLength   *int                 `json:"minLength,omitempty"`
	MaxLength   *int                 `json:"maxLength,omitempty"`
	Preview     string               `json:"preview,omitempty"`
	FileUpload  bool                 `json:"fileUpload,omitempty"`
	FileAccept  string               `json:"fileAccept,omitempty"`
	Hidden      bool                 `json:"hidden,omitempty"`
}
