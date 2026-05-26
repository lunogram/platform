// Package payload contains pure logic for building Web Push notification
// payloads. It is free of WASM (Extism PDK) dependencies so it can be
// tested with standard `go test`.
package payload

import (
	"encoding/json"

	"github.com/lunogram/platform/pkg/modules/providers"
)

// Build returns the JSON-encoded Web Push notification payload for the
// supplied push payload and metadata. Metadata key/value pairs are emitted
// as top-level custom keys alongside the standard notification fields so
// that delivery webhooks and service workers can echo them back. Reserved
// keys ("title", "body", "data", "image", "badge", "sound") are never
// overwritten by metadata; conflicting metadata keys are silently dropped.
func Build(push providers.PushPayload, metadata map[string]string) ([]byte, error) {
	out := map[string]any{
		"title": push.Title,
		"body":  push.Body,
	}

	if len(push.Data) > 0 {
		out["data"] = push.Data
	}
	if push.ImageURL != nil {
		out["image"] = *push.ImageURL
	}
	if push.Badge != nil {
		out["badge"] = *push.Badge
	}
	if push.Sound != nil {
		out["sound"] = *push.Sound
	}

	for k, v := range metadata {
		switch k {
		case "title", "body", "data", "image", "badge", "sound":
			continue
		}
		out[k] = v
	}

	return json.Marshal(out)
}
