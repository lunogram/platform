package modules

// Manifest is the interface all module manifests must implement.
type Manifest interface {
	GetMetadata() Metadata
}

// Metadata contains common metadata for all modules.
type Metadata struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
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
}
