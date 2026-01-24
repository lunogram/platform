package providers

import "github.com/lunogram/platform/pkg/modules"

// ProviderManifest is the manifest for provider modules.
type ProviderManifest struct {
	Metadata modules.Metadata `json:"metadata"`
	Website  string           `json:"website,omitempty"`
	Version  string           `json:"version"`
	License  string           `json:"license"`
	Author   modules.Author   `json:"author"`
	Spec     ProviderSpec     `json:"spec"`
}

// GetMetadata implements modules.Manifest.
func (m ProviderManifest) GetMetadata() modules.Metadata {
	return m.Metadata
}

// ProviderSpec defines the specification for a provider module.
type ProviderSpec struct {
	Channels []Channel           `json:"channels"`
	Config   *modules.JSONSchema `json:"config,omitempty"`
}
