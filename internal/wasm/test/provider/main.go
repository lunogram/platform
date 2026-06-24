package main

import (
	"encoding/json"
	"fmt"

	"github.com/extism/go-pdk"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
)

//go:export manifest
func Manifest() int32 {
	manifest := modules.IntegrationManifest{
		APIVersion: "v1",
		Metadata: modules.Metadata{
			ID:          "testprovider",
			Title:       "Test Provider",
			Description: "Test provider for WASM module testing",
			Tags:        []string{"test", "mock"},
		},
		Version: "1.0.0",
		License: "MIT",
		Author: modules.Author{
			Name:  "Lunogram",
			Email: "dev@lunogram.io",
			URL:   "https://lunogram.com",
		},
		Config: &modules.JSONSchema{
			Type: "object",
			Properties: []modules.JSONSchemaProperty{
				{
					Name: "api_key",
					Schema: &modules.JSONSchema{
						Type:        "string",
						Description: "Test API key",
					},
				},
			},
		},
		Capabilities: []modules.Capability{
			{
				Type:    "provider",
				Version: "v1",
				Spec: mustMarshalJSON(modules.ProviderSpec{
					Webhook: true,
					Channels: []modules.Channel{
						modules.ChannelEmail,
						modules.ChannelSMS,
						modules.ChannelPush,
					},
					Platforms: []modules.Platform{
						modules.PlatformIOS,
						modules.PlatformAndroid,
						modules.PlatformWeb,
					},
				}),
			},
		},
	}

	err := pdk.OutputJSON(manifest)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

type Config struct {
	APIKey string `json:"api_key"`
}

//go:export provider_send
func Send() int32 {
	var req providers.SendRequest[Config]
	err := pdk.InputJSON(&req)
	if err != nil {
		pdk.SetError(err)
		return -2 // permanent: malformed input
	}

	switch req.Channel {
	case providers.ChannelEmail:
		_, err := req.GetEmailPayload()
		if err != nil {
			pdk.SetError(err)
			return -2 // permanent: invalid payload
		}

	case providers.ChannelSMS:
		_, err := req.GetSMSPayload()
		if err != nil {
			pdk.SetError(err)
			return -2 // permanent: invalid payload
		}

	case providers.ChannelPush:
		_, err := req.GetPushPayload()
		if err != nil {
			pdk.SetError(err)
			return -2 // permanent: invalid payload
		}

	default:
		err := fmt.Errorf("unsupported channel: %s", req.Channel)
		pdk.SetError(err)
		return -2 // permanent: unsupported channel
	}

	response := providers.SendResponse{
		Status: "sent",
		Metadata: map[string]any{
			"channel":  req.Channel,
			"provider": "testprovider",
		},
	}

	err = pdk.OutputJSON(response)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

// testWebhookPayload is the expected JSON body for the test provider's webhook.
type testWebhookPayload struct {
	EventType   string `json:"event_type"`
	ReferenceID string `json:"reference_id"`
	Timestamp   string `json:"timestamp"`
}

//go:export provider_webhook
func WebhookHandler() int32 {
	var req providers.WebhookRequest
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(err)
		return -2
	}

	var payload testWebhookPayload
	if err := json.Unmarshal(req.Body, &payload); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse webhook body: %w", err))
		return -2
	}

	var eventName providers.WebhookEventName
	switch payload.EventType {
	case "delivered":
		eventName = providers.EventDelivered
	case "bounced":
		eventName = providers.EventBounced
	case "opened":
		eventName = providers.EventOpened
	case "clicked":
		eventName = providers.EventClicked
	default:
		err := pdk.OutputJSON(providers.WebhookResponse{Events: []providers.WebhookEvent{}})
		if err != nil {
			pdk.SetError(err)
			return -1
		}
		return 0
	}

	response := providers.WebhookResponse{
		Events: []providers.WebhookEvent{
			{
				EventName: eventName,
				MessageID: payload.ReferenceID,
				Timestamp: payload.Timestamp,
				Data: map[string]any{
					"provider":   "testprovider",
					"event_type": payload.EventType,
				},
			},
		},
	}

	if err := pdk.OutputJSON(response); err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

func main() {}

func mustMarshalJSON(v any) json.RawMessage {
	spec, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return spec
}
