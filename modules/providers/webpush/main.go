package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/extism/go-pdk"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
)

// safeTransport wraps HTTP transport to ensure resp.Body is never nil
type safeTransport struct {
	inner http.RoundTripper
}

func (t *safeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.Body == nil {
		resp.Body = io.NopCloser(bytes.NewReader(nil))
	}
	return resp, nil
}

//go:export manifest
func Manifest() int32 {
	manifest := providers.ProviderManifest{
		Metadata: modules.Metadata{
			ID:          "webpush",
			Title:       "Web Push",
			Description: "Send push notifications to web browsers via Web Push Protocol",
			Icon:        "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgc3Ryb2tlPSJjdXJyZW50Q29sb3IiIHN0cm9rZS13aWR0aD0iMiIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIiBzdHJva2UtbGluZWpvaW49InJvdW5kIj48cGF0aCBkPSJNMTggOGE2IDYgMCAwIDAtMTItMCIvPjxwYXRoIGQ9Ik0yIDhoMTYiLz48cGF0aCBkPSJNNiAxNWwtMi0zaDE2bC0yIDMiLz48cGF0aCBkPSJNNiAxNWgxMiIvPjwvc3ZnPg==",
			Color:       "#5A67D8",
			Tags:        []string{"push", "web", "browser", "notifications"},
		},
		Website: "https://developer.mozilla.org/en-US/docs/Web/API/Push_API",
		Version: "1.0.0",
		License: "MIT",
		Author: modules.Author{
			Name:  "Lunogram",
			Email: "dev@lunogram.io",
			URL:   "https://lunogram.com",
		},
		Spec: providers.ProviderSpec{
			Channels: []providers.Channel{providers.ChannelPush},
			Config: &modules.JSONSchema{
				Type: "object",
				Properties: []modules.JSONSchemaProperty{
					{
						Name: "data",
						Schema: &modules.JSONSchema{
							Type: "object",
							Properties: []modules.JSONSchemaProperty{
								{
									Name: "vapidPublicKey",
									Schema: &modules.JSONSchema{
										Type:        "string",
										Title:       "VAPID Public Key",
										Description: "Your VAPID public key (base64url encoded)",
									},
								},
								{
									Name: "vapidPrivateKey",
									Schema: &modules.JSONSchema{
										Type:        "string",
										Title:       "VAPID Private Key",
										Description: "Your VAPID private key (base64url encoded)",
										Format:      "password",
									},
								},
								{
									Name: "vapidEmail",
									Schema: &modules.JSONSchema{
										Type:        "string",
										Title:       "VAPID Email",
										Description: "Contact email for VAPID (e.g., mailto:admin@example.com)",
									},
								},
							},
							Required: []string{"vapidPublicKey", "vapidPrivateKey", "vapidEmail"},
						},
					},
				},
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
	VapidPublicKey  string `json:"vapidPublicKey"`
	VapidPrivateKey string `json:"vapidPrivateKey"`
	VapidEmail      string `json:"vapidEmail"`
}

//go:export send
func Send() int32 {
	var req providers.SendRequest[Config]
	err := pdk.InputJSON(&req)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to parse request: %w", err))
		return -1
	}

	// Only push channel is supported
	if req.Channel != providers.ChannelPush {
		pdk.SetError(fmt.Errorf("unsupported channel: %s (expected 'push')", req.Channel))
		return -1
	}

	// Get push payload
	push, err := req.GetPushPayload()
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to parse push payload: %w", err))
		return -1
	}

	// Validate we have Web Push targets
	if len(push.WebPushTargets) == 0 {
		pdk.SetError(fmt.Errorf("no Web Push subscriptions provided (found %d FCM tokens but this provider only supports Web Push)", len(push.Tokens)))
		return -1
	}

	// Validate config
	if req.Config.VapidPublicKey == "" {
		pdk.SetError(fmt.Errorf("vapidPublicKey is required in provider configuration"))
		return -1
	}
	if req.Config.VapidPrivateKey == "" {
		pdk.SetError(fmt.Errorf("vapidPrivateKey is required in provider configuration"))
		return -1
	}
	if req.Config.VapidEmail == "" {
		pdk.SetError(fmt.Errorf("vapidEmail is required in provider configuration"))
		return -1
	}

	pdk.Log(pdk.LogInfo, fmt.Sprintf("Sending Web Push notification to %d subscriptions", len(push.WebPushTargets)))
	pdk.Log(pdk.LogInfo, fmt.Sprintf("Title: %s, Body: %s", push.Title, push.Body))

	// Build notification payload
	notification := map[string]any{
		"title": push.Title,
		"body":  push.Body,
	}

	if push.Data != nil && len(push.Data) > 0 {
		notification["data"] = push.Data
	}

	if push.ImageURL != nil {
		notification["image"] = *push.ImageURL
	}

	if push.Badge != nil {
		notification["badge"] = *push.Badge
	}

	if push.Sound != nil {
		notification["sound"] = *push.Sound
	}

	payloadBytes, err := json.Marshal(notification)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to marshal notification payload: %w", err))
		return -1
	}

	// Send to all Web Push subscriptions
	successCount := 0
	failureCount := 0
	var errors []string

	for i, target := range push.WebPushTargets {
		pdk.Log(pdk.LogDebug, fmt.Sprintf("Sending to subscription %d/%d: %s", i+1, len(push.WebPushTargets), target.Endpoint))

		err := sendWebPushNotification(req.Config, target, payloadBytes)
		if err != nil {
			failureCount++
			errorMsg := fmt.Sprintf("Subscription %d failed: %v", i+1, err)
			errors = append(errors, errorMsg)
			pdk.Log(pdk.LogWarn, errorMsg)
		} else {
			successCount++
			pdk.Log(pdk.LogDebug, fmt.Sprintf("Successfully sent to subscription %d", i+1))
		}
	}

	// Log summary
	pdk.Log(pdk.LogInfo, fmt.Sprintf("Web Push complete: %d succeeded, %d failed", successCount, failureCount))

	// Build response
	response := providers.SendResponse{
		Status: "sent",
		Metadata: map[string]any{
			"success_count": successCount,
			"failure_count": failureCount,
			"total_targets": len(push.WebPushTargets),
		},
	}

	if len(errors) > 0 {
		response.Metadata["errors"] = errors
		// If all failed, change status
		if successCount == 0 {
			response.Status = "failed"
		} else {
			response.Status = "partial"
		}
	}

	err = pdk.OutputJSON(response)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

func sendWebPushNotification(config Config, target providers.WebPushTarget, payload []byte) error {
	// Validate target has required fields
	if target.Endpoint == "" {
		return fmt.Errorf("subscription missing endpoint")
	}
	if target.Keys.Auth == "" {
		return fmt.Errorf("subscription missing auth key")
	}
	if target.Keys.P256dh == "" {
		return fmt.Errorf("subscription missing p256dh key")
	}

	// Build subscription info
	subscription := &webpush.Subscription{
		Endpoint: target.Endpoint,
		Keys: webpush.Keys{
			Auth:   target.Keys.Auth,
			P256dh: target.Keys.P256dh,
		},
	}

	// Build VAPID options
	// TTL: 24 hours (how long push services should queue the message)
	options := &webpush.Options{
		Subscriber:      config.VapidEmail,
		VAPIDPublicKey:  config.VapidPublicKey,
		VAPIDPrivateKey: config.VapidPrivateKey,
		TTL:             86400, // 24 hours in seconds
	}

	// Send notification using webpush-go library
	resp, err := webpush.SendNotification(payload, subscription, options)
	if err != nil {
		return fmt.Errorf("webpush send failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	switch resp.StatusCode {
	case 201:
		// Success
		return nil
	case 400:
		return fmt.Errorf("invalid request (status 400)")
	case 401:
		return fmt.Errorf("unauthorized - check VAPID keys (status 401)")
	case 404:
		return fmt.Errorf("subscription not found (status 404)")
	case 410:
		return fmt.Errorf("subscription expired (status 410) - should be removed from database")
	case 413:
		return fmt.Errorf("payload too large (status 413)")
	case 429:
		return fmt.Errorf("rate limited (status 429)")
	default:
		if resp.StatusCode >= 500 {
			return fmt.Errorf("push service error (status %d)", resp.StatusCode)
		}
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
}

func main() {}
