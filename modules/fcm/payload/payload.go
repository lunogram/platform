// Package payload contains pure logic for building FCM v1 message bodies.
// It is free of WASM (Extism PDK) dependencies so it can be tested with
// standard `go test`.
package payload

import (
	"encoding/json"
	"fmt"

	"github.com/lunogram/platform/pkg/modules/providers"
)

// Message is the top-level wrapper expected by the FCM v1 send endpoint.
type Message struct {
	Message Body `json:"message"`
}

// Body is the FCM v1 message body.
type Body struct {
	Token        string            `json:"token"`
	Notification *Notification     `json:"notification,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
	Android      *AndroidConfig    `json:"android,omitempty"`
	APNS         *APNSConfig       `json:"apns,omitempty"`
}

// Notification is the cross-platform notification block.
type Notification struct {
	Title    string `json:"title,omitempty"`
	Body     string `json:"body,omitempty"`
	ImageURL string `json:"image,omitempty"`
}

// AndroidConfig holds Android-specific overrides.
type AndroidConfig struct {
	Notification *AndroidNotification `json:"notification,omitempty"`
}

// AndroidNotification holds Android-specific notification fields.
type AndroidNotification struct {
	Sound string `json:"sound,omitempty"`
}

// APNSConfig holds APNs-specific overrides forwarded by FCM.
type APNSConfig struct {
	Payload *APNSPayload `json:"payload,omitempty"`
}

// APNSPayload mirrors the APS sub-payload exposed via FCM.
type APNSPayload struct {
	APS *APS `json:"aps,omitempty"`
}

// APS holds the Apple-defined APS fields exposed via FCM.
type APS struct {
	Sound string `json:"sound,omitempty"`
}

// Build returns the JSON-encoded FCM v1 message body for the supplied push
// payload, target token and metadata. Metadata key/value pairs are emitted
// as fields under "data" so that delivery webhooks can echo them back.
// Existing keys in push.Data take precedence and are not overwritten.
func Build(token string, push providers.PushPayload, metadata map[string]string) ([]byte, error) {
	body := Body{
		Token:        token,
		Notification: &Notification{Title: push.Title, Body: push.Body},
	}
	if push.ImageURL != nil {
		body.Notification.ImageURL = *push.ImageURL
	}

	data := make(map[string]string, len(push.Data)+len(metadata))
	for k, v := range metadata {
		data[k] = v
	}
	for k, v := range push.Data {
		data[k] = fmt.Sprintf("%v", v)
	}
	if len(data) > 0 {
		body.Data = data
	}

	if push.Sound != nil {
		body.Android = &AndroidConfig{Notification: &AndroidNotification{Sound: *push.Sound}}
		body.APNS = &APNSConfig{Payload: &APNSPayload{APS: &APS{Sound: *push.Sound}}}
	}

	return json.Marshal(Message{Message: body})
}
