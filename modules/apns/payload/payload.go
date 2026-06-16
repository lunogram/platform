// Package payload contains pure logic for building APNs notification
// payloads. It is free of WASM (Extism PDK) dependencies so it can be
// tested with standard `go test`.
package payload

import (
	"encoding/json"

	"github.com/lunogram/platform/pkg/modules/providers"
)

// APS is the standard Apple-defined "aps" sub-payload.
type APS struct {
	Alert *Alert  `json:"alert,omitempty"`
	Badge *int    `json:"badge,omitempty"`
	Sound *string `json:"sound,omitempty"`
}

// Alert is the localized alert sub-payload.
type Alert struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
}

// Build returns the JSON-encoded APNs notification payload for the supplied
// push payload and metadata. Metadata key/value pairs are emitted as
// top-level custom keys alongside "aps" so that delivery webhooks can echo
// them back. The reserved keys "aps" and "data" are never overwritten by
// metadata; conflicting metadata keys are silently dropped.
func Build(push providers.PushPayload, metadata map[string]string) ([]byte, error) {
	out := map[string]any{
		"aps": APS{
			Alert: &Alert{Title: push.Title, Body: push.Body},
			Badge: push.Badge,
			Sound: push.Sound,
		},
	}

	if len(push.Data) > 0 {
		out["data"] = push.Data
	}

	for k, v := range metadata {
		if k == "aps" || k == "data" {
			continue
		}
		out[k] = v
	}

	return json.Marshal(out)
}
