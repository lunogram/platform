package actions

import "github.com/lunogram/platform/pkg/modules"

// ActionManifest is the manifest for action modules.
type ActionManifest struct {
	Metadata  modules.Metadata    `json:"metadata"`
	Version   string              `json:"version"`
	License   string              `json:"license"`
	Author    modules.Author      `json:"author"`
	Config    *modules.JSONSchema `json:"config,omitempty"` // shared module-level config (API keys, etc.)
	Functions []ActionFunction    `json:"functions"`        // required, at least one
}

// GetMetadata implements modules.Manifest.
func (m ActionManifest) GetMetadata() modules.Metadata {
	return m.Metadata
}

// ActionFunction declares a single callable operation within a module.
type ActionFunction struct {
	ID          string              `json:"id"`              // e.g. "get_account", "create_coupon"
	Title       string              `json:"title"`           // e.g. "Get Account"
	Description string              `json:"description"`     // e.g. "Retrieve a Stripe account by ID"
	Input       *modules.JSONSchema `json:"input,omitempty"` // per-function input schema
}
