package actions

import "github.com/lunogram/platform/pkg/modules"

// ActionManifest is the manifest for action modules.
type ActionManifest struct {
	Metadata modules.Metadata `json:"metadata"`
	Version  string           `json:"version"`
	License  string           `json:"license"`
	Author   modules.Author   `json:"author"`
	Spec     ActionSpec       `json:"spec"`
}

// GetMetadata implements modules.Manifest.
func (m ActionManifest) GetMetadata() modules.Metadata {
	return m.Metadata
}

// ActionSpec defines the specification for an action module.
type ActionSpec struct {
	Config  *modules.JSONSchema `json:"config,omitempty"`
	Payload *modules.JSONSchema `json:"payload,omitempty"`
}
